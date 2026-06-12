package model

import "time"

// SIPDirection indicates whether a SIP message was received from or
// transmitted to a remote endpoint.
type SIPDirection int

const (
	SIPRx SIPDirection = iota // received from remote
	SIPTx                     // transmitted to remote
)

// SIPMessage is a single SIP request or response captured by the PJSIP logger
// in the Asterisk full log. Requires "pjsip set logger on" to be active.
//
// SIP messages in the log are multi-line; the parser assembles them from the
// PJSIP envelope header through the "<----------->" terminator line.
type SIPMessage struct {
	Timestamp  time.Time
	Direction  SIPDirection
	Transport  string // "UDP", "TCP", "TLS"
	RemoteAddr string // "ip:port"
	LogCallID  string // C-XXXXXXXX from the log line (links to LogLine.CallID)

	// Parsed from the SIP start line.
	Method     string // "INVITE", "BYE", "ACK", "CANCEL", "OPTIONS", "" for responses
	StatusCode int    // 0 for requests; e.g. 200, 486
	StatusText string // "OK", "Busy Here", "" for requests

	// Key headers.
	CallID      string // Call-ID header
	From        string // From header value
	To          string // To header value
	CSeq        string // CSeq header value
	ContentType string // Content-Type header

	// Body (SDP, etc.)
	Body string
}

// IsRequest reports whether this is a SIP request (as opposed to a response).
func (m *SIPMessage) IsRequest() bool { return m.Method != "" }

// IsResponse reports whether this is a SIP response.
func (m *SIPMessage) IsResponse() bool { return m.StatusCode != 0 }

// Summary returns a one-line description: "INVITE" for requests,
// "200 OK" for responses.
func (m *SIPMessage) Summary() string {
	if m.IsRequest() {
		return m.Method
	}
	if m.StatusText != "" {
		return m.StatusCode100str() + " " + m.StatusText
	}
	return m.StatusCode100str()
}

// StatusCode100str returns the numeric status code as a string.
// Named to avoid collision with the int field.
func (m *SIPMessage) StatusCode100str() string {
	if m.StatusCode == 0 {
		return ""
	}
	return itoa(m.StatusCode)
}

// itoa converts a small positive int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [10]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
