package interval

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"30m", 30 * time.Minute, false},
		{"6h", 6 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"1m", time.Minute, false},
		{"30s", 0, true},
		{"", 0, true},
		{"abc", 0, true},
	}

	for _, tc := range tests {
		got, err := Parse(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("Parse(%q) expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("Parse(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}