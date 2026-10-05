package runtime

import (
	"slices"
	"testing"

	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

func TestPendingInterruptIgnoresAnsweredRequests(t *testing.T) {
	request := func(id string) *session.Event {
		return &session.Event{RequestedInput: &session.RequestInput{InterruptID: id}}
	}
	answer := func(id string) *session.Event {
		event := &session.Event{}
		event.Content = &genai.Content{Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{ID: id}}}}
		return event
	}
	cases := map[string]struct {
		events []*session.Event
		want   string
	}{
		"none":     {nil, ""},
		"pending":  {[]*session.Event{request("a")}, "a"},
		"answered": {[]*session.Event{request("a"), answer("a")}, ""},
		"latest":   {[]*session.Event{request("a"), answer("a"), request("b")}, "b"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := PendingInterrupt(slices.Values(tc.events)); got != tc.want {
				t.Fatalf("PendingInterrupt() = %q, want %q", got, tc.want)
			}
		})
	}
}
