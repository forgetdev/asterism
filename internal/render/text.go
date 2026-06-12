// Package render produces human-readable output from Call data.
//
// Text produces a terminal-friendly call timeline. JSON produces a
// machine-readable representation. Both share the same helper functions
// for extracting call-level summary fields.
package render

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/forgetdev/asterism/internal/model"
	"github.com/forgetdev/asterism/internal/q850"
	"github.com/forgetdev/asterism/internal/sip"
)

// ANSI color codes used by the text renderer.
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiBlue   = "\033[34m"
	ansiCyan   = "\033[36m"
)

// TextOptions controls text rendering behaviour.
type TextOptions struct {
	Color bool // enable ANSI color codes
}

// Text writes a textual summary of one or more calls to w.
// Each call is rendered as a header block followed by its event timeline.
//
// The format is designed for terminal reading at 80+ columns. We keep it
// close to what one would write by hand when debugging a call from logs —
// offset, event type, channel, then key details. Not optimized for machine
// parsing; use JSON for that.
func Text(w io.Writer, calls []model.Call, opts TextOptions) error {
	r := &textRenderer{w: w, color: opts.Color}
	for i, call := range calls {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if err := r.renderCall(call); err != nil {
			return err
		}
	}
	return nil
}

// textRenderer holds the writer and options so they don't need to be threaded
// through every helper call.
type textRenderer struct {
	w     io.Writer
	color bool
}

func (r *textRenderer) bold(s string) string {
	if !r.color {
		return s
	}
	return ansiBold + s + ansiReset
}

func (r *textRenderer) dim(s string) string {
	if !r.color {
		return s
	}
	return ansiDim + s + ansiReset
}

func (r *textRenderer) colorWarn(s string) string {
	if !r.color {
		return s
	}
	return ansiYellow + s + ansiReset
}

func (r *textRenderer) colorResult(result string) string {
	if !r.color {
		return result
	}
	var code string
	switch result {
	case "ANSWERED":
		code = ansiGreen
	case "BUSY", "NO ANSWER":
		code = ansiYellow
	case "FAILED", "CONGESTION":
		code = ansiRed
	default:
		return result
	}
	return code + result + ansiReset
}

func (r *textRenderer) colorEventType(t model.EventType) string {
	s := string(t)
	if !r.color {
		return s
	}
	var code string
	switch t {
	case model.EventChanStart, model.EventChanEnd:
		code = ansiCyan
	case model.EventAnswer:
		code = ansiGreen
	case model.EventHangup:
		code = ansiYellow
	case model.EventBridgeEnter, model.EventBridgeExit:
		code = ansiBlue
	default:
		return s
	}
	return code + s + ansiReset
}

func (r *textRenderer) renderCall(call model.Call) error {
	channels := call.Channels()

	fmt.Fprintf(r.w, "%s\n", r.bold("═══════════════════════════════════════════════════════════════════"))
	fmt.Fprintf(r.w, "%s\n", r.bold("Call linkedid="+call.LinkedID))
	fmt.Fprintf(r.w, "  Start:        %s\n", call.StartTime().Format("2006-01-02 15:04:05.000"))
	fmt.Fprintf(r.w, "  Duration:     %s\n", effectiveDuration(call).Round(time.Millisecond))
	if result := callResult(call); result != "" {
		fmt.Fprintf(r.w, "  Result:       %s\n", r.colorResult(result))
	}
	if caller := callCaller(call); caller != "" {
		fmt.Fprintf(r.w, "  Caller:       %s\n", caller)
	}
	if callee := callCallee(call); callee != "" {
		fmt.Fprintf(r.w, "  Callee:       %s\n", callee)
	}
	if bs := callBillSec(call); bs > 0 {
		fmt.Fprintf(r.w, "  BillSec:      %s\n", bs.Round(time.Second))
	}
	if ev := primaryHangup(call); ev != nil {
		if data, err := model.DecodeExtra(ev.Extra); err == nil {
			if data.HangupCauseSet {
				fmt.Fprintf(r.w, "  HangupCause:  %s\n", q850.Describe(data.HangupCause))
			}
			if data.DialStatus != "" {
				fmt.Fprintf(r.w, "  Dialstatus:   %s\n", data.DialStatus)
			}
		}
	}
	fmt.Fprintf(r.w, "  Channels:     %s\n", strings.Join(channels, ", "))
	fmt.Fprintf(r.w, "  Events:       %d\n", countVisible(call.Events))
	if len(call.SIPMessages) > 0 {
		fmt.Fprintf(r.w, "  SIP msgs:     %d", len(call.SIPMessages))
		if callID := sipCallID(call.SIPMessages); callID != "" {
			fmt.Fprintf(r.w, "  (Call-ID: %s)", callID)
		}
		fmt.Fprintln(r.w)
		if codecs := sip.Codecs(call.SIPMessages); len(codecs) > 0 {
			fmt.Fprintf(r.w, "  Codecs:       %s\n", strings.Join(codecs, ", "))
		}
	}
	if warns := sip.Diagnose(call); len(warns) > 0 {
		for _, w := range warns {
			fmt.Fprintf(r.w, "  %-14s%s\n", r.colorWarn("! "+w.Category+":"), w.Message)
		}
	}
	fmt.Fprintf(r.w, "%s\n", r.bold("───────────────────────────────────────────────────────────────────"))

	callStart := call.StartTime()
	r.renderTimeline(call, callStart)
	return nil
}

