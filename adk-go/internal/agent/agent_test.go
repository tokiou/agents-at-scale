package agent

import "testing"

func TestNew(t *testing.T) {
	if _, err := New(); err != nil {
		t.Fatalf("New() error = %v", err)
	}
}
