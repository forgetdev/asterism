// Package cel parses Asterisk CEL (Channel Event Logging) CSV files.
//
// The CSV format is defined by cel_custom.conf in /etc/asterisk. asterism
// assumes the column layout from the asterism lab's cel_custom.conf:
//
//	eventtype, eventtime, cid_num, cid_name, channame, exten, context,
//	uniqueid, linkedid, bridgepeer, appname, appdata, eventextra
//
// CEL CSVs have no header row. The parser is strict about column count:
// a row with the wrong number of fields is an error, not a warning. This
// is deliberate — silent data corruption is worse than a loud failure.
package cel

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/forgetdev/asterism/internal/model"
)

// expectedColumns is the number of fields each CEL row must have, based on
// the cel_custom.conf mapping asterism expects. Changing the mapping requires
// updating this constant and Parse.
const expectedColumns = 13

// Parse reads CEL events from r and returns them in file order.
// Malformed rows return an error with line number for diagnosis.
func Parse(r io.Reader) ([]model.Event, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = expectedColumns
	// Asterisk does not write a header, so no header skip.

	var events []model.Event
	lineNum := 0
	for {
		lineNum++
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("cel: line %d: %w", lineNum, err)
		}

		ev, err := rowToEvent(row)
		if err != nil {
			return nil, fmt.Errorf("cel: line %d: %w", lineNum, err)
		}
		events = append(events, ev)
	}
	return events, nil
}

// ParseFile is a convenience wrapper that opens the path and parses it.
func ParseFile(path string) ([]model.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cel: opening %s: %w", path, err)
	}
	defer f.Close()
	return Parse(f)
}

// ParseLenient reads CEL events from r, skipping any row that cannot be parsed
// (wrong column count, bad timestamp, etc.) instead of returning an error.
// Returns the parsed events, the number of rows skipped, and any fatal I/O error.
// Use this for production data where a handful of malformed rows are expected.
func ParseLenient(r io.Reader) ([]model.Event, int, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // accept any column count; we check manually
	reader.LazyQuotes = true    // tolerate bare quotes inside fields

	var events []model.Event
	skipped := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			skipped++
			continue
		}
		if len(row) != expectedColumns {
			skipped++
			continue
		}
		ev, err := rowToEvent(row)
		if err != nil {
			skipped++
			continue
		}
		events = append(events, ev)
	}
	return events, skipped, nil
}

// ParseFileLenient is a convenience wrapper for ParseLenient.
func ParseFileLenient(path string) ([]model.Event, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("cel: opening %s: %w", path, err)
	}
	defer f.Close()
	return ParseLenient(f)
}

// rowToEvent converts a 13-field CSV row into a typed Event.
// The timestamp parsing is the most fragile part: Asterisk emits eventtime
// as a Unix epoch with microseconds (e.g., "1779999013.320140"). We parse
// it as a float and convert to time.Time, preserving microsecond precision.
func rowToEvent(row []string) (model.Event, error) {
	ts, err := parseEventTime(row[1])
	if err != nil {
		return model.Event{}, fmt.Errorf("parsing timestamp %q: %w", row[1], err)
	}

	return model.Event{
		Type:        model.EventType(row[0]),
		Timestamp:   ts,
		CIDNum:      row[2],
		CIDName:     row[3],
		ChannelName: row[4],
		Exten:       row[5],
		Context:     row[6],
		UniqueID:    row[7],
		LinkedID:    row[8],
		BridgePeer:  row[9],
		AppName:     row[10],
		AppData:     row[11],
		Extra:       row[12],
	}, nil
}

// parseEventTime parses an Asterisk CEL eventtime field.
// Format: Unix epoch with fractional seconds, e.g., "1779999013.320140".
// We split on "." and assemble manually to preserve microsecond precision —
// strconv.ParseFloat would round.
func parseEventTime(s string) (time.Time, error) {
	parts := strings.SplitN(s, ".", 2)
	secs, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	var nsec int64
	if len(parts) == 2 {
		// Pad or truncate the fractional part to 9 digits (nanoseconds).
		frac := parts[1]
		if len(frac) > 9 {
			frac = frac[:9]
		} else {
			frac = frac + strings.Repeat("0", 9-len(frac))
		}
		nsec, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return time.Time{}, err
		}
	}
	return time.Unix(secs, nsec), nil
}
