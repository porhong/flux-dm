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

func TestInstanceActivatorNotifiesRunningInstance(t *testing.T) {
	name := fmt.Sprintf("Local\\FluxDM-activation-test-%d", time.Now().UnixNano())
	listener, err := NewInstanceActivator(name)
	if err != nil {
		t.Fatalf("create listener: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	received := make(chan struct{}, 1)
	if err := listener.Start(func() { received <- struct{}{} }); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	signaler, err := NewInstanceActivator(name)
	if err != nil {
		t.Fatalf("create signaler: %v", err)
	}
	t.Cleanup(func() { _ = signaler.Close() })
	if err := signaler.Notify(); err != nil {
		t.Fatalf("notify listener: %v", err)
	}

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("activation notification was not received")
	}
}

func TestInstanceActivatorRequiresNameAndHandler(t *testing.T) {
	activator, err := NewInstanceActivator(" \t")
	if err == nil {
		t.Fatal("expected empty activation name to be rejected")
	}
	if activator != nil {
		t.Fatal("empty activation name returned an activator")
	}

	activator, err = NewInstanceActivator(fmt.Sprintf("Local\\FluxDM-activation-handler-test-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("create activator: %v", err)
	}
	t.Cleanup(func() { _ = activator.Close() })
	if err := activator.Start(nil); err == nil {
		t.Fatal("expected nil activation handler to be rejected")
	}
}
