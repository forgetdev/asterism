// Package model defines the core domain types for representing Asterisk calls.
//
// A Call is a logical unit of telephony activity, identified by its LinkedID.
// A Call contains one or more Channels (legs), each with its own UniqueID.
// Each Channel emits a sequence of Events during its lifetime.
//
// The relationship is strict: all Channels in a Call share the same LinkedID,
// and the LinkedID equals the UniqueID of the originating Channel.
package model

import "time"

// EventType enumerates the kinds of channel events Asterisk emits through CEL.
// This is intentionally a subset — asterism v0 only handles events relevant
// to reconstructing a basic call. Transfers, parks, and pickups come later.
type EventType string

const (
	EventChanStart    EventType = "CHAN_START"
	EventChanEnd      EventType = "CHAN_END"
	EventAnswer       EventType = "ANSWER"
	EventHangup       EventType = "HANGUP"
	EventAppStart     EventType = "APP_START"
	EventAppEnd       EventType = "APP_END"
	EventBridgeEnter  EventType = "BRIDGE_ENTER"
	EventBridgeExit   EventType = "BRIDGE_EXIT"
	EventLinkedIDEnd  EventType = "LINKEDID_END"
)

// Event is a single CEL row, parsed into a typed structure.
// The Extra field holds the raw JSON-ish blob from the eventextra column,
// which carries hangupcause, bridge_id, dialstatus, etc. We do not unmarshal
// it eagerly because the schema varies by event type — render-time decoding
// is more honest.
type Event struct {
	Type        EventType
	Timestamp   time.Time
	CIDNum      string
	CIDName     string
	ChannelName string
	Exten       string
	Context     string
	UniqueID    string
	LinkedID    string
	BridgePeer  string
	AppName     string
	AppData     string
	Extra       string
}

// Call represents a single logical call, grouping all events sharing a LinkedID.
// Events are kept in arrival order; sorting by Timestamp is done at correlation time.
type Call struct {
	LinkedID string
	Events   []Event
}

// Channels returns the distinct channel names observed in this call.
// Useful for cross-referencing with the full log later.
func (c *Call) Channels() []string {
	seen := make(map[string]bool)
	var out []string
	for _, e := range c.Events {
		if e.ChannelName == "" {
			continue
		}
		if !seen[e.ChannelName] {
			seen[e.ChannelName] = true
			out = append(out, e.ChannelName)
		}
	}
	return out
}

// StartTime returns the earliest event timestamp in the call.
// Returns zero time if the call has no events (should not happen in practice).
func (c *Call) StartTime() time.Time {
	if len(c.Events) == 0 {
		return time.Time{}
	}
	earliest := c.Events[0].Timestamp
	for _, e := range c.Events[1:] {
		if e.Timestamp.Before(earliest) {
			earliest = e.Timestamp
		}
	}
	return earliest
}

// EndTime returns the latest event timestamp in the call.
func (c *Call) EndTime() time.Time {
	if len(c.Events) == 0 {
		return time.Time{}
	}
	latest := c.Events[0].Timestamp
	for _, e := range c.Events[1:] {
		if e.Timestamp.After(latest) {
			latest = e.Timestamp
		}
	}
	return latest
}
