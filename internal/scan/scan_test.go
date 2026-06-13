package scan

import (
	"testing"
	"time"

	"github.com/forgetdev/asterism/internal/model"
)

// epoch is an arbitrary anchor time used across all test events.
var epoch = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

// intPtr returns a pointer to n, for constructing Options.ManyTransfers.
func intPtr(n int) *int { return &n }

func ts(secondsOffset float64) time.Time {
	return epoch.Add(time.Duration(secondsOffset * float64(time.Second)))
}

// bridgeEnter / bridgeExit build minimal CEL events with bridge_id in Extra.
func bridgeEnter(t time.Time, bridgeID string) model.Event {
	return model.Event{
		Type:      model.EventBridgeEnter,
		Timestamp: t,
		Extra:     `{"bridge_id":"` + bridgeID + `","bridge_technology":"simple_bridge"}`,
	}
}

func bridgeExit(t time.Time, bridgeID string) model.Event {
	return model.Event{
		Type:      model.EventBridgeExit,
		Timestamp: t,
		Extra:     `{"bridge_id":"` + bridgeID + `","bridge_technology":"native_rtp"}`,
	}
}

func transferEvent(typ model.EventType) model.Event {
	return model.Event{Type: typ, Timestamp: epoch}
}

func hangupEvent(linkedID string) model.Event {
	return model.Event{
		Type:      model.EventHangup,
		Timestamp: epoch,
		UniqueID:  linkedID,
		LinkedID:  linkedID,
		Extra:     `{"hangupcause":16,"hangupsource":"","dialstatus":"ANSWER"}`,
	}
}

// makeCall builds a model.Call with the given events and linkedID "test.1".
func makeCall(events ...model.Event) model.Call {
	for i := range events {
		if events[i].LinkedID == "" {
			events[i].LinkedID = "test.1"
		}
	}
	return model.Call{LinkedID: "test.1", Events: events}
}

// --- long-hold tests ---

func TestCheckLongHold_Exceeds(t *testing.T) {
	call := makeCall(
		bridgeEnter(ts(0), "bridge-a"),
		bridgeExit(ts(70), "bridge-a"), // 70 s bridge
	)
	matches, _ := Run([]model.Call{call}, Options{LongHold: 60 * time.Second})
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].LinkedID != "test.1" {
		t.Errorf("wrong linkedID: %q", matches[0].LinkedID)
	}
	t.Logf("reason: %s", matches[0].Reason)
}

func TestCheckLongHold_BelowThreshold(t *testing.T) {
	call := makeCall(
		bridgeEnter(ts(0), "bridge-a"),
		bridgeExit(ts(30), "bridge-a"), // 30 s — below 60 s threshold
	)
	matches, _ := Run([]model.Call{call}, Options{LongHold: 60 * time.Second})
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matches))
	}
}

func TestCheckLongHold_MultipleBridgesLongestWins(t *testing.T) {
	// Two bridges: 30 s and 80 s. Only the 80 s one exceeds 60 s.
	call := makeCall(
		bridgeEnter(ts(0), "bridge-a"),
		bridgeExit(ts(30), "bridge-a"),
		bridgeEnter(ts(40), "bridge-b"),
		bridgeExit(ts(120), "bridge-b"),
	)
	matches, _ := Run([]model.Call{call}, Options{LongHold: 60 * time.Second})
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}

func TestCheckLongHold_NoBridgeEvents(t *testing.T) {
	call := makeCall(hangupEvent("test.1"))
	matches, _ := Run([]model.Call{call}, Options{LongHold: 10 * time.Second})
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches for call with no bridge events, got %d", len(matches))
	}
}

// --- many-transfers tests ---

func TestCheckManyTransfers_Exceeds(t *testing.T) {
	call := makeCall(
		transferEvent(model.EventBlindTransfer),
		transferEvent(model.EventAttendedTransfer),
		transferEvent(model.EventBlindTransfer),
	)
	// 3 transfers > threshold 2
	matches, _ := Run([]model.Call{call}, Options{ManyTransfers: intPtr(2)})
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}

