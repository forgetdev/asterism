// Package fulllog parses the Asterisk full log (/var/log/asterisk/full).
//
// The full log adds dialplan execution detail and channel lifecycle events that
// CEL omits. asterism uses it to enrich a call's timeline with the "why" behind
// CEL events — e.g. which dialplan extension was executing, what app_dial saw.
//
// Correlation with CEL is by channel name: the [C-XXXXXXXX] call ID in the log
// is an internal Asterisk counter that is NOT the LinkedID, so channel name is
// the only reliable join key between the two sources.
//
// Line format:
//
//	[Mon DD HH:MM:SS] LEVEL[thread][C-callid] source.c: message
//	[Mon DD HH:MM:SS] LEVEL[thread] source.c: message  (no call context)
package fulllog

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/forgetdev/asterism/internal/model"
)

// logLineRE matches an Asterisk full log line.
// Groups: (timestamp) (level) (thread) (callid, optional) (source) (message)
var logLineRE = regexp.MustCompile(
	`^\[([A-Za-z]+ +\d+ \d{2}:\d{2}:\d{2})\] ` +
		`(\w+)\[(\d+)\]` +
		`(?:\[(C-[0-9A-Fa-f]+)\])?` +
		` (\S+): (.*)$`,
)

// timestampLayout matches Asterisk's log timestamp, e.g. "Jun 11 23:51:24".
// No year is present; we use the current year at parse time.
const timestampLayout = "Jan _2 15:04:05"

// Parse reads an Asterisk full log from r and returns parsed lines in file order.
// Lines that don't match the expected format are silently skipped — the full log
// contains continuation lines, blank lines, and other noise that is not useful.
func Parse(r io.Reader, year int) ([]model.LogLine, error) {
	var lines []model.LogLine
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		raw := scanner.Text()
		m := logLineRE.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		ts, err := parseTimestamp(m[1], year)
		if err != nil {
			continue
		}
		lines = append(lines, model.LogLine{
			Timestamp: ts,
			Level:     m[2],
			CallID:    m[4],
			Source:    m[5],
			Message:   m[6],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("fulllog: scanning: %w", err)
	}
	return lines, nil
}

// ParseFile is a convenience wrapper that opens the path and parses it.
// The year is needed because Asterisk log timestamps omit the year.
// Pass 0 to use the current year.
func ParseFile(path string, year int) ([]model.LogLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("fulllog: opening %s: %w", path, err)
	}
	defer f.Close()
	if year == 0 {
		year = time.Now().Year()
	}
	return Parse(f, year)
}

// parseTimestamp parses a log timestamp like "Jun 11 23:51:24" into a time.Time.
// Asterisk logs local time without a timezone suffix. We use time.Local so that
// when asterism runs on the same machine as Asterisk, log line offsets align
// correctly with CEL Unix timestamps. Cross-machine analysis may need a manual
// offset; that is a future concern.
func parseTimestamp(s string, year int) (time.Time, error) {
	t, err := time.Parse(timestampLayout, s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(year, t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local), nil
}
