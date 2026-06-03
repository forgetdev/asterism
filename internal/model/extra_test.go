package model

import "testing"

func TestDecodeExtraEmpty(t *testing.T) {
	got, err := DecodeExtra("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.HangupCauseSet {
		t.Errorf("empty input should not set HangupCause")
	}
}

func TestDecodeExtraHangup(t *testing.T) {
	in := `{"hangupcause":16,"hangupsource":"PJSIP/1001-00000003","dialstatus":"ANSWER"}`
	got, err := DecodeExtra(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.HangupCauseSet || got.HangupCause != 16 {
		t.Errorf("HangupCause: got %d (set=%v), want 16 (set=true)", got.HangupCause, got.HangupCauseSet)
	}
	if got.HangupSource != "PJSIP/1001-00000003" {
		t.Errorf("HangupSource: got %q", got.HangupSource)
	}
	if got.DialStatus != "ANSWER" {
		t.Errorf("DialStatus: got %q", got.DialStatus)
	}
}

func TestDecodeExtraBridge(t *testing.T) {
	in := `{"bridge_id":"03a544b0-038e-4d2d-95b1-eddbb7cdb866","bridge_technology":"native_rtp"}`
	got, err := DecodeExtra(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.BridgeTechnology != "native_rtp" {
		t.Errorf("BridgeTechnology: got %q, want native_rtp", got.BridgeTechnology)
	}
	if got.BridgeID == "" {
		t.Errorf("BridgeID should be populated")
	}
}

// HangupCause 0 is a valid Q.850 code (UNALLOCATED_NUMBER_OR_UNKNOWN), so the
// presence flag must distinguish it from an absent field.
func TestDecodeExtraHangupCauseZero(t *testing.T) {
	in := `{"hangupcause":0}`
	got, err := DecodeExtra(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.HangupCauseSet {
		t.Errorf("hangupcause:0 should set the presence flag")
	}
	if got.HangupCause != 0 {
		t.Errorf("HangupCause: got %d, want 0", got.HangupCause)
	}
}

func TestDecodeExtraMalformed(t *testing.T) {
	_, err := DecodeExtra(`{not valid json`)
	if err == nil {
		t.Errorf("expected error for malformed JSON, got nil")
	}
}
