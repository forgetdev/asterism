// Package q850 translates ITU-T Q.850 hangup cause codes into human-readable
// names. Asterisk emits these codes in CEL HANGUP events as integers (e.g.,
// hangupcause:16). The raw number is meaningless to most readers; this package
// maps it to the standard name (e.g., NORMAL_CLEARING).
//
// The codes follow the Q.850 standard as used by Asterisk. Not every code in
// the 1-127 range is defined — Asterisk uses a well-known subset. Unknown codes
// are returned as "UNKNOWN" so the caller can still display the number.
//
// Reference: ITU-T Recommendation Q.850, and Asterisk's causes.h.
package q850

import "fmt"

// names maps Q.850 cause codes to their standard symbolic names.
// This is the subset Asterisk actually emits in practice. Adding a code here
// is safe; the lookup degrades gracefully for anything absent.
var names = map[int]string{
	0:   "UNALLOCATED_NUMBER_OR_UNKNOWN",
	1:   "UNALLOCATED_NUMBER",
	2:   "NO_ROUTE_TRANSIT_NET",
	3:   "NO_ROUTE_DESTINATION",
	5:   "MISDIALLED_TRUNK_PREFIX",
	6:   "CHANNEL_UNACCEPTABLE",
	7:   "CALL_AWARDED_DELIVERED",
	8:   "PRE_EMPTED",
	14:  "NUMBER_PORTED_NOT_HERE",
	16:  "NORMAL_CLEARING",
	17:  "USER_BUSY",
	18:  "NO_USER_RESPONSE",
	19:  "NO_ANSWER",
	20:  "SUBSCRIBER_ABSENT",
	21:  "CALL_REJECTED",
	22:  "NUMBER_CHANGED",
	23:  "REDIRECTED_TO_NEW_DESTINATION",
	26:  "ANSWERED_ELSEWHERE",
	27:  "DESTINATION_OUT_OF_ORDER",
	28:  "INVALID_NUMBER_FORMAT",
	29:  "FACILITY_REJECTED",
	30:  "RESPONSE_TO_STATUS_ENQUIRY",
	31:  "NORMAL_UNSPECIFIED",
	34:  "NORMAL_CIRCUIT_CONGESTION",
	38:  "NETWORK_OUT_OF_ORDER",
	41:  "NORMAL_TEMPORARY_FAILURE",
	42:  "SWITCH_CONGESTION",
	43:  "ACCESS_INFO_DISCARDED",
	44:  "REQUESTED_CHAN_UNAVAIL",
	45:  "PRE_EMPTED_45",
	50:  "FACILITY_NOT_SUBSCRIBED",
	52:  "OUTGOING_CALL_BARRED",
	54:  "INCOMING_CALL_BARRED",
	57:  "BEARERCAPABILITY_NOTAUTH",
	58:  "BEARERCAPABILITY_NOTAVAIL",
	65:  "BEARERCAPABILITY_NOTIMPL",
	66:  "CHAN_NOT_IMPLEMENTED",
	69:  "FACILITY_NOT_IMPLEMENTED",
	81:  "INVALID_CALL_REFERENCE",
	88:  "INCOMPATIBLE_DESTINATION",
	95:  "INVALID_MSG_UNSPECIFIED",
	96:  "MANDATORY_IE_MISSING",
	97:  "MESSAGE_TYPE_NONEXIST",
	98:  "WRONG_MESSAGE",
	99:  "IE_NONEXIST",
	100: "INVALID_IE_CONTENTS",
	101: "WRONG_CALL_STATE",
	102: "RECOVERY_ON_TIMER_EXPIRE",
	103: "MANDATORY_IE_LENGTH_ERROR",
	111: "PROTOCOL_ERROR",
	127: "INTERWORKING",
}

// Name returns the symbolic name for a Q.850 cause code.
// Unknown codes return "UNKNOWN".
func Name(code int) string {
	if n, ok := names[code]; ok {
		return n
	}
	return "UNKNOWN"
}

// Describe returns a human-friendly string combining name and code,
// e.g., "NORMAL_CLEARING (16)". This is what renderers should display.
func Describe(code int) string {
	return fmt.Sprintf("%s (%d)", Name(code), code)
}
