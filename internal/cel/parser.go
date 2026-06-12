// Package cel parses Asterisk CEL (Channel Event Logging) CSV files.
//
// The CSV format is defined by cel_custom.conf in /etc/asterisk. The default
// column layout asterism expects is the 13-column reference mapping:
//
//	eventtype, eventtime, calleridnum, calleridname, channel, exten, context,
//	uniqueid, linkedid, bridgepeer, appname, appdata, eventextra
//
// Users whose cel_custom.conf differs can pass a custom column list via
// ParseFileWithColumns / ParseFileLenientWithColumns. See DefaultColumns.
//
// CEL CSVs have no header row. The parser is strict about column count by
// default: a row with the wrong number of fields is an error, not a warning.
// This is deliberate — silent data corruption is worse than a loud failure.
package cel

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/forgetdev/asterism/internal/model"
)

// DefaultColumns is the canonical 13-column layout asterism expects from
// cel_custom.conf. Each name is the Asterisk CEL variable name without ${}.
var DefaultColumns = []string{
	"eventtype", "eventtime", "calleridnum", "calleridname",
	"channel", "exten", "context", "uniqueid", "linkedid",
	"bridgepeer", "appname", "appdata", "eventextra",
}

// knownColumns lists every column name asterism understands.
var knownColumns = map[string]bool{
	"eventtype": true, "eventtime": true, "calleridnum": true,
	"calleridname": true, "channel": true, "exten": true,
	"context": true, "uniqueid": true, "linkedid": true,
	"bridgepeer": true, "appname": true, "appdata": true, "eventextra": true,
}

// requiredColumns must be present in every column mapping.
var requiredColumns = []string{"eventtype", "eventtime", "channel", "uniqueid", "linkedid"}

// colIndices maps each known column to its 0-based position in a CSV row.
// A value of -1 means the column is absent; getField returns "" for those.
type colIndices struct {
	eventType  int
	eventTime  int
	cidNum     int
	cidName    int
	channel    int
	exten      int
	context    int
	uniqueID   int
	linkedID   int
	bridgePeer int
	appName    int
	appData    int
	eventExtra int
}

// defaultColIndices is the pre-built index for DefaultColumns.
var defaultColIndices = mustBuildColIndices(DefaultColumns)

func mustBuildColIndices(cols []string) colIndices {
	ci, err := buildColIndices(cols)
	if err != nil {
		panic("cel: invalid default columns: " + err.Error())
	}
	return ci
}

// BuildColIndices validates cols and returns the corresponding colIndices.
// Exported so main.go can validate --cel-columns at startup.
func BuildColIndices(cols []string) (colIndices, error) {
	return buildColIndices(cols)
}

func buildColIndices(cols []string) (colIndices, error) {
	for _, c := range cols {
		if !knownColumns[c] {
			known := sortedKeys(knownColumns)
			return colIndices{}, fmt.Errorf("unknown column %q — valid names: %s",
				c, strings.Join(known, ", "))
		}
	}
	m := make(map[string]int, len(cols))
	for i, c := range cols {
		if _, dup := m[c]; dup {
			return colIndices{}, fmt.Errorf("duplicate column %q at position %d", c, i)
		}
		m[c] = i
	}
	for _, req := range requiredColumns {
		if _, ok := m[req]; !ok {
			return colIndices{}, fmt.Errorf("required column %q is missing", req)
		}
	}
	idx := func(name string) int {
		if i, ok := m[name]; ok {
			return i
		}
		return -1
	}
	return colIndices{
		eventType:  idx("eventtype"),
		eventTime:  idx("eventtime"),
		cidNum:     idx("calleridnum"),
		cidName:    idx("calleridname"),
		channel:    idx("channel"),
		exten:      idx("exten"),
		context:    idx("context"),
		uniqueID:   idx("uniqueid"),
		linkedID:   idx("linkedid"),
		bridgePeer: idx("bridgepeer"),
		appName:    idx("appname"),
		appData:    idx("appdata"),
		eventExtra: idx("eventextra"),
	}, nil
}

