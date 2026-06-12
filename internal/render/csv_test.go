package render

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/forgetdev/asterism/internal/model"
)

func TestCSV_Header(t *testing.T) {
	var buf bytes.Buffer
	if err := CSV(&buf, nil); err != nil {
		t.Fatalf("CSV error: %v", err)
	}
	rows := parseCSV(t, buf.String())
	if len(rows) != 1 {
		t.Fatalf("want 1 row (header), got %d", len(rows))
	}
	want := []string{"linkedid", "start", "duration_s", "result", "caller", "callee",
		"transfer", "billsec_s", "hangup_cause", "dialstatus", "channels"}
	if len(rows[0]) != len(want) {
		t.Fatalf("header column count: got %d, want %d", len(rows[0]), len(want))
	}
	for i, col := range want {
		if rows[0][i] != col {
			t.Errorf("header[%d]: got %q, want %q", i, rows[0][i], col)
		}
	}
}

func TestCSV_EmptyCalls(t *testing.T) {
	var buf bytes.Buffer
	if err := CSV(&buf, []model.Call{}); err != nil {
		t.Fatalf("CSV error: %v", err)
	}
	rows := parseCSV(t, buf.String())
	if len(rows) != 1 {
		t.Errorf("want only header row for empty calls, got %d rows", len(rows))
	}
}

func TestCSV_SimpleCall(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	call := model.Call{
		LinkedID: "1700000000.1",
		Events: []model.Event{
			{
				Type: model.EventChanStart, Timestamp: start,
				UniqueID: "1700000000.1", LinkedID: "1700000000.1",
				ChannelName: "PJSIP/1001-00000001", CIDNum: "1001",
			},
			{
				Type: model.EventHangup, Timestamp: start.Add(30 * time.Second),
				UniqueID: "1700000000.1", LinkedID: "1700000000.1",
				Extra: `{"hangupcause":16,"dialstatus":"ANSWER"}`,
			},
		},
		CDRs: []model.CDR{
			{UniqueID: "1700000000.1", Src: "1001", Dst: "1002",
				Duration: 30 * time.Second, BillSec: 25 * time.Second, Disposition: "ANSWERED"},
		},
	}

	var buf bytes.Buffer
	if err := CSV(&buf, []model.Call{call}); err != nil {
		t.Fatalf("CSV error: %v", err)
	}
	rows := parseCSV(t, buf.String())
	if len(rows) != 2 {
		t.Fatalf("want 2 rows (header + 1 call), got %d", len(rows))
	}
	row := rows[1]
	if row[0] != "1700000000.1" {
		t.Errorf("linkedid: got %q", row[0])
	}
	if row[2] != "30" {
		t.Errorf("duration_s: got %q, want 30", row[2])
	}
	if row[7] != "25" {
		t.Errorf("billsec_s: got %q, want 25", row[7])
	}
	// row[8]=hangup_cause, row[9]=dialstatus
	if !strings.Contains(row[8], "NORMAL_CLEARING") && !strings.Contains(row[8], "16") {
		t.Errorf("hangup_cause (col 8) should mention 16 or NORMAL_CLEARING, got %q", row[8])
	}
}

func TestCSV_TransferCall_BillsecSum(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	call := model.Call{
		LinkedID: "1700000000.1",
		Events: []model.Event{
			{Type: model.EventChanStart, Timestamp: start,
				UniqueID: "1700000000.1", LinkedID: "1700000000.1"},
			{Type: model.EventBlindTransfer, Timestamp: start.Add(30 * time.Second),
				UniqueID: "1700000000.1", LinkedID: "1700000000.1",
				Extra: `{"extension":"1002","context":"internal","transferee":"PJSIP/caller-00000001"}`},
			{Type: model.EventHangup, Timestamp: start.Add(50 * time.Second),
				UniqueID: "1700000000.1", LinkedID: "1700000000.1"},
		},
		CDRs: []model.CDR{
			{UniqueID: "1700000000.1", BillSec: 30 * time.Second, Disposition: "ANSWERED"},
			{UniqueID: "1700000000.2", BillSec: 15 * time.Second, Disposition: "ANSWERED"},
		},
	}

	var buf bytes.Buffer
	if err := CSV(&buf, []model.Call{call}); err != nil {
		t.Fatalf("CSV error: %v", err)
	}
	rows := parseCSV(t, buf.String())
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	if rows[1][7] != "45" {
		t.Errorf("billsec_s (multi-leg sum): got %q, want 45", rows[1][7])
	}
	if rows[1][6] == "" {
		t.Errorf("transfer field should not be empty for blind transfer call")
	}
}

func TestCSV_ColumnCount(t *testing.T) {
	var buf bytes.Buffer
	call := model.Call{
		LinkedID: "x",
		Events: []model.Event{
			{Type: model.EventChanStart, Timestamp: time.Unix(1_700_000_000, 0),
				UniqueID: "x", LinkedID: "x"},
		},
	}
	if err := CSV(&buf, []model.Call{call}); err != nil {
		t.Fatalf("CSV error: %v", err)
	}
	rows := parseCSV(t, buf.String())
	for i, row := range rows {
		if len(row) != 11 {
			t.Errorf("row %d: got %d columns, want 11", i, len(row))
		}
	}
}

func parseCSV(t *testing.T, s string) [][]string {
	t.Helper()
	r := csv.NewReader(strings.NewReader(s))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parsing CSV output: %v", err)
	}
	return rows
}