// renderTimeline interleaves CEL events, full log lines, and SIP messages in
// chronological order. LINKEDID_END events are suppressed.
func (r *textRenderer) renderTimeline(call model.Call, callStart time.Time) {
	type item struct {
		t      time.Time
		event  *model.Event
		logLine *model.LogLine
		sipMsg  *model.SIPMessage
	}

	var items []item
	for i := range call.Events {
		e := &call.Events[i]
		if e.Type == model.EventLinkedIDEnd {
			continue
		}
		items = append(items, item{t: e.Timestamp, event: e})
	}
	for i := range call.LogLines {
		l := &call.LogLines[i]
		items = append(items, item{t: l.Timestamp, logLine: l})
	}
	for i := range call.SIPMessages {
		m := &call.SIPMessages[i]
		items = append(items, item{t: m.Timestamp, sipMsg: m})
	}

	// Stable insertion sort — slices are small and already nearly sorted.
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].t.Before(items[j-1].t); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}

	for _, it := range items {
		switch {
		case it.event != nil:
			r.renderEvent(*it.event, callStart)
		case it.logLine != nil:
			r.renderLogLine(*it.logLine, callStart)
		case it.sipMsg != nil:
			r.renderSIPMessage(*it.sipMsg, callStart)
		}
	}
}

// renderLogLine prints a single full log line in the timeline.
func (r *textRenderer) renderLogLine(l model.LogLine, callStart time.Time) {
	offset := l.Timestamp.Sub(callStart)
	msg := l.Message
	if r.color {
		msg = ansiDim + msg + ansiReset
	}
	fmt.Fprintf(r.w, "  [%s] %-14s %s\n",
		r.dim("+"+fmt.Sprintf("%-9s", formatOffset(offset))),
		r.dim("LOG "+l.Level),
		msg,
	)
}

// renderSIPMessage prints a single SIP message in the timeline.
// Format: [+Xs] SIP RX/TX    METHOD or STATUS addr
func (r *textRenderer) renderSIPMessage(m model.SIPMessage, callStart time.Time) {
	offset := m.Timestamp.Sub(callStart)
	dirLabel := "SIP RX"
	arrow := "from"
	if m.Direction == model.SIPTx {
		dirLabel = "SIP TX"
		arrow = "→"
	}
	detail := m.Summary() + " " + arrow + " " + m.RemoteAddr
	if r.color {
		detail = ansiCyan + detail + ansiReset
	}
	fmt.Fprintf(r.w, "  [%s] %-14s %s\n",
		r.dim("+"+fmt.Sprintf("%-9s", formatOffset(offset))),
		r.dim(dirLabel),
		detail,
	)
}

// sipCallID returns the first non-empty SIP Call-ID seen in msgs.
func sipCallID(msgs []model.SIPMessage) string {
	for _, m := range msgs {
		if m.CallID != "" {
			return m.CallID
		}
	}
	return ""
}

// renderEvent prints a single event line, formatted with offset from call start.
// Offset is more useful than absolute timestamp when scanning a call timeline.
func (r *textRenderer) renderEvent(ev model.Event, callStart time.Time) {
	offset := ev.Timestamp.Sub(callStart)
	fmt.Fprintf(r.w, "  [%s] %-14s %s",
		r.dim("+"+fmt.Sprintf("%-9s", formatOffset(offset))),
		r.colorEventType(ev.Type),
		ev.ChannelName,
	)
	if ev.AppName != "" {
		fmt.Fprintf(r.w, "  %s(%s)", ev.AppName, cleanAppData(ev.AppData))
	}
	if detail := formatExtra(ev); detail != "" {
		fmt.Fprintf(r.w, "  %s", detail)
	}
	fmt.Fprintln(r.w)
}

// formatOffset renders a duration with adaptive precision. Events in a call
// often cluster within the same millisecond (channel setup happens in
// microseconds), and rounding everything to ms collapses them all to "0s",
// hiding their ordering. So: sub-millisecond offsets show microseconds;
// everything else rounds to milliseconds.
func formatOffset(d time.Duration) string {
	if d < time.Millisecond {
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
