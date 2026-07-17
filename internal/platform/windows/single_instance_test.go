package windows

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestAcquireInstanceLockRejectsDuplicateAndReleases(t *testing.T) {
	name := fmt.Sprintf("Local\\FluxDM-test-%d", time.Now().UnixNano())
	first, err := AcquireInstanceLock(name)
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}

	second, err := AcquireInstanceLock(name)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second acquisition error = %v, want ErrAlreadyRunning", err)
	}
	if second != nil {
		t.Fatal("second acquisition returned a lock")
	}

	if err := first.Close(); err != nil {
		t.Fatalf("close first lock: %v", err)
	}
	third, err := AcquireInstanceLock(name)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatalf("close released lock: %v", err)
	}
}

func TestAcquireInstanceLockRequiresName(t *testing.T) {
	lock, err := AcquireInstanceLock(" \t")
	if err == nil {
		t.Fatal("expected empty name to be rejected")
	}
	if lock != nil {
		t.Fatal("empty name returned a lock")
	}
}
