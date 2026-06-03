package q850

import "testing"

func TestName(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{16, "NORMAL_CLEARING"},
		{17, "USER_BUSY"},
		{19, "NO_ANSWER"},
		{21, "CALL_REJECTED"},
		{34, "NORMAL_CIRCUIT_CONGESTION"},
		{999, "UNKNOWN"},
		{-1, "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := Name(tt.code); got != tt.want {
			t.Errorf("Name(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestDescribe(t *testing.T) {
	if got, want := Describe(16), "NORMAL_CLEARING (16)"; got != want {
		t.Errorf("Describe(16) = %q, want %q", got, want)
	}
	if got, want := Describe(999), "UNKNOWN (999)"; got != want {
		t.Errorf("Describe(999) = %q, want %q", got, want)
	}
}
