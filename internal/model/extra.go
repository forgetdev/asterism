package model

import "encoding/json"

// ExtraData holds the decoded contents of a CEL event's eventextra field.
// Asterisk emits this as a JSON object whose keys vary by event type:
//
//   - HANGUP events carry hangupcause, hangupsource, dialstatus
//   - BRIDGE_ENTER / BRIDGE_EXIT carry bridge_id, bridge_technology
//   - other events may carry nothing
//
// All fields are optional. A field left at its zero value means it was absent
// from the JSON. Callers should check presence flags where the distinction
// between "absent" and "zero" matters (e.g., HangupCause 0 is a valid code).
type ExtraData struct {
	HangupCause      int
	HangupCauseSet   bool
	HangupSource     string
	DialStatus       string
	BridgeID         string
	BridgeTechnology string
	// Transfer fields (BLINDTRANSFER / ATTENDEDTRANSFER events).
	TransferExten  string // target extension, e.g. "1002"
	TransferContext string // target context, e.g. "internal"
	Transferee     string // channel being handed off
}

// rawExtra mirrors the JSON shape Asterisk emits. We unmarshal into this and
// then copy into ExtraData, because we want a presence flag for HangupCause
// (an int whose zero value 0 is itself a valid Q.850 code) which a plain
// struct field cannot express.
type rawExtra struct {
	HangupCause      *int    `json:"hangupcause"`
	HangupSource     *string `json:"hangupsource"`
	DialStatus       *string `json:"dialstatus"`
	BridgeID         *string `json:"bridge_id"`
	BridgeTechnology *string `json:"bridge_technology"`
	// Transfer fields.
	TransferExten   *string `json:"extension"`
	TransferContext *string `json:"context"`
	Transferee      *string `json:"transferee"`
}

// DecodeExtra parses an event's Extra string into a structured ExtraData.
// An empty string returns an empty ExtraData with no error. Malformed JSON
// returns an error so the caller can decide whether to surface or ignore it.
//
// We intentionally do not store the decoded result on the Event — decoding is
// done at render time, on demand. This keeps the parser fast and the model
// thin. If profiling later shows repeated decoding is a bottleneck, we can
// memoize, but premature caching is not worth the complexity now.
func DecodeExtra(s string) (ExtraData, error) {
	if s == "" {
		return ExtraData{}, nil
	}
	var raw rawExtra
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return ExtraData{}, err
	}

	var out ExtraData
	if raw.HangupCause != nil {
		out.HangupCause = *raw.HangupCause
		out.HangupCauseSet = true
	}
	if raw.HangupSource != nil {
		out.HangupSource = *raw.HangupSource
	}
	if raw.DialStatus != nil {
		out.DialStatus = *raw.DialStatus
	}
	if raw.BridgeID != nil {
		out.BridgeID = *raw.BridgeID
	}
	if raw.BridgeTechnology != nil {
		out.BridgeTechnology = *raw.BridgeTechnology
	}
	if raw.TransferExten != nil {
		out.TransferExten = *raw.TransferExten
	}
	if raw.TransferContext != nil {
		out.TransferContext = *raw.TransferContext
	}
	if raw.Transferee != nil {
		out.Transferee = *raw.Transferee
	}
	return out, nil
}
