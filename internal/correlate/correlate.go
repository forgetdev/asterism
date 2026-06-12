// Package correlate groups CEL events into Calls by LinkedID.
//
// Correlation is pure: it does not read files, make decisions about presentation,
// or modify event data. It receives a flat event list and returns a Call list
// where each Call holds the events that share its LinkedID, sorted chronologically.
package correlate

import (
	"sort"

	"github.com/forgetdev/asterism/internal/model"
)

// ByLinkedID groups events into calls by their LinkedID field.
// The returned calls are sorted by start time (earliest first).
// Events within each call are sorted by timestamp ascending.
//
// Events with empty LinkedID are dropped. In well-formed CEL output this
// should never happen; if it does, it indicates a parser or Asterisk bug
// worth investigating. We do not log here — the caller decides what to
// do about anomalies. Returning them silently is the lesser evil for v0.
//
// TODO(v0.0.2): expose dropped event count via a return param or log hook.
func ByLinkedID(events []model.Event) []model.Call {
	groups := make(map[string][]model.Event)
	for _, e := range events {
		if e.LinkedID == "" {
			continue
		}
		groups[e.LinkedID] = append(groups[e.LinkedID], e)
	}

	calls := make([]model.Call, 0, len(groups))
	for linkedID, evs := range groups {
		sort.Slice(evs, func(i, j int) bool {
			return evs[i].Timestamp.Before(evs[j].Timestamp)
		})
		calls = append(calls, model.Call{
			LinkedID: linkedID,
			Events:   evs,
		})
	}

	sort.Slice(calls, func(i, j int) bool {
		return calls[i].StartTime().Before(calls[j].StartTime())
	})
	return calls
}

// AttachCDR enriches calls with their Call Detail Records, joining on UniqueID.
//
// A CDR carries no LinkedID, only the UniqueID of the channel it bills. We
// therefore index every channel UniqueID seen in each call's events and assign
// a CDR to the call that owns its UniqueID. This catches per-leg CDRs whose
// UniqueID is a non-originating channel, not just the call's primary record.
//
// Like ByLinkedID, this is pure: it mutates the passed calls in place (filling
// their CDRs slice) and returns them for chaining. CDRs whose UniqueID matches
// no known channel are dropped — they belong to calls absent from the CEL
// stream, which asterism cannot render anyway.
func AttachCDR(calls []model.Call, cdrs []model.CDR) []model.Call {
	// Map each channel UniqueID to its call's index. The originating channel's
	// UniqueID equals the LinkedID, but dialed legs have their own UniqueIDs,
	// so we index all of them.
	index := make(map[string]int)
	for i := range calls {
		for _, e := range calls[i].Events {
			if e.UniqueID != "" {
				index[e.UniqueID] = i
			}
		}
	}

	for _, rec := range cdrs {
		if i, ok := index[rec.UniqueID]; ok {
			calls[i].CDRs = append(calls[i].CDRs, rec)
		}
	}
	return calls
}
