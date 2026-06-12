// Package cdr parses Asterisk CDR (Call Detail Record) CSV files.
//
// The format is the csv backend's Master.csv, whose columns are fixed by the
// cdr_csv module and toggled by cdr.conf. asterism assumes the asterism lab's
// cdr.conf, which enables loguniqueid and loguserfield, yielding 18 columns:
//
//	accountcode, src, dst, dcontext, clid, channel, dstchannel, lastapp,
//	lastdata, start, answer, end, duration, billsec, disposition, amaflags,
//	uniqueid, userfield
//
// loguniqueid is the one that matters: without it there is no uniqueid column
// and a CDR cannot be joined to a correlated Call. The lab enables it, so we
// require it — a row with the wrong column count is an error, not a warning,
// mirroring the CEL parser's strictness. Silent data corruption is worse than
// a loud failure.
package cdr

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/forgetdev/asterism/internal/model"
)

// expectedColumns is the field count each CDR row must have, given the lab's
// cdr.conf (loguniqueid=yes, loguserfield=yes). Changing the config requires
// updating this constant and rowToCDR.
const expectedColumns = 18

// timeLayout matches Asterisk's csv backend timestamp format, e.g.
// "2026-05-28 20:10:13". There is no fractional-second component and no zone
// suffix; with usegmtime=yes these are GMT. We parse into UTC.
const timeLayout = "2006-01-02 15:04:05"

// Parse reads CDR records from r and returns them in file order.
// Malformed rows return an error with line number for diagnosis.
func Parse(r io.Reader) ([]model.CDR, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = expectedColumns
	// The csv backend writes no header row, so no header skip.

	var records []model.CDR
	lineNum := 0
	for {
		lineNum++
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("cdr: line %d: %w", lineNum, err)
		}

		rec, err := rowToCDR(row)
		if err != nil {
			return nil, fmt.Errorf("cdr: line %d: %w", lineNum, err)
		}
		records = append(records, rec)
	}
	return records, nil
}

// ParseFile is a convenience wrapper that opens the path and parses it.
func ParseFile(path string) ([]model.CDR, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cdr: opening %s: %w", path, err)
	}
	defer f.Close()
	return Parse(f)
}

// ParseLenient reads CDR records from r, skipping any row that cannot be parsed
// instead of returning an error. Returns records, skipped count, and any fatal I/O error.
func ParseLenient(r io.Reader) ([]model.CDR, int, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	var records []model.CDR
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
		rec, err := rowToCDR(row)
		if err != nil {
			skipped++
			continue
		}
		records = append(records, rec)
	}
	return records, skipped, nil
}

// ParseFileLenient is a convenience wrapper for ParseLenient.
func ParseFileLenient(path string) ([]model.CDR, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("cdr: opening %s: %w", path, err)
	}
	defer f.Close()
	return ParseLenient(f)
}

// rowToCDR converts an 18-field CSV row into a typed CDR.
func rowToCDR(row []string) (model.CDR, error) {
	start, err := parseCDRTime(row[9])
	if err != nil {
		return model.CDR{}, fmt.Errorf("parsing start %q: %w", row[9], err)
	}
	answer, err := parseCDRTime(row[10])
	if err != nil {
		return model.CDR{}, fmt.Errorf("parsing answer %q: %w", row[10], err)
	}
	end, err := parseCDRTime(row[11])
	if err != nil {
		return model.CDR{}, fmt.Errorf("parsing end %q: %w", row[11], err)
	}
	duration, err := parseSeconds(row[12])
	if err != nil {
		return model.CDR{}, fmt.Errorf("parsing duration %q: %w", row[12], err)
	}
	billsec, err := parseSeconds(row[13])
	if err != nil {
		return model.CDR{}, fmt.Errorf("parsing billsec %q: %w", row[13], err)
	}

	return model.CDR{
		AccountCode: row[0],
		Src:         row[1],
		Dst:         row[2],
		DContext:    row[3],
		CLID:        row[4],
		Channel:     row[5],
		DstChannel:  row[6],
		LastApp:     row[7],
		LastData:    row[8],
		Start:       start,
		Answer:      answer,
		End:         end,
		Duration:    duration,
		BillSec:     billsec,
		Disposition: row[14],
		AMAFlags:    row[15],
		UniqueID:    row[16],
		UserField:   row[17],
	}, nil
}

// parseCDRTime parses a CDR timestamp. An empty string (e.g. the answer time
// of an unanswered call) yields the zero time with no error.
func parseCDRTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(timeLayout, s)
}

// parseSeconds parses a whole-second count into a time.Duration. An empty
// string yields zero — defensive against rows truncated before billing.
func parseSeconds(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return time.Duration(n) * time.Second, nil
}
