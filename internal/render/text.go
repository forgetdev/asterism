// Package render produces human-readable output from Call data.
//
// v0 supports only text rendering to stdout. HTML and ladder diagrams come later.
package render

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/forgetdev/asterism/internal/model"
	"github.com/forgetdev/asterism/internal/q850"
)

// Text writes a textual summary of one or more calls to w.
// Each call is rendered as a header block followed by its event timeline.
//
// The format is designed for terminal reading at 80+ columns. We keep it
// close to what one would write by hand when debugging a call from logs —
// offset, event type, channel, then key details. Not optimized for machine
// parsing; for that we will add a JSON renderer later.
func Text(w io.Writer, calls []model.Call) error {
	for i, call := range calls {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if err := renderCall(w, call); err != nil {
			return err
		}
	}
	return nil
}

func renderCall(w io.Writer, call model.Call) error {
	channels := call.Channels()

	fmt.Fprintf(w, "═══════════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(w, "Call linkedid=%s\n", call.LinkedID)
	fmt.Fprintf(w, "  Start:        %s\n", call.StartTime().Format("2006-01-02 15:04:05.000"))
	fmt.Fprintf(w, "  Duration:     %s\n", effectiveDuration(call).Round(time.Millisecond))
	if result := callResult(call); result != "" {
		fmt.Fprintf(w, "  Result:       %s\n", result)
	}
	if caller := callCaller(call); caller != "" {
		fmt.Fprintf(w, "  Caller:       %s\n", caller)
	}
	if callee := callCallee(call); callee != "" {
		fmt.Fprintf(w, "  Callee:       %s\n", callee)
	}
	if bs := callBillSec(call); bs > 0 {
		fmt.Fprintf(w, "  BillSec:      %s\n", bs.Round(time.Second))
	}
	if ev := primaryHangup(call); ev != nil {
		if data, err := model.DecodeExtra(ev.Extra); err == nil {
			if data.HangupCauseSet {
				fmt.Fprintf(w, "  HangupCause:  %s\n", q850.Describe(data.HangupCause))
			}
			if data.DialStatus != "" {
				fmt.Fprintf(w, "  Dialstatus:   %s\n", data.DialStatus)
			}
		}
	}
	fmt.Fprintf(w, "  Channels:     %s\n", strings.Join(channels, ", "))
	fmt.Fprintf(w, "  Events:       %d\n", countVisible(call.Events))
	fmt.Fprintf(w, "───────────────────────────────────────────────────────────────────\n")

	callStart := call.StartTime()
	for _, ev := range call.Events {
		if ev.Type == model.EventLinkedIDEnd {
			continue
		}
		renderEvent(w, ev, callStart)
	}
	return nil
}

// renderEvent prints a single event line, formatted with offset from call start.
// Offset is more useful than absolute timestamp when scanning a call timeline.
func renderEvent(w io.Writer, ev model.Event, callStart time.Time) {
	offset := ev.Timestamp.Sub(callStart)
	fmt.Fprintf(w, "  [+%-9s] %-14s %s",
		formatOffset(offset),
		ev.Type,
		ev.ChannelName,
	)
	if ev.AppName != "" {
		fmt.Fprintf(w, "  %s(%s)", ev.AppName, cleanAppData(ev.AppData))
	}
	if detail := formatExtra(ev); detail != "" {
		fmt.Fprintf(w, "  %s", detail)
	}
	fmt.Fprintln(w)
}

// formatOffset renders a duration with adaptive precision. Events in a call
// often cluster within the same millisecond (channel setup happens in
// microseconds), and rounding everything to ms collapses them all to "0s",
// hiding their ordering. So: sub-millisecond offsets show microseconds;
// everything else rounds to milliseconds.
func formatOffset(d time.Duration) string {
	if d < time.Millisecond {
		// Show microseconds for tight clusters. time.Duration's String()
		// already does this well below 1ms (e.g., "184µs", "478µs").
		return d.String()
	}
	return d.Round(time.Millisecond).String()
}

// cleanAppData strips a redundant outer pair of parentheses from appdata.
// Asterisk's AppDial uses "(Outgoing Line)" as its data, and the renderer
// wraps app data in parens itself, producing "AppDial((Outgoing Line))".
// If the data already starts with '(' and ends with ')', we drop the inner
// pair so it reads "AppDial(Outgoing Line)".
func cleanAppData(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		return s[1 : len(s)-1]
	}
	return s
}

