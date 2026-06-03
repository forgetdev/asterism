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
	duration := call.EndTime().Sub(call.StartTime())

	fmt.Fprintf(w, "═══════════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(w, "Call linkedid=%s\n", call.LinkedID)
	fmt.Fprintf(w, "  Start:    %s\n", call.StartTime().Format("2006-01-02 15:04:05.000"))
	fmt.Fprintf(w, "  Duration: %s\n", duration.Round(time.Millisecond))
	fmt.Fprintf(w, "  Channels: %s\n", strings.Join(channels, ", "))
	fmt.Fprintf(w, "  Events:   %d\n", len(call.Events))
	fmt.Fprintf(w, "───────────────────────────────────────────────────────────────────\n")

	callStart := call.StartTime()
	for _, ev := range call.Events {
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
