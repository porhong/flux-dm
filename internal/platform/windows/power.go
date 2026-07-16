package windows

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	powrprof        = windows.NewLazySystemDLL("powrprof.dll")
	setSuspendState = powrprof.NewProc("SetSuspendState")
)

func Sleep() error     { return suspend(false) }
func Hibernate() error { return suspend(true) }

func suspend(hibernate bool) error {
	value := uintptr(0)
	if hibernate {
		value = 1
	}
	success, _, callErr := setSuspendState.Call(value, 0, 0)
	if success == 0 {
		return fmt.Errorf("request system suspend: %w", callErr)
	}
	return nil
}

func Shutdown() error {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		return fmt.Errorf("open process token: %w", err)
	}
	defer token.Close()
	name, err := windows.UTF16PtrFromString("SeShutdownPrivilege")
	if err != nil {
		return err
	}
	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, name, &luid); err != nil {
		return fmt.Errorf("lookup shutdown privilege: %w", err)
	}
	privileges := windows.Tokenprivileges{PrivilegeCount: 1, Privileges: [1]windows.LUIDAndAttributes{{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}}}
	if err := windows.AdjustTokenPrivileges(token, false, &privileges, uint32(unsafe.Sizeof(privileges)), nil, nil); err != nil {
		return fmt.Errorf("enable shutdown privilege: %w", err)
	}
	const ewxPowerOff = 0x00000008
	const ewxForceIfHung = 0x00000010
	if err := windows.ExitWindowsEx(ewxPowerOff|ewxForceIfHung, 0); err != nil {
		return fmt.Errorf("request shutdown: %w", err)
	}
	return nil
}
