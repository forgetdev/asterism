package model

import "time"

// RegistrationFailure represents a failed SIP REGISTER attempt logged by
// Asterisk's pjsip_distributor module. These are not tied to any specific
// call; they appear as NOTICE lines in the full log.
type RegistrationFailure struct {
	Timestamp time.Time
	Endpoint  string // Asterisk endpoint/extension, e.g. "1001"
	From      string // full From header value, e.g. '"user" <sip:1001@x.x.x.x>'
	SourceIP  string // source IP:port, e.g. "170.81.69.34:43931"
	CallID    string // SIP Call-ID of the failed REGISTER
	Reason    string // failure reason, e.g. "Failed to authenticate"
}
