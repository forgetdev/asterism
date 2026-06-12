package filter

import (
	"testing"
	"time"

	"github.com/forgetdev/asterism/internal/model"
)

// helpers

var epoch = time.Unix(1_700_000_000, 0)

func makeCall(linkedID string, start time.Time, dur time.Duration, events []model.Event) model.Call {
	ev := make([]model.Event, 0, len(events)+2)
	ev = append(ev, model.Event{
		Type:      model.EventChanStart,
		Timestamp: start,
		UniqueID:  linkedID,
		LinkedID:  linkedID,
	})
	ev = append(ev, events...)
	ev = append(ev, model.Event{
		Type:      model.EventHangup,
		Timestamp: start.Add(dur),
		UniqueID:  linkedID,
		LinkedID:  linkedID,
	})
	return model.Call{LinkedID: linkedID, Events: ev}
}

func makeCallWithHangupExtra(linkedID string, extra string) model.Call {
	start := epoch
	return model.Call{
		LinkedID: linkedID,
		Events: []model.Event{
			{Type: model.EventChanStart, Timestamp: start, UniqueID: linkedID, LinkedID: linkedID},
			{Type: model.EventHangup, Timestamp: start.Add(10 * time.Second),
				UniqueID: linkedID, LinkedID: linkedID, Extra: extra},
		},
	}
}

// LinkedID

func TestCalls_LinkedID_Match(t *testing.T) {
	calls := []model.Call{
		makeCall("id.1", epoch, 10*time.Second, nil),
		makeCall("id.2", epoch, 10*time.Second, nil),
	}
	got := Calls(calls, Options{LinkedID: "id.1"})
	if len(got) != 1 || got[0].LinkedID != "id.1" {
		t.Errorf("want [id.1], got %v", linkedIDs(got))
	}
}

func TestCalls_LinkedID_NoMatch(t *testing.T) {
	calls := []model.Call{makeCall("id.1", epoch, 10*time.Second, nil)}
	got := Calls(calls, Options{LinkedID: "id.99"})
	if len(got) != 0 {
		t.Errorf("want empty, got %v", linkedIDs(got))
	}
}

// Channel

func TestCalls_Channel_Match(t *testing.T) {
	calls := []model.Call{
		{LinkedID: "a", Events: []model.Event{
			{Type: model.EventChanStart, Timestamp: epoch, ChannelName: "PJSIP/1001-00000001"},
		}},
		{LinkedID: "b", Events: []model.Event{
			{Type: model.EventChanStart, Timestamp: epoch, ChannelName: "PJSIP/1002-00000002"},
		}},
	}
	got := Calls(calls, Options{Channel: "1001"})
	if len(got) != 1 || got[0].LinkedID != "a" {
		t.Errorf("want [a], got %v", linkedIDs(got))
	}
}

// Extension

func TestCalls_Extension_Match(t *testing.T) {
	calls := []model.Call{
		{LinkedID: "a", Events: []model.Event{
			{Type: model.EventChanStart, Timestamp: epoch, Exten: "1001"},
		}},
		{LinkedID: "b", Events: []model.Event{
			{Type: model.EventChanStart, Timestamp: epoch, Exten: "1002"},
		}},
	}
	got := Calls(calls, Options{Extension: "1001"})
	if len(got) != 1 || got[0].LinkedID != "a" {
		t.Errorf("want [a], got %v", linkedIDs(got))
	}
}

// From / To

func TestCalls_From(t *testing.T) {
	t0 := epoch
	t1 := epoch.Add(time.Hour)
	t2 := epoch.Add(2 * time.Hour)
	calls := []model.Call{
		makeCall("early", t0, time.Minute, nil),
		makeCall("late", t2, time.Minute, nil),
	}
	got := Calls(calls, Options{From: t1})
	if len(got) != 1 || got[0].LinkedID != "late" {
		t.Errorf("want [late], got %v", linkedIDs(got))
	}
}

func TestCalls_To(t *testing.T) {
	t0 := epoch
	t2 := epoch.Add(2 * time.Hour)
	calls := []model.Call{
		makeCall("early", t0, time.Minute, nil),
		makeCall("late", t2, time.Minute, nil),
	}
	cutoff := epoch.Add(time.Hour)
	got := Calls(calls, Options{To: cutoff})
	if len(got) != 1 || got[0].LinkedID != "early" {
		t.Errorf("want [early], got %v", linkedIDs(got))
	}
}

func TestCalls_FromTo_BothBounds(t *testing.T) {
	calls := []model.Call{
		makeCall("before", epoch, time.Minute, nil),
		makeCall("within", epoch.Add(90*time.Minute), time.Minute, nil),
		makeCall("after", epoch.Add(3*time.Hour), time.Minute, nil),
	}
	got := Calls(calls, Options{
		From: epoch.Add(time.Hour),
		To:   epoch.Add(2 * time.Hour),
	})
	if len(got) != 1 || got[0].LinkedID != "within" {
		t.Errorf("want [within], got %v", linkedIDs(got))
	}
}

