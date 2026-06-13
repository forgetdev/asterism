// Package scan implements the pattern-matching engine for the `asterism scan`
// subcommand. It evaluates a set of built-in anomaly patterns against a slice
// of correlated calls and returns a match list — one entry per call that
// triggered at least one pattern.
//
// Design: each pattern is an independent predicate that receives a model.Call
// and returns a non-empty reason string when the call is suspicious, or an
// empty string when it is not. The engine is intentionally side-effect-free so
// callers can parallelise in the future without a lock.
package scan

import (
	"fmt"
	"strings"
	"time"

	"github.com/forgetdev/asterism/internal/model"
	"github.com/forgetdev/asterism/internal/sip"
)

// Options controls which patterns are active and their thresholds.
// A zero-value Options with no patterns set still computes the no-answer
// aggregate (NoAnswerRate is always reported in the summary).
type Options struct {
	// LongHold, when non-zero, flags calls whose total bridge duration exceeds
	// this threshold. "Bridge duration" is the span from the earliest
	// BRIDGE_ENTER to the latest BRIDGE_EXIT sharing the same bridge_id.
	// A call with multiple bridges uses the longest single bridge span.
	LongHold time.Duration

	// ManyTransfers, when non-nil, flags calls with more than *ManyTransfers
	// transfer events (BLINDTRANSFER + ATTENDEDTRANSFER combined). nil disables
	// the check. Set to a pointer to 0 to flag any call with at least one
	// transfer.
	ManyTransfers *int

	// CodecFailure, when true, flags calls where SIP diagnostics detected a
	// codec negotiation failure (no a=rtpmap lines in the INVITE SDP) or an
	// RTP setup failure (native_rtp WARNING log lines). Requires the call to
	// have SIPMessages or LogLines populated via --full-log.
	CodecFailure bool
}

// Match is a single call that triggered one or more scan patterns.
type Match struct {
	LinkedID string
	Reason   string // human-readable, one-line; multiple patterns are joined with "; "
}

// Summary holds aggregate metrics computed over all calls, regardless of which
// matches were found.
type Summary struct {
	Total    int
	Matched  int
	NoAnswer int // calls where disposition == "NO ANSWER" or no CDR ANSWER event
}

// NoAnswerRate returns the fraction of unanswered calls as a float64 in [0,1].
// Returns 0 when Total is zero.
func (s Summary) NoAnswerRate() float64 {
	if s.Total == 0 {
		return 0
	}
	return float64(s.NoAnswer) / float64(s.Total)
}

// Run evaluates opts against every call in calls and returns matched calls
// plus aggregate summary statistics. Order of matches follows the order of
// calls in the input slice.
func Run(calls []model.Call, opts Options) ([]Match, Summary) {
	var matches []Match
	var summary Summary
	summary.Total = len(calls)

	for _, call := range calls {
		if isNoAnswer(call) {
			summary.NoAnswer++
		}

		var reasons []string

		if opts.LongHold > 0 {
			if r := checkLongHold(call, opts.LongHold); r != "" {
				reasons = append(reasons, r)
			}
		}
		if opts.ManyTransfers != nil {
			if r := checkManyTransfers(call, *opts.ManyTransfers); r != "" {
				reasons = append(reasons, r)
			}
		}
		if opts.CodecFailure {
			if r := checkCodecFailure(call); r != "" {
				reasons = append(reasons, r)
			}
		}

		if len(reasons) > 0 {
			matches = append(matches, Match{
				LinkedID: call.LinkedID,
				Reason:   strings.Join(reasons, "; "),
			})
		}
	}

	summary.Matched = len(matches)
	return matches, summary
}

// isNoAnswer reports whether the call was never answered. We check the primary
// CDR disposition first; if no CDR is attached we look for an ANSWER event in
// the CEL stream as a fallback.
func isNoAnswer(call model.Call) bool {
	if cdr := call.PrimaryCDR(); cdr != nil {
		return strings.EqualFold(cdr.Disposition, "NO ANSWER")
	}
	// Fallback: no CDR — check whether any ANSWER event is present.
	for _, e := range call.Events {
		if e.Type == model.EventAnswer {
			return false
		}
	}
	return true
}

