package correlate

import (
	"testing"
	"time"

	"github.com/forgetdev/asterism/internal/model"
)

var epoch = time.Unix(1_700_000_000, 0)

// ByLinkedID

func TestByLinkedID_GroupsByLinkedID(t *testing.T) {
	events := []model.Event{
		{Type: model.EventChanStart, Timestamp: epoch, UniqueID: "a.1", LinkedID: "a.1"},
		{Type: model.EventChanStart, Timestamp: epoch, UniqueID: "b.1", LinkedID: "b.1"},
		{Type: model.EventHangup, Timestamp: epoch.Add(time.Second), UniqueID: "a.1", LinkedID: "a.1"},
	}
	calls := ByLinkedID(events)
	if len(calls) != 2 {
		t.Fatalf("want 2 calls, got %d", len(calls))
	}
}

func TestByLinkedID_EventsSortedByTimestamp(t *testing.T) {
	t0 := epoch
	t1 := epoch.Add(time.Second)
	t2 := epoch.Add(2 * time.Second)
	events := []model.Event{
		{Type: model.EventHangup, Timestamp: t2, UniqueID: "x.1", LinkedID: "x.1"},
		{Type: model.EventAnswer, Timestamp: t1, UniqueID: "x.1", LinkedID: "x.1"},
		{Type: model.EventChanStart, Timestamp: t0, UniqueID: "x.1", LinkedID: "x.1"},
	}
	calls := ByLinkedID(events)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	evs := calls[0].Events
	if evs[0].Type != model.EventChanStart || evs[1].Type != model.EventAnswer || evs[2].Type != model.EventHangup {
		t.Errorf("events not sorted: %v", []model.EventType{evs[0].Type, evs[1].Type, evs[2].Type})
	}
}

func TestByLinkedID_CallsSortedByStartTime(t *testing.T) {
	t0 := epoch
	t1 := epoch.Add(time.Hour)
	events := []model.Event{
		{Type: model.EventChanStart, Timestamp: t1, UniqueID: "late.1", LinkedID: "late.1"},
		{Type: model.EventChanStart, Timestamp: t0, UniqueID: "early.1", LinkedID: "early.1"},
	}
	calls := ByLinkedID(events)
	if calls[0].LinkedID != "early.1" || calls[1].LinkedID != "late.1" {
		t.Errorf("calls not sorted by start time: %v %v", calls[0].LinkedID, calls[1].LinkedID)
	}
}

func TestByLinkedID_DropsEmptyLinkedID(t *testing.T) {
	events := []model.Event{
		{Type: model.EventChanStart, Timestamp: epoch, UniqueID: "a.1", LinkedID: ""},
		{Type: model.EventChanStart, Timestamp: epoch, UniqueID: "b.1", LinkedID: "b.1"},
	}
	calls := ByLinkedID(events)
	if len(calls) != 1 || calls[0].LinkedID != "b.1" {
		t.Errorf("want 1 call with id b.1, got %v", calls)
	}
}

// AttachCDR

func TestAttachCDR_AttachesToMatchingCall(t *testing.T) {
	calls := ByLinkedID([]model.Event{
		{Type: model.EventChanStart, Timestamp: epoch, UniqueID: "a.1", LinkedID: "a.1"},
		{Type: model.EventChanStart, Timestamp: epoch, UniqueID: "b.1", LinkedID: "b.1"},
	})
	cdrs := []model.CDR{
		{UniqueID: "a.1", Disposition: "ANSWERED", BillSec: 10 * time.Second},
	}
	calls = AttachCDR(calls, cdrs)

	var callA, callB *model.Call
	for i := range calls {
		if calls[i].LinkedID == "a.1" {
			callA = &calls[i]
		} else {
			callB = &calls[i]
		}
	}
	if callA == nil || len(callA.CDRs) != 1 {
		t.Errorf("call a.1 should have 1 CDR, got %d", len(callA.CDRs))
	}
	if callB == nil || len(callB.CDRs) != 0 {
		t.Errorf("call b.1 should have 0 CDRs, got %d", len(callB.CDRs))
	}
}

func TestAttachCDR_MultiLeg(t *testing.T) {
	// A transfer call: originating channel + dialed channel, same linkedid
	calls := ByLinkedID([]model.Event{
		{Type: model.EventChanStart, Timestamp: epoch, UniqueID: "a.1", LinkedID: "a.1"},
		{Type: model.EventChanStart, Timestamp: epoch, UniqueID: "a.2", LinkedID: "a.1"},
	})
	cdrs := []model.CDR{
		{UniqueID: "a.1", BillSec: 30 * time.Second},
		{UniqueID: "a.2", BillSec: 10 * time.Second},
	}
	calls = AttachCDR(calls, cdrs)
	if len(calls) != 1 || len(calls[0].CDRs) != 2 {
		t.Errorf("want 1 call with 2 CDRs, got %d calls, %d CDRs",
			len(calls), len(calls[0].CDRs))
	}
}

func TestAttachCDR_UnknownUniqueID_Dropped(t *testing.T) {
	calls := ByLinkedID([]model.Event{
		{Type: model.EventChanStart, Timestamp: epoch, UniqueID: "a.1", LinkedID: "a.1"},
	})
	cdrs := []model.CDR{
		{UniqueID: "unknown.99"},
	}
	calls = AttachCDR(calls, cdrs)
	if len(calls[0].CDRs) != 0 {
		t.Errorf("want 0 CDRs (unknown uniqueid dropped), got %d", len(calls[0].CDRs))
	}
}
