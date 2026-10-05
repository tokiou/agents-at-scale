package config

import (
	"testing"
	"time"
)

func TestRetryDelaysDoubleFromBase(t *testing.T) {
	delays := Config{JobMaxAttempts: 4, JobRetryBaseMS: 500}.RetryDelays()
	want := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second}
	if len(delays) != len(want) {
		t.Fatalf("delays = %v, want %v", delays, want)
	}
	for i := range want {
		if delays[i] != want[i] {
			t.Fatalf("delays = %v, want %v", delays, want)
		}
	}
	if got := (Config{JobMaxAttempts: 1, JobRetryBaseMS: 500}).RetryDelays(); len(got) != 1 {
		t.Fatalf("single attempt still needs a delay level for busy conversations, got %v", got)
	}
}
