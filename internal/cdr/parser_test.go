package cdr

import (
	"strings"
	"testing"
	"time"
)

// A valid 18-column CDR row (all fields present).
// CLID uses simple format (no embedded quotes) to keep the CSV unambiguous.
const validRow = `,"1001","1002","internal","1001 <1001>","PJSIP/1001-00000001","PJSIP/1002-00000002","Dial","PJSIP/1002",` +
	`"2026-06-12 16:39:45","2026-06-12 16:39:50","2026-06-12 16:40:25","40","35","ANSWERED","DOCUMENTATION","1700000000.1",""` + "\n"

func TestParse_ValidRow(t *testing.T) {
	records, err := Parse(strings.NewReader(validRow))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("want 1 record, got %d", len(records))
	}
	r := records[0]
	if r.Src != "1001" {
		t.Errorf("Src: got %q, want 1001", r.Src)
	}
	if r.Dst != "1002" {
		t.Errorf("Dst: got %q, want 1002", r.Dst)
	}
	if r.UniqueID != "1700000000.1" {
		t.Errorf("UniqueID: got %q, want 1700000000.1", r.UniqueID)
	}
	if r.Duration != 40*time.Second {
		t.Errorf("Duration: got %v, want 40s", r.Duration)
	}
	if r.BillSec != 35*time.Second {
		t.Errorf("BillSec: got %v, want 35s", r.BillSec)
	}
	if r.Disposition != "ANSWERED" {
		t.Errorf("Disposition: got %q, want ANSWERED", r.Disposition)
	}
}

func TestParse_WrongColumnCount_Error(t *testing.T) {
	// 17 columns — missing userfield
	short := `,"1001","1002","internal","\"1001\" <1001>","PJSIP/1001-00000001","PJSIP/1002-00000002","Dial","PJSIP/1002",` +
		`"2026-06-12 16:39:45","2026-06-12 16:39:50","2026-06-12 16:40:25","40","35","ANSWERED","DOCUMENTATION","1700000000.1"` + "\n"
	_, err := Parse(strings.NewReader(short))
	if err == nil {
		t.Fatal("want error for wrong column count, got nil")
	}
}

func TestParse_ErrorContainsLineNumber(t *testing.T) {
	// Two valid rows then one bad row.
	input := validRow + validRow + `"bad","row"` + "\n"
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "line 3") {
		t.Errorf("error should mention line 3, got: %q", msg)
	}
}

func TestParseLenient_SkipsBadRows(t *testing.T) {
	bad := `"too","few","columns"` + "\n"
	input := validRow + bad + validRow
	records, skipped, err := ParseLenient(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseLenient error: %v", err)
	}
	if skipped != 1 {
		t.Errorf("skipped: got %d, want 1", skipped)
	}
	if len(records) != 2 {
		t.Errorf("records: got %d, want 2", len(records))
	}
}

func TestParseLenient_AllBad_ReturnsEmpty(t *testing.T) {
	input := `"bad"` + "\n" + `"also","bad"` + "\n"
	records, skipped, err := ParseLenient(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("want 0 records, got %d", len(records))
	}
	if skipped != 2 {
		t.Errorf("want 2 skipped, got %d", skipped)
	}
}

func TestParseCDRTime_Empty(t *testing.T) {
	ts, err := parseCDRTime("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ts.IsZero() {
		t.Errorf("empty string should yield zero time, got %v", ts)
	}
}

func TestParseFixture(t *testing.T) {
	records, err := ParseFile("../../testdata/fixture-02-ramal-answered/cdr.csv")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one CDR record")
	}
}
