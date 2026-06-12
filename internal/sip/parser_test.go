package sip

import (
	"testing"

	"github.com/forgetdev/asterism/internal/model"
)

const fixtureLog = "../../testdata/fixture-04-sip-dialog/full.log"

func TestParseFile(t *testing.T) {
	msgs, err := ParseFile(fixtureLog, 2026)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	// Fixture contains: INVITE(rx), 100(tx), INVITE(tx), 100(rx), 180(rx),
	// 180(tx), 200(rx), 200(tx), ACK(rx), BYE(rx), 200(tx) = 11 messages
	if len(msgs) != 11 {
		t.Fatalf("got %d messages, want 11", len(msgs))
	}

	// First message: received INVITE
	m := msgs[0]
	if m.Method != "INVITE" {
		t.Errorf("msgs[0].Method = %q, want INVITE", m.Method)
	}
	if m.Direction != model.SIPRx {
		t.Errorf("msgs[0].Direction = %v, want SIPRx", m.Direction)
	}
	if m.Transport != "UDP" {
		t.Errorf("msgs[0].Transport = %q, want UDP", m.Transport)
	}
	if m.CallID != "abcd1234-5678-abcd@pbx.local" {
		t.Errorf("msgs[0].CallID = %q, want abcd1234-5678-abcd@pbx.local", m.CallID)
	}
	if m.LogCallID != "C-00000005" {
		t.Errorf("msgs[0].LogCallID = %q, want C-00000005", m.LogCallID)
	}
	if m.Body == "" {
		t.Error("msgs[0].Body is empty, want SDP")
	}

	// Second message: 100 Trying (tx)
	m = msgs[1]
	if m.StatusCode != 100 {
		t.Errorf("msgs[1].StatusCode = %d, want 100", m.StatusCode)
	}
	if m.StatusText != "Trying" {
		t.Errorf("msgs[1].StatusText = %q, want Trying", m.StatusText)
	}
	if m.Direction != model.SIPTx {
		t.Errorf("msgs[1].Direction = %v, want SIPTx", m.Direction)
	}

	// Check 180 Ringing (rx from 1001)
	m = msgs[4]
	if m.StatusCode != 180 {
		t.Errorf("msgs[4].StatusCode = %d, want 180", m.StatusCode)
	}

	// Check BYE (rx)
	var byeMsg *model.SIPMessage
	for i := range msgs {
		if msgs[i].Method == "BYE" {
			byeMsg = &msgs[i]
			break
		}
	}
	if byeMsg == nil {
		t.Fatal("no BYE message found")
	}
	if byeMsg.Direction != model.SIPRx {
		t.Errorf("BYE.Direction = %v, want SIPRx", byeMsg.Direction)
	}
}

func TestCodecs(t *testing.T) {
	msgs, err := ParseFile(fixtureLog, 2026)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	codecs := Codecs(msgs)
	if len(codecs) == 0 {
		t.Fatal("Codecs returned nil")
	}
	want := map[string]bool{"PCMU": true, "PCMA": true, "telephone-event": true}
	for _, c := range codecs {
		if !want[c] {
			t.Errorf("unexpected codec %q", c)
		}
		delete(want, c)
	}
	for c := range want {
		t.Errorf("codec %q not found", c)
	}
}

func TestDiagnoseNoIssues(t *testing.T) {
	msgs, err := ParseFile(fixtureLog, 2026)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	call := model.Call{SIPMessages: msgs}
	warns := Diagnose(call)
	for _, w := range warns {
		if w.Category == "SIP" {
			t.Errorf("unexpected SIP warning: %s", w.Message)
		}
	}
}
