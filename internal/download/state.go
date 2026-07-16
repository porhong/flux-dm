package download

import "fmt"

type State string

const (
	StateQueued      State = "queued"
	StateProbing     State = "probing"
	StatePreparing   State = "preparing"
	StateDownloading State = "downloading"
	StatePausing     State = "pausing"
	StatePaused      State = "paused"
	StateRetrying    State = "retrying"
	StateCompleted   State = "completed"
	StateFailed      State = "failed"
	StateCancelled   State = "cancelled"
)

var transitions = map[State]map[State]struct{}{
	StateQueued:      {StateProbing: {}, StateCancelled: {}},
	StateProbing:     {StatePreparing: {}, StateFailed: {}, StateCancelled: {}},
	StatePreparing:   {StateDownloading: {}, StateFailed: {}, StateCancelled: {}},
	StateDownloading: {StatePausing: {}, StateCompleted: {}, StateFailed: {}, StateCancelled: {}},
	StatePausing:     {StatePaused: {}, StateFailed: {}, StateCancelled: {}},
	StatePaused:      {StatePreparing: {}, StateQueued: {}, StateFailed: {}, StateCancelled: {}},
	StateRetrying:    {StatePreparing: {}, StateFailed: {}, StateCancelled: {}},
	StateFailed:      {StateRetrying: {}, StateQueued: {}},
	StateCancelled:   {StateQueued: {}},
}

var recoveryTransitions = map[State]map[State]struct{}{
	StateProbing:     {StateQueued: {}, StateFailed: {}},
	StatePreparing:   {StateQueued: {}, StatePaused: {}, StateFailed: {}},
	StateDownloading: {StateQueued: {}, StatePaused: {}, StateFailed: {}},
	StatePausing:     {StateQueued: {}, StatePaused: {}, StateFailed: {}},
	StateRetrying:    {StateQueued: {}, StatePaused: {}, StateFailed: {}},
}

type TransitionError struct {
	From State
	To   State
}

func (d *Download) Recover(to State) error {
	if _, ok := recoveryTransitions[d.State][to]; !ok {
		return &TransitionError{From: d.State, To: to}
	}
	d.State = to
	return nil
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("invalid download state transition %q -> %q", e.From, e.To)
}

func (d *Download) Transition(to State) error {
	if _, ok := transitions[d.State][to]; !ok {
		return &TransitionError{From: d.State, To: to}
	}
	d.State = to
	return nil
}