// Duration

func TestCalls_MinDuration(t *testing.T) {
	calls := []model.Call{
		makeCall("short", epoch, 5*time.Second, nil),
		makeCall("long", epoch, 120*time.Second, nil),
	}
	got := Calls(calls, Options{MinDuration: 60 * time.Second})
	if len(got) != 1 || got[0].LinkedID != "long" {
		t.Errorf("want [long], got %v", linkedIDs(got))
	}
}

func TestCalls_MaxDuration(t *testing.T) {
	calls := []model.Call{
		makeCall("short", epoch, 5*time.Second, nil),
		makeCall("long", epoch, 120*time.Second, nil),
	}
	got := Calls(calls, Options{MaxDuration: 10 * time.Second})
	if len(got) != 1 || got[0].LinkedID != "short" {
		t.Errorf("want [short], got %v", linkedIDs(got))
	}
}

// HangupCause

func TestCalls_HangupCause_NumericMatch(t *testing.T) {
	calls := []model.Call{
		makeCallWithHangupExtra("normal", `{"hangupcause":16}`),
		makeCallWithHangupExtra("busy", `{"hangupcause":17}`),
	}
	got := Calls(calls, Options{HangupCause: "16"})
	if len(got) != 1 || got[0].LinkedID != "normal" {
		t.Errorf("want [normal], got %v", linkedIDs(got))
	}
}

func TestCalls_HangupCause_NameMatch(t *testing.T) {
	calls := []model.Call{
		makeCallWithHangupExtra("normal", `{"hangupcause":16}`),
		makeCallWithHangupExtra("busy", `{"hangupcause":17}`),
	}
	got := Calls(calls, Options{HangupCause: "BUSY"})
	if len(got) != 1 || got[0].LinkedID != "busy" {
		t.Errorf("want [busy], got %v", linkedIDs(got))
	}
}

func TestCalls_HangupCause_NoExtra(t *testing.T) {
	calls := []model.Call{makeCall("no-extra", epoch, 10*time.Second, nil)}
	got := Calls(calls, Options{HangupCause: "16"})
	if len(got) != 0 {
		t.Errorf("want empty for call with no hangup extra, got %v", linkedIDs(got))
	}
}

// No filter

func TestCalls_NoFilter_ReturnsAll(t *testing.T) {
	calls := []model.Call{
		makeCall("a", epoch, 10*time.Second, nil),
		makeCall("b", epoch, 20*time.Second, nil),
	}
	got := Calls(calls, Options{})
	if len(got) != 2 {
		t.Errorf("want 2 calls, got %d", len(got))
	}
}

// Events

func TestEvents_FiltersByType(t *testing.T) {
	call := model.Call{
		LinkedID: "x",
		Events: []model.Event{
			{Type: model.EventChanStart, Timestamp: epoch},
			{Type: model.EventHangup, Timestamp: epoch.Add(time.Second)},
			{Type: model.EventAnswer, Timestamp: epoch.Add(2 * time.Second)},
		},
	}
	got := Events([]model.Call{call}, []model.EventType{model.EventHangup})
	if len(got) != 1 {
		t.Fatalf("want 1 call, got %d", len(got))
	}
	if len(got[0].Events) != 1 || got[0].Events[0].Type != model.EventHangup {
		t.Errorf("want only HANGUP event, got %v", got[0].Events)
	}
}

func TestEvents_EmptyTypes_ReturnsAll(t *testing.T) {
	calls := []model.Call{{LinkedID: "x", Events: []model.Event{
		{Type: model.EventChanStart, Timestamp: epoch},
	}}}
	got := Events(calls, nil)
	if len(got[0].Events) != 1 {
		t.Errorf("want events unchanged, got %v", got[0].Events)
	}
}

// ParseEventTypes

func TestParseEventTypes(t *testing.T) {
	tests := []struct {
		input string
		want  []model.EventType
	}{
		{"HANGUP", []model.EventType{"HANGUP"}},
		{"HANGUP,APP_START", []model.EventType{"HANGUP", "APP_START"}},
		{"hangup", []model.EventType{"HANGUP"}}, // uppercased
		{" HANGUP , APP_END ", []model.EventType{"HANGUP", "APP_END"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := ParseEventTypes(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("ParseEventTypes(%q): got %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("ParseEventTypes(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

// ParseTime

func TestParseTime(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"2026-06-12 16:39:45", false},
		{"2026-06-12T16:39:45", false},
		{"2026-06-12", false},
		{"not-a-date", true},
		{"12/06/2026", true},
	}
	for _, tt := range tests {
		_, err := ParseTime(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseTime(%q): err=%v, wantErr=%v", tt.input, err, tt.wantErr)
		}
	}
}

func linkedIDs(calls []model.Call) []string {
	ids := make([]string, len(calls))
	for i, c := range calls {
		ids[i] = c.LinkedID
	}
	return ids
}
