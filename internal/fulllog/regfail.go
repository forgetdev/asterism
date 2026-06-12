package fulllog

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/forgetdev/asterism/internal/model"
)

// regFailRE matches the message part of a NOTICE line from pjsip_distributor.c:
// Request 'REGISTER' from '"user" <sip:1001@x.x.x.x>' failed for 'ip:port' (callid: ...) - reason
var regFailRE = regexp.MustCompile(
	`Request 'REGISTER' from '(.+?)' failed for '([^']+)' \(callid: ([^)]+)\) - (.+)`)

// sipUserRE extracts the username from a SIP URI like <sip:user@host>.
var sipUserRE = regexp.MustCompile(`sip:([^@>]+)@`)

// ParseRegistrationFailures scans path for failed SIP REGISTER attempts
// logged by Asterisk's pjsip_distributor module and returns them in log order.
// Pass 0 as year to use the current year.
func ParseRegistrationFailures(path string) ([]model.RegistrationFailure, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("fulllog: opening %s: %w", path, err)
	}
	defer f.Close()
	return parseRegistrationFailures(f, time.Now().Year())
}

func parseRegistrationFailures(r io.Reader, year int) ([]model.RegistrationFailure, error) {
	var failures []model.RegistrationFailure
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		m := logLineRE.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		// m[2]=level, m[5]=source, m[6]=message
		if m[2] != "NOTICE" || !strings.Contains(m[5], "pjsip_distributor") {
			continue
		}
		rm := regFailRE.FindStringSubmatch(m[6])
		if rm == nil {
			continue
		}
		ts, err := parseTimestamp(m[1], year)
		if err != nil {
			continue
		}
		fromStr := rm[1]
		endpoint := ""
		if um := sipUserRE.FindStringSubmatch(fromStr); um != nil {
			endpoint = um[1]
		}
		failures = append(failures, model.RegistrationFailure{
			Timestamp: ts,
			Endpoint:  endpoint,
			From:      fromStr,
			SourceIP:  rm[2],
			CallID:    strings.TrimSpace(rm[3]),
			Reason:    strings.TrimSpace(rm[4]),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("fulllog: scanning: %w", err)
	}
	return failures, nil
}
