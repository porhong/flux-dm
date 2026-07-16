package application

import (
	"errors"
	"testing"
)

func TestApplicationErrorPreservesCodeAndCause(t *testing.T) {
	cause := errors.New("offline")
	err := NewError(ErrUnavailable, "service unavailable", cause)

	if err.Code != ErrUnavailable {
		t.Fatalf("expected %q, got %q", ErrUnavailable, err.Code)
	}
	if !errors.Is(err, cause) {
		t.Fatal("expected wrapped cause")
	}
	if err.Error() != "unavailable: service unavailable" {
		t.Fatalf("backend cause leaked through public error: %q", err.Error())
	}
}
