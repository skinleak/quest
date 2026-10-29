package components

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0.00 ms"},
		{420 * time.Microsecond, "0.42 ms"},
		{8340 * time.Microsecond, "8.3 ms"},
		{143 * time.Millisecond, "143 ms"},
		{1240 * time.Millisecond, "1.24 s"},
		{9970 * time.Microsecond, "10 ms"},
		{999700 * time.Microsecond, "1.00 s"},
		{90 * time.Second, "90.00 s"},
	}
	for _, tt := range tests {
		if got := FormatDuration(tt.d); got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{2150, "2.1 KB"},
		{10 << 20, "10.0 MB"},
	}
	for _, tt := range tests {
		if got := FormatSize(tt.n); got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
