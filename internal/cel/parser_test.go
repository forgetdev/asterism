package cel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forgetdev/asterism/internal/model"
)

func TestParseFixture02(t *testing.T) {
	// Resolve the testdata path relative to the repo root.
	// Running `go test ./internal/cel` from the repo root sets cwd to internal/cel,
	// so we go up two levels.
	path := filepath.Join("..", "..", "testdata", "fixture-02-ramal-answered", "cel.csv")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("fixture not present at %s", path)
	}

	events, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if got, want := len(events), 15; got != want {
		t.Errorf("event count: got %d, want %d", got, want)
	}

	// First event must be CHAN_START of PJSIP/1002.
	first := events[0]
	if first.Type != model.EventChanStart {
		t.Errorf("first event type: got %q, want %q", first.Type, model.EventChanStart)
	}
	if first.ChannelName != "PJSIP/1002-00000002" {
		t.Errorf("first event channel: got %q, want %q", first.ChannelName, "PJSIP/1002-00000002")
	}

	// Last event must be LINKEDID_END.
	last := events[len(events)-1]
	if last.Type != model.EventLinkedIDEnd {
		t.Errorf("last event type: got %q, want %q", last.Type, model.EventLinkedIDEnd)
	}

	// All events must share the same LinkedID.
	expectedLinkedID := "1779999013.2"
	for i, ev := range events {
		if ev.LinkedID != expectedLinkedID {
			t.Errorf("event %d: linkedid = %q, want %q", i, ev.LinkedID, expectedLinkedID)
		}
	}
}

// Custom columns

func TestBuildColIndices_UnknownColumn(t *testing.T) {
	_, err := buildColIndices([]string{"eventtype", "eventtime", "channel", "uniqueid", "linkedid", "nosuchcol"})
	if err == nil {
		t.Fatal("want error for unknown column, got nil")
	}
	if !strings.Contains(err.Error(), "nosuchcol") {
		t.Errorf("error should name the bad column, got: %v", err)
	}
}

func TestBuildColIndices_MissingRequired(t *testing.T) {
	// missing "linkedid"
	_, err := buildColIndices([]string{"eventtype", "eventtime", "channel", "uniqueid"})
	if err == nil {
		t.Fatal("want error for missing required column, got nil")
	}
	if !strings.Contains(err.Error(), "linkedid") {
		t.Errorf("error should name the missing column, got: %v", err)
	}
}

func TestBuildColIndices_DuplicateColumn(t *testing.T) {
	_, err := buildColIndices([]string{"eventtype", "eventtime", "channel", "uniqueid", "linkedid", "eventtype"})
	if err == nil {
		t.Fatal("want error for duplicate column, got nil")
	}
}

func TestParseWithColumns_CustomOrder(t *testing.T) {
	// Reordered: linkedid first, then the rest of the required fields
	cols := []string{"linkedid", "uniqueid", "eventtype", "eventtime", "channel"}
	// Build one minimal row: linkedid=L1, uniqueid=U1, eventtype=CHAN_START, eventtime=epoch, channel=PJSIP/1001
	row := "L1,U1,CHAN_START,1700000000,PJSIP/1001\n"
	events, err := ParseWithColumns(strings.NewReader(row), cols)
	if err != nil {
		t.Fatalf("ParseWithColumns error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.LinkedID != "L1" {
		t.Errorf("LinkedID: got %q, want L1", ev.LinkedID)
	}
	if ev.UniqueID != "U1" {
		t.Errorf("UniqueID: got %q, want U1", ev.UniqueID)
	}
	if ev.Type != model.EventChanStart {
		t.Errorf("Type: got %q, want CHAN_START", ev.Type)
	}
	if ev.ChannelName != "PJSIP/1001" {
		t.Errorf("ChannelName: got %q, want PJSIP/1001", ev.ChannelName)
	}
}

func TestParse_WrongColumnCount_ErrorMessage(t *testing.T) {
	// 12-column row (one short of default 13)
	row := "CHAN_START,1700000000,1001,Alice,PJSIP/1001-001,1001,internal,U1,L1,,,\n"
	_, err := Parse(strings.NewReader(row))
	if err == nil {
		t.Fatal("want error for wrong column count, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "got 12") || !strings.Contains(msg, "want 13") {
		t.Errorf("error should report field counts, got: %q", msg)
	}
}

func TestParseEventTime(t *testing.T) {
	tests := []struct {
		input     string
		wantSec   int64
		wantNanos int64
	}{
		{"1779999013.320140", 1779999013, 320140000},
		{"1779999013.2", 1779999013, 200000000},
		{"1779999013", 1779999013, 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseEventTime(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Unix() != tt.wantSec {
				t.Errorf("seconds: got %d, want %d", got.Unix(), tt.wantSec)
			}
			if int64(got.Nanosecond()) != tt.wantNanos {
				t.Errorf("nanos: got %d, want %d", got.Nanosecond(), tt.wantNanos)
			}
		})
	}
}
