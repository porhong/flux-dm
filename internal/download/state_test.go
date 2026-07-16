package download

import (
	"errors"
	"testing"
)

func TestDownloadTransition(t *testing.T) {
	task := Download{State: StateQueued}
	for _, next := range []State{StateProbing, StatePreparing, StateDownloading, StateCompleted} {
		if err := task.Transition(next); err != nil {
			t.Fatalf("transition to %s: %v", next, err)
		}
	}
}

func TestDownloadRejectsInvalidTransition(t *testing.T) {
	task := Download{State: StateQueued}
	err := task.Transition(StateCompleted)
	var transitionError *TransitionError
	if !errors.As(err, &transitionError) {
		t.Fatalf("expected TransitionError, got %v", err)
	}
	if task.State != StateQueued {
		t.Fatalf("state changed after invalid transition: %s", task.State)
	}
}

func TestDownloadRecoveryTransitions(t *testing.T) {
	tests := []struct {
		from State
		to   State
	}{
		{StateProbing, StateQueued},
		{StatePreparing, StatePaused},
		{StateDownloading, StatePaused},
		{StatePausing, StateFailed},
		{StateRetrying, StateQueued},
	}

	for _, test := range tests {
		t.Run(string(test.from)+"_to_"+string(test.to), func(t *testing.T) {
			task := Download{State: test.from}
			if err := task.Recover(test.to); err != nil {
				t.Fatalf("recover %s to %s: %v", test.from, test.to, err)
			}
			if task.State != test.to {
				t.Fatalf("recovered state = %s, want %s", task.State, test.to)
			}
		})
	}
}

func TestDownloadRejectsInvalidRecoveryTransition(t *testing.T) {
	task := Download{State: StateCompleted}
	if err := task.Recover(StateQueued); err == nil {
		t.Fatal("expected completed download recovery to be rejected")
	}
}
