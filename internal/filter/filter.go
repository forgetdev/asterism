// Package filter selects calls and events from a parsed CEL stream.
//
// There are two levels of filtering:
//
//   - Call-level: LinkedID, Channel, Extension, From, To, MinDuration,
//     MaxDuration, HangupCause — a call that doesn't match is dropped entirely.
//   - Event-level: EventTypes — events within a kept call are further reduced
//     to only those whose type appears in the list.
//
// Filters compose: all call-level predicates must pass for a call to survive.
// An empty/zero value means "no restriction on this field".
package filter

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/forgetdev/asterism/internal/model"
	"github.com/forgetdev/asterism/internal/q850"
)

// Options controls which calls are included and which events are shown within them.
type Options struct {
	LinkedID    string            // keep only the call with this LinkedID
	Channel     string            // keep only calls containing this channel name (substring)
	Extension   string            // keep only calls where any event has this Exten
	From        time.Time         // keep only calls starting at or after this time (zero = no bound)
	To          time.Time         // keep only calls starting at or before this time (zero = no bound)
	MinDuration time.Duration     // keep only calls with duration >= this (0 = no minimum)
	MaxDuration time.Duration     // keep only calls with duration <= this (0 = no maximum)
	HangupCause string            // keep only calls with this hangup cause — name ("NORMAL_CLEARING") or code ("16")
	EventTypes  []model.EventType // within kept calls, show only these event types (nil = all)
}

// Calls applies the call-level predicates in opts and returns matching calls.
// EventTypes is not applied here — pass the result to Events for that.
func Calls(calls []model.Call, opts Options) []model.Call {
	if !opts.hasCallFilter() {
		return calls
	}
	out := make([]model.Call, 0, len(calls))
	for _, c := range calls {
		if opts.LinkedID != "" && c.LinkedID != opts.LinkedID {
			continue
		}
		if opts.Channel != "" && !callHasChannel(c, opts.Channel) {
			continue
		}
		if opts.Extension != "" && !callHasExtension(c, opts.Extension) {
			continue
		}
		start := c.StartTime()
		if !opts.From.IsZero() && start.Before(opts.From) {
			continue
		}
		if !opts.To.IsZero() && start.After(opts.To) {
			continue
		}
		dur := callDuration(c)
		if opts.MinDuration > 0 && dur < opts.MinDuration {
			continue
		}
		if opts.MaxDuration > 0 && dur > opts.MaxDuration {
			continue
		}
		if opts.HangupCause != "" && !callMatchesHangupCause(c, opts.HangupCause) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// Events filters the event list within each call to the types listed in types.
// Returns a new slice of calls with filtered event lists; calls with no matching
// events are still included (their header is still useful for diagnosis).
// If types is empty, calls is returned unchanged.
func Events(calls []model.Call, types []model.EventType) []model.Call {
	if len(types) == 0 {
		return calls
	}
	allowed := make(map[model.EventType]bool, len(types))
	for _, t := range types {
		allowed[t] = true
	}
	out := make([]model.Call, len(calls))
	for i, c := range calls {
		filtered := c
		filtered.Events = nil
		for _, e := range c.Events {
			if allowed[e.Type] {
				filtered.Events = append(filtered.Events, e)
			}
		}
		out[i] = filtered
	}
	return out
}

// ParseEventTypes splits a comma-separated event type string (e.g.
// "HANGUP,APP_START,APP_END") into a slice of EventType values.
// Whitespace around commas is trimmed. An empty string returns nil.
func ParseEventTypes(s string) []model.EventType {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]model.EventType, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, model.EventType(strings.ToUpper(p)))
		}
	}
	return out
}

// ParseTime parses a timestamp string for --from / --to flags. Accepted formats:
// "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02". Returns an error
// if none match.
func ParseTime(s string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse %q — use YYYY-MM-DD or YYYY-MM-DD HH:MM:SS", s)
}

// hasCallFilter reports whether any call-level predicate is set.
func (o Options) hasCallFilter() bool {
	return o.LinkedID != "" || o.Channel != "" || o.Extension != "" ||
		!o.From.IsZero() || !o.To.IsZero() ||
		o.MinDuration != 0 || o.MaxDuration != 0 ||
		o.HangupCause != ""
}

// callDuration returns the wall-clock span of a call from CEL start to end.
// We use CEL times (not CDR) so transferred calls report the full duration.
func callDuration(c model.Call) time.Duration {
	return c.EndTime().Sub(c.StartTime())
}

// callHangupCode extracts the Q.850 cause code from the originating channel's
// HANGUP event, returning (code, true) or (0, false) if none is found.
func callHangupCode(c model.Call) (int, bool) {
	for _, e := range c.Events {
		if e.Type == model.EventHangup && e.UniqueID == c.LinkedID {
			if data, err := model.DecodeExtra(e.Extra); err == nil && data.HangupCauseSet {
				return data.HangupCause, true
			}
		}
	}
	return 0, false
}

// callMatchesHangupCause reports whether the call's primary hangup cause matches
// the user-supplied string. cause may be a numeric code ("16") or a name
// substring ("NORMAL_CLEARING", "BUSY"). Matching is case-insensitive.
func callMatchesHangupCause(c model.Call, cause string) bool {
	code, ok := callHangupCode(c)
	if !ok {
		return false
	}
	if n, err := strconv.Atoi(cause); err == nil {
		return code == n
	}
	desc := strings.ToLower(q850.Describe(code))
	return strings.Contains(desc, strings.ToLower(cause))
}

func callHasChannel(c model.Call, name string) bool {
	for _, e := range c.Events {
		if strings.Contains(e.ChannelName, name) {
			return true
		}
	}
	return false
}

func callHasExtension(c model.Call, exten string) bool {
	for _, e := range c.Events {
		if e.Exten == exten {
			return true
		}
	}
	return false
}