func TestCheckManyTransfers_AtThreshold(t *testing.T) {
	// Exactly 2 transfers, threshold is 2 → should NOT match (must be > threshold)
	call := makeCall(
		transferEvent(model.EventBlindTransfer),
		transferEvent(model.EventAttendedTransfer),
	)
	matches, _ := Run([]model.Call{call}, Options{ManyTransfers: intPtr(2)})
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches when transfers == threshold, got %d", len(matches))
	}
}

func TestCheckManyTransfers_NoTransfers(t *testing.T) {
	call := makeCall(hangupEvent("test.1"))
	matches, _ := Run([]model.Call{call}, Options{ManyTransfers: intPtr(1)})
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matches))
	}
}

// --- no-answer rate tests ---

func TestNoAnswerRate_CDRDisposition(t *testing.T) {
	answered := model.Call{
		LinkedID: "a.1",
		Events:   []model.Event{{Type: model.EventChanStart, LinkedID: "a.1", UniqueID: "a.1", Timestamp: epoch}},
		CDRs: []model.CDR{
			{UniqueID: "a.1", Disposition: "ANSWERED"},
		},
	}
	noAnswer := model.Call{
		LinkedID: "b.2",
		Events:   []model.Event{{Type: model.EventChanStart, LinkedID: "b.2", UniqueID: "b.2", Timestamp: epoch}},
		CDRs: []model.CDR{
			{UniqueID: "b.2", Disposition: "NO ANSWER"},
		},
	}

	_, summary := Run([]model.Call{answered, noAnswer}, Options{})
	if summary.Total != 2 {
		t.Errorf("total: want 2, got %d", summary.Total)
	}
	if summary.NoAnswer != 1 {
		t.Errorf("no-answer: want 1, got %d", summary.NoAnswer)
	}
	rate := summary.NoAnswerRate()
	if rate != 0.5 {
		t.Errorf("no-answer rate: want 0.5, got %f", rate)
	}
}

func TestNoAnswerRate_FallbackNoCDR(t *testing.T) {
	// Without CDR, presence of an ANSWER event means the call was answered.
	answeredNoCDR := makeCall(
		model.Event{Type: model.EventAnswer, Timestamp: epoch, LinkedID: "test.1"},
	)
	noAnswerNoCDR := model.Call{
		LinkedID: "c.3",
		Events:   []model.Event{{Type: model.EventChanStart, LinkedID: "c.3", Timestamp: epoch}},
	}

	_, summary := Run([]model.Call{answeredNoCDR, noAnswerNoCDR}, Options{})
	if summary.NoAnswer != 1 {
		t.Errorf("no-answer: want 1, got %d", summary.NoAnswer)
	}
}

// --- combined patterns ---

func TestRun_MultiplePatterns(t *testing.T) {
	// A call that triggers both long-hold and many-transfers.
	call := makeCall(
		bridgeEnter(ts(0), "bridge-a"),
		bridgeExit(ts(90), "bridge-a"),
		transferEvent(model.EventBlindTransfer),
		transferEvent(model.EventAttendedTransfer),
		transferEvent(model.EventBlindTransfer),
	)
	matches, _ := Run([]model.Call{call}, Options{
		LongHold:      60 * time.Second,
		ManyTransfers: intPtr(2),
	})
	if len(matches) != 1 {
		t.Fatalf("expected 1 match (both patterns on same call), got %d", len(matches))
	}
	if matches[0].Reason == "" {
		t.Error("reason should not be empty")
	}
	// Reason should mention both patterns.
	if !containsAll(matches[0].Reason, "long-hold", "many-transfers") {
		t.Errorf("reason %q should mention both patterns", matches[0].Reason)
	}
}

func TestRun_EmptyCalls(t *testing.T) {
	matches, summary := Run(nil, Options{LongHold: time.Minute})
	if len(matches) != 0 {
		t.Errorf("expected no matches for empty input, got %d", len(matches))
	}
	if summary.Total != 0 {
		t.Errorf("expected total=0, got %d", summary.Total)
	}
}

// --- formatDuration helper ---

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Millisecond, "500ms"},
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m30s"},
		{3600 * time.Second, "1h"},
		{3672 * time.Second, "1h1m12s"},
		{7200 * time.Second, "2h"},
	}
	for _, tc := range cases {
		got := formatDuration(tc.d)
		if got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// containsAll reports whether s contains all the given substrings.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
