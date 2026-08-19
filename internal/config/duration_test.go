package config

import (
	"testing"
	"time"
)

func TestDurationUnmarshalText(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"5m", 5 * time.Minute, false},
		{"90s", 90 * time.Second, false},
		{"1h30m", 90 * time.Minute, false},
		{"not-a-duration", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			var d duration
			err := d.UnmarshalText([]byte(tt.in))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("UnmarshalText(%q): expected error, got none", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("UnmarshalText(%q): unexpected error: %v", tt.in, err)
			}
			if time.Duration(d) != tt.want {
				t.Errorf("UnmarshalText(%q) = %v, want %v", tt.in, time.Duration(d), tt.want)
			}
		})
	}
}

func TestDurationString(t *testing.T) {
	d := duration(90 * time.Second)
	if got, want := d.String(), "1m30s"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
