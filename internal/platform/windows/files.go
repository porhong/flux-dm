package windows

import (
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32                        = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW              = shell32.NewProc("ShellExecuteW")
	procSHParseDisplayName         = shell32.NewProc("SHParseDisplayName")
	procSHOpenFolderAndSelectItems = shell32.NewProc("SHOpenFolderAndSelectItems")
	procSHFileOperationW           = shell32.NewProc("SHFileOperationW")
	ole32                          = windows.NewLazySystemDLL("ole32.dll")
	procCoTaskMemFree              = ole32.NewProc("CoTaskMemFree")
)

const (
	swShownormal      = 1
	foDelete          = 3
	fofAllowUndo      = 0x0040
	fofNoConfirmation = 0x0010
)

// FileShell performs direct Windows Shell API calls. No command interpreter is
// used, so filenames are never parsed as shell commands.
type FileShell struct{}

func (FileShell) Open(path string) error {
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode file path: %w", err)
	}
	result, _, callErr := procShellExecuteW.Call(0, 0, uintptr(unsafe.Pointer(file)), 0, 0, swShownormal)
	if result <= 32 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("open file: %w", callErr)
		}
		return fmt.Errorf("open file: shell error %d", result)
	}
	return nil
}

func (FileShell) Reveal(path string) error {
	folderPIDL, releaseFolder, err := parsePIDL(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer releaseFolder()
	filePIDL, releaseFile, err := parsePIDL(path)
	if err != nil {
		return err
	}
	defer releaseFile()
	result, _, callErr := procSHOpenFolderAndSelectItems.Call(folderPIDL, 1, uintptr(unsafe.Pointer(&filePIDL)), 0)
	if int32(result) != 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("reveal file: %w", callErr)
		}
		return fmt.Errorf("reveal file: shell error 0x%x", result)
	}
	return nil
}

func (FileShell) Recycle(path string) error {
	from, err := windows.UTF16FromString(path + "\x00")
	if err != nil {
		return fmt.Errorf("encode file path: %w", err)
	}
	op := shFileOpStruct{wFunc: foDelete, pFrom: &from[0], fFlags: fofAllowUndo | fofNoConfirmation}
	result, _, callErr := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if result != 0 || op.anyOperationsAborted != 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("recycle file: %w", callErr)
		}
		return fmt.Errorf("recycle file: shell error %d", result)
	}
	return nil
}

func parsePIDL(path string) (uintptr, func(), error) {
	value, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, nil, fmt.Errorf("encode file path: %w", err)
	}
	var pidl uintptr
	result, _, callErr := procSHParseDisplayName.Call(uintptr(unsafe.Pointer(value)), 0, uintptr(unsafe.Pointer(&pidl)), 0, 0)
	if int32(result) != 0 || pidl == 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return 0, nil, fmt.Errorf("parse file path: %w", callErr)
		}
		return 0, nil, fmt.Errorf("parse file path: shell error 0x%x", result)
	}
	return pidl, func() { procCoTaskMemFree.Call(pidl) }, nil
}

type shFileOpStruct struct {
	hwnd                 uintptr
	wFunc                uint32
	pFrom                *uint16
	pTo                  *uint16
	fFlags               uint16
	anyOperationsAborted int32
	hNameMappings        uintptr
	progressTitle        *uint16
}
