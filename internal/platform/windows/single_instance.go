package windows

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/windows"
)

// ErrAlreadyRunning reports that another process already owns the application lock.
var ErrAlreadyRunning = errors.New("application is already running")

// InstanceLock owns a named Windows mutex for the lifetime of one application process.
// Windows releases the mutex if the process exits unexpectedly, so it cannot become stale
// after sleep, hibernation, or a crash.
type InstanceLock struct {
	handle windows.Handle
}

// InstanceActivator signals the running desktop process that it should restore
// and foreground its window. It owns at most one waiting goroutine, which Close
// always releases before its event handle is closed.
type InstanceActivator struct {
	handle    windows.Handle
	startOnce sync.Once
	closeOnce sync.Once
	closed    atomic.Bool
	done      chan struct{}
	closeErr  error
}

// AcquireInstanceLock prevents more than one FluxDM desktop process from running in a
// user session. The name must be stable for every build of the desktop application.
func AcquireInstanceLock(name string) (*InstanceLock, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("instance lock name is required")
	}

	handle, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(name))
	if err == nil {
		return &InstanceLock{handle: handle}, nil
	}
	if err == windows.ERROR_ALREADY_EXISTS {
		// CreateMutex returns a valid handle even when another process created the
		// mutex first. Close our reference immediately; the other process keeps it.
		_ = windows.CloseHandle(handle)
		return nil, ErrAlreadyRunning
	}
	return nil, fmt.Errorf("create instance lock: %w", err)
}

// Close releases this process's reference to the instance mutex.
func (l *InstanceLock) Close() error {
	if l == nil || l.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(l.handle)
	l.handle = 0
	if err != nil {
		return fmt.Errorf("close instance lock: %w", err)
	}
	return nil
}

// NewInstanceActivator creates or opens an auto-reset event shared by every
// FluxDM process in the current user session.
func NewInstanceActivator(name string) (*InstanceActivator, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("instance activation name is required")
	}

	handle, err := windows.CreateEvent(nil, 0, 0, windows.StringToUTF16Ptr(name))
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return nil, fmt.Errorf("create instance activation event: %w", err)
	}
	if handle == 0 {
		return nil, errors.New("create instance activation event: invalid handle")
	}
	return &InstanceActivator{handle: handle, done: make(chan struct{})}, nil
}

// Start waits for activation requests and invokes handler once for each request.
// It is safe to call only once; subsequent calls leave the original handler active.
func (a *InstanceActivator) Start(handler func()) error {
	if a == nil || a.handle == 0 {
		return errors.New("instance activator is not initialized")
	}
	if handler == nil {
		return errors.New("instance activation handler is required")
	}
	if a.closed.Load() {
		return errors.New("instance activator is closed")
	}

	a.startOnce.Do(func() {
		go func() {
			defer close(a.done)
			for {
				result, waitErr := windows.WaitForSingleObject(a.handle, windows.INFINITE)
				if waitErr != nil || result != windows.WAIT_OBJECT_0 || a.closed.Load() {
					return
				}
				handler()
			}
		}()
	})
	return nil
}

// Notify asks the primary instance to restore its window.
func (a *InstanceActivator) Notify() error {
	if a == nil || a.handle == 0 {
		return errors.New("instance activator is not initialized")
	}
	if err := windows.SetEvent(a.handle); err != nil {
		return fmt.Errorf("signal instance activation event: %w", err)
	}
	return nil
}

// Close stops the listener, if running, and releases the event handle.
func (a *InstanceActivator) Close() error {
	if a == nil {
		return nil
	}
	a.closeOnce.Do(func() {
		a.closed.Store(true)
		a.startOnce.Do(func() { close(a.done) })
		if a.handle != 0 {
			if err := windows.SetEvent(a.handle); err != nil {
				a.closeErr = fmt.Errorf("wake instance activation listener: %w", err)
			}
			<-a.done
			if err := windows.CloseHandle(a.handle); err != nil && a.closeErr == nil {
				a.closeErr = fmt.Errorf("close instance activation event: %w", err)
			}
			a.handle = 0
		}
	})
	return a.closeErr
}
