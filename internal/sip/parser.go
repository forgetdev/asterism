// Package sip parses SIP messages from the Asterisk full log.
//
// When PJSIP request/response logging is enabled ("pjsip set logger on"),
// Asterisk writes each SIP message inline in the full log, wrapped by an
// envelope header and a terminator:
//
//	[Mon DD HH:MM:SS] VERBOSE[tid][C-callid] res_pjsip_logger.c: <--- Received SIP request (N bytes) from UDP:ip:port --->
//	INVITE sip:1001@pbx SIP/2.0
//	Via: ...
//	...
//	[blank line]
//	[optional body]
//	<----------->
//
// The parser is a simple state machine that recognises envelope lines and
// accumulates subsequent lines until the terminator. Non-SIP log lines are
// silently skipped.
package sip

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/forgetdev/asterism/internal/model"
)

// envelopeRE matches the PJSIP logger header that precedes each SIP message.
// Capture groups: (timestamp) (C-callid, optional) (direction) (transport:addr)
var envelopeRE = regexp.MustCompile(
	`^\[([A-Za-z]+ +\d+ \d{2}:\d{2}:\d{2})\] ` +
		`\w+\[\d+\](?:\[(C-[0-9A-Fa-f]+)\])? ` +
		`res_pjsip_logger\.c: <--- (Received|Transmitting) SIP (?:request|response) \(\d+ bytes\) (?:from|to) (\S+) --->$`,
)

// termRE matches the "<----------->" terminator that ends each SIP message.
var termRE = regexp.MustCompile(`^<-{3,}>$`)

const timestampLayout = "Jan _2 15:04:05"

// Parse reads an Asterisk full log from r and returns all SIP messages found.
// Non-SIP lines (dialplan, warnings, etc.) are silently skipped.
func Parse(r io.Reader, year int) ([]model.SIPMessage, error) {
	var msgs []model.SIPMessage
	scanner := bufio.NewScanner(r)

	var (
		inSIP       bool
		current     model.SIPMessage
		headerLines []string
		bodyLines   []string
		inBody      bool
	)

	for scanner.Scan() {
		line := scanner.Text()

		if !inSIP {
			m := envelopeRE.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			ts, err := parseTimestamp(m[1], year)
			if err != nil {
				continue
			}
			transport, addr := splitTransportAddr(m[4])
			current = model.SIPMessage{
				Timestamp:  ts,
				LogCallID:  m[2],
				Transport:  transport,
				RemoteAddr: addr,
			}
			if m[3] == "Received" {
				current.Direction = model.SIPRx
			} else {
				current.Direction = model.SIPTx
			}
			inSIP = true
			headerLines = headerLines[:0]
			bodyLines = bodyLines[:0]
			inBody = false
			continue
		}

		// Inside a SIP message.
		if termRE.MatchString(line) {
			if err := parseSIPContent(headerLines, bodyLines, &current); err == nil {
				msgs = append(msgs, current)
			}
			inSIP = false
			continue
		}

		if !inBody {
			if line == "" {
				inBody = true
			} else {
				headerLines = append(headerLines, line)
			}
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("sip: scanning: %w", err)
	}
	return msgs, nil
}

// ParseFile is a convenience wrapper that opens path and parses it.
// Pass 0 for year to use the current year.
func ParseFile(path string, year int) ([]model.SIPMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("sip: opening %s: %w", path, err)
	}
	defer f.Close()
	if year == 0 {
		year = time.Now().Year()
	}
	return Parse(f, year)
}

func parseTimestamp(s string, year int) (time.Time, error) {
	t, err := time.Parse(timestampLayout, s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(year, t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local), nil
}

// splitTransportAddr splits "UDP:192.168.1.1:5060" into ("UDP", "192.168.1.1:5060").
func splitTransportAddr(s string) (transport, addr string) {
	idx := strings.IndexByte(s, ':')
	if idx < 0 {
		return s, ""
	}
	return s[:idx], s[idx+1:]
}

// parseSIPContent parses the SIP start line and headers into msg.
func parseSIPContent(headers, body []string, msg *model.SIPMessage) error {
	if len(headers) == 0 {
		return fmt.Errorf("empty SIP message")
	}
	startLine := headers[0]
	if strings.HasPrefix(startLine, "SIP/2.0 ") {
		rest := startLine[8:]
		idx := strings.IndexByte(rest, ' ')
		if idx < 0 {
			// Response with no reason phrase (unusual but valid).
			code, err := strconv.Atoi(strings.TrimSpace(rest))
			if err != nil {
				return fmt.Errorf("malformed response: %q", startLine)
			}
			msg.StatusCode = code
		} else {
			code, err := strconv.Atoi(rest[:idx])
			if err != nil {
				return fmt.Errorf("malformed response code: %q", startLine)
			}
			msg.StatusCode = code
			msg.StatusText = rest[idx+1:]
		}
	} else {
		fields := strings.Fields(startLine)
		if len(fields) == 0 {
			return fmt.Errorf("empty start line")
		}
		msg.Method = fields[0]
	}

	for _, h := range headers[1:] {
		idx := strings.IndexByte(h, ':')
		if idx < 0 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(h[:idx]))
		value := strings.TrimSpace(h[idx+1:])
		switch name {
		case "call-id", "i":
			msg.CallID = value
		case "from", "f":
			msg.From = value
		case "to", "t":
			msg.To = value
		case "cseq":
			msg.CSeq = value
		case "content-type", "c":
			msg.ContentType = value
		}
	}

	if len(body) > 0 {
		msg.Body = strings.Join(body, "\n")
	}
	return nil
}
