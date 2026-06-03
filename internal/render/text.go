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
)

// Text writes a textual summary of one or more calls to w.
// Each call is rendered as a header block followed by its event timeline.
//
// The format is designed for terminal reading at 80+ columns. We keep it
// close to what one would write by hand when debugging a call from logs —
// timestamp, event type, channel, then key details. Not optimized for
// machine parsing; for that we will add a JSON renderer later.
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

	for _, ev := range call.Events {
		renderEvent(w, ev, call.StartTime())
	}
	return nil
}

// renderEvent prints a single event line, formatted with offset from call start.
// Offset is more useful than absolute timestamp when scanning a call timeline.
func renderEvent(w io.Writer, ev model.Event, callStart time.Time) {
	offset := ev.Timestamp.Sub(callStart)
	fmt.Fprintf(w, "  [+%-9s] %-14s %s",
		offset.Round(time.Millisecond),
		ev.Type,
		ev.ChannelName,
	)
	if ev.AppName != "" {
		fmt.Fprintf(w, "  %s(%s)", ev.AppName, ev.AppData)
	}
	if ev.Extra != "" {
		fmt.Fprintf(w, "  %s", ev.Extra)
	}
	fmt.Fprintln(w)
}