// checkLongHold returns a non-empty reason when any bridge in call has a
// duration that exceeds threshold. It finds bridge spans by pairing the
// earliest BRIDGE_ENTER with the latest BRIDGE_EXIT per bridge_id.
func checkLongHold(call model.Call, threshold time.Duration) string {
	type span struct {
		enter time.Time
		exit  time.Time
	}
	bridges := make(map[string]*span)

	for _, e := range call.Events {
		if e.Type != model.EventBridgeEnter && e.Type != model.EventBridgeExit {
			continue
		}
		extra, err := model.DecodeExtra(e.Extra)
		if err != nil || extra.BridgeID == "" {
			continue
		}
		id := extra.BridgeID
		if bridges[id] == nil {
			bridges[id] = &span{}
		}
		s := bridges[id]
		if e.Type == model.EventBridgeEnter {
			if s.enter.IsZero() || e.Timestamp.Before(s.enter) {
				s.enter = e.Timestamp
			}
		} else {
			if s.exit.IsZero() || e.Timestamp.After(s.exit) {
				s.exit = e.Timestamp
			}
		}
	}

	var longest time.Duration
	for _, s := range bridges {
		if s.enter.IsZero() || s.exit.IsZero() {
			continue
		}
		if d := s.exit.Sub(s.enter); d > longest {
			longest = d
		}
	}

	if longest > threshold {
		return fmt.Sprintf("long-hold: bridge duration %s exceeds threshold %s",
			formatDuration(longest), formatDuration(threshold))
	}
	return ""
}

// checkManyTransfers returns a non-empty reason when the number of transfer
// events (BLINDTRANSFER + ATTENDEDTRANSFER) exceeds threshold.
func checkManyTransfers(call model.Call, threshold int) string {
	count := 0
	for _, e := range call.Events {
		if e.Type == model.EventBlindTransfer || e.Type == model.EventAttendedTransfer {
			count++
		}
	}
	if count > threshold {
		return fmt.Sprintf("many-transfers: %d transfers (threshold %d)", count, threshold)
	}
	return ""
}

// checkCodecFailure returns a non-empty reason when SIP diagnostics indicate
// a codec or RTP setup problem. This requires --full-log to have been provided
// (otherwise SIPMessages and LogLines are empty and the check is a no-op).
func checkCodecFailure(call model.Call) string {
	// Check for RTP setup warnings via the sip diagnostics engine.
	warns := sip.Diagnose(call)
	for _, w := range warns {
		if w.Category == "RTP" {
			return fmt.Sprintf("codec-failure: %s", w.Message)
		}
	}

	// Check whether the INVITE had no negotiable codecs in its SDP (i.e. the
	// SDP was present but contained no a=rtpmap lines — a common symptom of
	// codec mismatch that prevents the call from bridging).
	if len(call.SIPMessages) > 0 {
		codecs := sip.Codecs(call.SIPMessages)
		// If we have SIP messages but no codecs, that may indicate a bare SDP
		// or non-SDP INVITE. Only flag when an SDP body was present but empty.
		if codecs == nil {
			hasSDPInvite := false
			for _, m := range call.SIPMessages {
				if m.Method == "INVITE" && m.Body != "" {
					hasSDPInvite = true
					break
				}
			}
			if hasSDPInvite {
				return "codec-failure: no codecs in INVITE SDP"
			}
		}
	}

	return ""
}

// formatDuration renders a duration in a human-readable format like "1h12m3s",
// "45s", or "500ms". It mirrors the style in the implementation guidance doc.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	switch {
	case h > 0 && m > 0 && s > 0:
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	case h > 0 && s > 0:
		return fmt.Sprintf("%dh%ds", h, s)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	case m > 0 && s > 0:
		return fmt.Sprintf("%dm%ds", m, s)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