// callResult returns the call's disposition from its primary CDR, or empty
// string when no CDR was attached.
func callResult(call model.Call) string {
	if cdr := call.PrimaryCDR(); cdr != nil {
		return cdr.Disposition
	}
	return ""
}

// callCaller returns the caller extension. Prefers CDR.Src; falls back to
// the CIDNum of the first CHAN_START event in the CEL stream.
func callCaller(call model.Call) string {
	if cdr := call.PrimaryCDR(); cdr != nil && cdr.Src != "" {
		return cdr.Src
	}
	for _, e := range call.Events {
		if e.Type == model.EventChanStart && e.CIDNum != "" {
			return e.CIDNum
		}
	}
	return ""
}

// callCallee returns the callee extension. Prefers CDR.Dst; falls back to
// extracting the extension from the Dial AppData in the CEL stream.
// Known limitation: breaks on transfers — the Dial target of the first leg
// may not be the party that actually answered. Revisit in a later version.
func callCallee(call model.Call) string {
	if cdr := call.PrimaryCDR(); cdr != nil && cdr.Dst != "" {
		return cdr.Dst
	}
	for _, e := range call.Events {
		if e.Type == model.EventAppStart && strings.EqualFold(e.AppName, "Dial") {
			return dialTarget(e.AppData)
		}
	}
	return ""
}

// dialTarget extracts the extension from a Dial AppData string like
// "PJSIP/1001,30". Returns the extension portion (e.g. "1001"), or the
// full first argument if no technology prefix is present.
func dialTarget(appData string) string {
	arg := strings.SplitN(appData, ",", 2)[0]
	if idx := strings.LastIndex(arg, "/"); idx >= 0 {
		return arg[idx+1:]
	}
	return arg
}

// effectiveDuration returns the call's duration. Prefers CDR.Duration (whole
// seconds from the billing backend); falls back to the CEL wall-clock span.
func effectiveDuration(call model.Call) time.Duration {
	if cdr := call.PrimaryCDR(); cdr != nil && cdr.Duration > 0 {
		return cdr.Duration
	}
	return call.EndTime().Sub(call.StartTime())
}

// callBillSec returns the answered (billable) duration from the primary CDR,
// or zero when no CDR was attached or the call was never answered.
func callBillSec(call model.Call) time.Duration {
	if cdr := call.PrimaryCDR(); cdr != nil {
		return cdr.BillSec
	}
	return 0
}

// primaryHangup finds the HANGUP event for the originating channel (the one
// whose UniqueID equals the LinkedID). This event carries the final hangup
// cause and dialstatus for the call as a whole.
func primaryHangup(call model.Call) *model.Event {
	for i := range call.Events {
		e := &call.Events[i]
		if e.Type == model.EventHangup && e.UniqueID == call.LinkedID {
			return e
		}
	}
	return nil
}

// countVisible returns the number of events that will be rendered
// (i.e. excluding LINKEDID_END, which is filtered out as noise).
func countVisible(events []model.Event) int {
	n := 0
	for _, e := range events {
		if e.Type != model.EventLinkedIDEnd {
			n++
		}
	}
	return n
}

// formatExtra decodes the event's Extra JSON and renders only the fields
// relevant to its event type, in a compact human-readable form. Falls back
// to the raw Extra string if decoding fails (better to show something than
// to silently drop diagnostic data).
func formatExtra(ev model.Event) string {
	if ev.Extra == "" {
		return ""
	}
	data, err := model.DecodeExtra(ev.Extra)
	if err != nil {
		// Malformed JSON: show the raw blob rather than losing it.
		return ev.Extra
	}

	var parts []string

	switch ev.Type {
	case model.EventHangup:
		if data.HangupCauseSet {
			parts = append(parts, "cause="+q850.Describe(data.HangupCause))
		}
		if data.HangupSource != "" {
			parts = append(parts, "by="+data.HangupSource)
		}
		if data.DialStatus != "" {
			parts = append(parts, "dialstatus="+data.DialStatus)
		}
	case model.EventBridgeEnter, model.EventBridgeExit:
		if data.BridgeTechnology != "" {
			parts = append(parts, "tech="+data.BridgeTechnology)
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}
