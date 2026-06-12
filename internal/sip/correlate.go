package sip

import "github.com/forgetdev/asterism/internal/model"

// AttachSIP correlates SIP messages to calls.
//
// Correlation is by LogCallID (the C-XXXXXXXX tag on the PJSIP logger line):
// for each call, collect the C-callids already present in its LogLines (from
// fulllog.AttachLog), then append SIP messages whose LogCallID matches.
//
// AttachSIP must be called after fulllog.AttachLog so that calls already have
// their LogLines populated with C-callids.
func AttachSIP(calls []model.Call, msgs []model.SIPMessage) []model.Call {
	for i := range calls {
		ids := logCallIDs(calls[i])
		if len(ids) == 0 {
			continue
		}
		for _, m := range msgs {
			if ids[m.LogCallID] {
				calls[i].SIPMessages = append(calls[i].SIPMessages, m)
			}
		}
	}
	return calls
}

// logCallIDs returns the set of C-XXXXXXXX identifiers seen in the call's
// log lines. Used as the join key for SIP messages.
func logCallIDs(call model.Call) map[string]bool {
	ids := make(map[string]bool)
	for _, l := range call.LogLines {
		if l.CallID != "" {
			ids[l.CallID] = true
		}
	}
	return ids
}