func getField(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return row[idx]
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Parse reads CEL events from r using DefaultColumns.
// Malformed rows return an error with line number and field count for diagnosis.
func Parse(r io.Reader) ([]model.Event, error) {
	return parseWith(r, defaultColIndices, len(DefaultColumns))
}

// ParseWithColumns reads CEL events using a custom column list.
// cols must contain at least the required columns (eventtype, eventtime,
// channel, uniqueid, linkedid). Unknown names cause an error at call time.
func ParseWithColumns(r io.Reader, cols []string) ([]model.Event, error) {
	ci, err := buildColIndices(cols)
	if err != nil {
		return nil, fmt.Errorf("cel: invalid columns: %w", err)
	}
	return parseWith(r, ci, len(cols))
}

func parseWith(r io.Reader, ci colIndices, wantCols int) ([]model.Event, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // checked manually for better error messages

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
		if len(row) != wantCols {
			return nil, fmt.Errorf("cel: line %d: got %d fields, want %d",
				lineNum, len(row), wantCols)
		}
		ev, err := rowToEvent(row, ci)
		if err != nil {
			return nil, fmt.Errorf("cel: line %d: %w", lineNum, err)
		}
		events = append(events, ev)
	}
	return events, nil
}

// ParseFile opens path and parses it with DefaultColumns.
func ParseFile(path string) ([]model.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cel: opening %s: %w", path, err)
	}
	defer f.Close()
	return Parse(f)
}

// ParseFileWithColumns opens path and parses it with a custom column list.
func ParseFileWithColumns(path string, cols []string) ([]model.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cel: opening %s: %w", path, err)
	}
	defer f.Close()
	return ParseWithColumns(f, cols)
}

// ParseLenient reads CEL events from r using DefaultColumns, skipping any row
// that cannot be parsed instead of returning an error.
// Returns events, skipped count, and any fatal I/O error.
func ParseLenient(r io.Reader) ([]model.Event, int, error) {
	return parseLenientWith(r, defaultColIndices, len(DefaultColumns))
}

// ParseLenientWithColumns is ParseLenient with a custom column list.
func ParseLenientWithColumns(r io.Reader, cols []string) ([]model.Event, int, error) {
	ci, err := buildColIndices(cols)
	if err != nil {
		return nil, 0, fmt.Errorf("cel: invalid columns: %w", err)
	}
	return parseLenientWith(r, ci, len(cols))
}

func parseLenientWith(r io.Reader, ci colIndices, wantCols int) ([]model.Event, int, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

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
		if len(row) != wantCols {
			skipped++
			continue
		}
		ev, err := rowToEvent(row, ci)
		if err != nil {
			skipped++
			continue
		}
		events = append(events, ev)
	}
	return events, skipped, nil
}

// ParseFileLenient opens path and parses it leniently with DefaultColumns.
func ParseFileLenient(path string) ([]model.Event, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("cel: opening %s: %w", path, err)
	}
	defer f.Close()
	return ParseLenient(f)
}

// ParseFileLenientWithColumns opens path and parses it leniently with a custom column list.
func ParseFileLenientWithColumns(path string, cols []string) ([]model.Event, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("cel: opening %s: %w", path, err)
	}
	defer f.Close()
	return ParseLenientWithColumns(f, cols)
}

// rowToEvent converts a CSV row into a typed Event using the given column indices.
func rowToEvent(row []string, ci colIndices) (model.Event, error) {
	ts, err := parseEventTime(getField(row, ci.eventTime))
	if err != nil {
		return model.Event{}, fmt.Errorf("parsing timestamp %q: %w", getField(row, ci.eventTime), err)
	}
	return model.Event{
		Type:        model.EventType(getField(row, ci.eventType)),
		Timestamp:   ts,
		CIDNum:      getField(row, ci.cidNum),
		CIDName:     getField(row, ci.cidName),
		ChannelName: getField(row, ci.channel),
		Exten:       getField(row, ci.exten),
		Context:     getField(row, ci.context),
		UniqueID:    getField(row, ci.uniqueID),
		LinkedID:    getField(row, ci.linkedID),
		BridgePeer:  getField(row, ci.bridgePeer),
		AppName:     getField(row, ci.appName),
		AppData:     getField(row, ci.appData),
		Extra:       getField(row, ci.eventExtra),
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
