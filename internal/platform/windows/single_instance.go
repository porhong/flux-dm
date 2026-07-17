package windows

import (
	"errors"
	"fmt"
	"strings"

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
