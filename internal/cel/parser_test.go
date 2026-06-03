package cel

import (
	"os"
	"path/filepath"
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
