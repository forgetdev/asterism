// Package filter selects calls and events from a parsed CEL stream.
//
// There are two levels of filtering:
//
//   - Call-level: LinkedID, Channel, Extension — a call that doesn't match is
//     dropped entirely from the output.
//   - Event-level: EventTypes — events within a kept call are further reduced
//     to only those whose type appears in the list.
//
// Filters compose: all call-level predicates must pass for a call to survive.
// An empty value (empty string / nil slice) means "no restriction on this field".
package filter

import (
	"strings"

	"github.com/forgetdev/asterism/internal/model"
)

// Options controls which calls are included and which events are shown within them.
type Options struct {
	LinkedID   string            // keep only the call with this LinkedID
	Channel    string            // keep only calls containing this channel name (substring match)
	Extension  string            // keep only calls where any event has this Exten
	EventTypes []model.EventType // within kept calls, show only these event types (nil = all)
}

// Calls applies the call-level predicates in opts and returns matching calls.
// EventTypes is not applied here — pass the result to Events for that.
func Calls(calls []model.Call, opts Options) []model.Call {
	if opts.LinkedID == "" && opts.Channel == "" && opts.Extension == "" {
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
