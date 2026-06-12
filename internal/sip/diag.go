package sip

import (
	"strings"

	"github.com/forgetdev/asterism/internal/model"
)

// Warning is a human-readable diagnostic notice about a call's SIP signaling
// or media. Included in call header output when issues are detected.
type Warning struct {
	Category string // "SIP", "RTP", "MEDIA"
	Message  string
}

// Diagnose scans SIP messages and log lines for known issues and returns any
// warnings found. Returns nil when nothing unusual is detected.
func Diagnose(call model.Call) []Warning {
	var warns []Warning
	warns = append(warns, checkFailedResponses(call.SIPMessages)...)
	warns = append(warns, checkNativeRTPWarnings(call.LogLines)...)
	return warns
}

// Codecs extracts the codec names offered in the first INVITE SDP, if present.
// Returns nil when no INVITE with SDP is found.
func Codecs(msgs []model.SIPMessage) []string {
	for i := range msgs {
		m := &msgs[i]
		if m.Method == "INVITE" &&
			strings.Contains(strings.ToLower(m.ContentType), "sdp") &&
			m.Body != "" {
			return parseSDPCodecs(m.Body)
		}
	}
	return nil
}

func checkFailedResponses(msgs []model.SIPMessage) []Warning {
	var warns []Warning
	seen := make(map[int]bool)
	for _, m := range msgs {
		if m.StatusCode >= 400 && !seen[m.StatusCode] {
			seen[m.StatusCode] = true
			warns = append(warns, Warning{
				Category: "SIP",
				Message:  "received " + m.StatusCode100str() + " " + m.StatusText,
			})
		}
	}
	return warns
}

func checkNativeRTPWarnings(lines []model.LogLine) []Warning {
	var warns []Warning
	for _, l := range lines {
		if l.Level == "WARNING" && strings.Contains(l.Message, "native_rtp") {
			warns = append(warns, Warning{
				Category: "RTP",
				Message:  l.Message,
			})
		}
	}
	return warns
}

// parseSDPCodecs extracts codec names from a=rtpmap lines in an SDP body.
// Returns e.g. ["PCMU", "PCMA", "telephone-event"].
func parseSDPCodecs(sdp string) []string {
	var codecs []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(sdp, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "a=rtpmap:") {
			continue
		}
		// a=rtpmap:0 PCMU/8000  →  extract "PCMU"
		rest := line[9:]
		idx := strings.IndexByte(rest, ' ')
		if idx < 0 {
			continue
		}
		encoding := rest[idx+1:]
		if slash := strings.IndexByte(encoding, '/'); slash >= 0 {
			encoding = encoding[:slash]
		}
		if !seen[encoding] {
			seen[encoding] = true
			codecs = append(codecs, encoding)
		}
	}
	return codecs
}
