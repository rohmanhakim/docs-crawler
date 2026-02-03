package timeutil

import (
	"testing"
	"time"
)

func TestMaxDuration(t *testing.T) {
	tests := []struct {
		name      string
		durations []time.Duration
		want      time.Duration
	}{
		{
			name:      "multiple values returns maximum",
			durations: []time.Duration{100 * time.Millisecond, 500 * time.Millisecond, 200 * time.Millisecond},
			want:      500 * time.Millisecond,
		},
		{
			name:      "single value returns that value",
			durations: []time.Duration{300 * time.Millisecond},
			want:      300 * time.Millisecond,
		},
		{
			name:      "empty slice returns zero",
			durations: []time.Duration{},
			want:      0,
		},
		{
			name:      "all same values returns that value",
			durations: []time.Duration{100 * time.Millisecond, 100 * time.Millisecond, 100 * time.Millisecond},
			want:      100 * time.Millisecond,
		},
		{
			name:      "negative durations handled correctly",
			durations: []time.Duration{-100 * time.Millisecond, 50 * time.Millisecond, -200 * time.Millisecond},
			want:      50 * time.Millisecond,
		},
		{
			name:      "all negative returns least negative",
			durations: []time.Duration{-100 * time.Millisecond, -50 * time.Millisecond, -200 * time.Millisecond},
			want:      -50 * time.Millisecond,
		},
		{
			name:      "zero in mix returns positive max",
			durations: []time.Duration{0, 100 * time.Millisecond, 0},
			want:      100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaxDuration(tt.durations)
			if got != tt.want {
				t.Errorf("MaxDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMaxDurationDoesNotMutateInput(t *testing.T) {
	original := []time.Duration{300 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond}
	expected := []time.Duration{300 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond}

	_ = MaxDuration(original)

	for i := range original {
		if original[i] != expected[i] {
			t.Errorf("MaxDuration mutated input slice: got %v at index %d, want %v", original[i], i, expected[i])
		}
	}
}

func TestDurationPtr(t *testing.T) {
	d := 5 * time.Second
	ptr := DurationPtr(d)

	if ptr == nil {
		t.Fatal("DurationPtr returned nil")
	}

	if *ptr != d {
		t.Errorf("DurationPtr() = %v, want %v", *ptr, d)
	}
}
