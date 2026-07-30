package windows

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/fluxdm/fluxdm/internal/update"
	"golang.org/x/sys/windows"
)

var wintrust = windows.NewLazySystemDLL("wintrust.dll").NewProc("WinVerifyTrust")

var wintrustActionGenericVerifyV2 = windows.GUID{Data1: 0x00AAC56B, Data2: 0xCD44, Data3: 0x11D0, Data4: [8]byte{0x8C, 0xC2, 0x00, 0xC0, 0x4F, 0xC2, 0x95, 0xEE}}

type winTrustFileInfo struct {
	cbStruct       uint32
	pcwszFilePath  *uint16
	hFile          windows.Handle
	pgKnownSubject *windows.GUID
}
type winTrustData struct {
	cbStruct            uint32
	pPolicyCallbackData uintptr
	pSIPClientData      uintptr
	dwUIChoice          uint32
	fdwRevocationChecks uint32
	dwUnionChoice       uint32
	pFile               *winTrustFileInfo
	dwStateAction       uint32
	hWVTStateData       windows.Handle
	pwszURLReference    *uint16
	dwProvFlags         uint32
	dwUIContext         uint32
}

// AuthenticodeVerifier requires Windows to validate the embedded signature
// against its trusted certificate chain. WinVerifyTrust includes timestamp
// validation when the signer carries a trusted RFC 3161 timestamp.
type AuthenticodeVerifier struct{}

func (AuthenticodeVerifier) VerifyProductionInstaller(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if filepath.Ext(abs) != ".exe" {
		return errors.New("update installer is not an executable")
	}
	name, err := windows.UTF16PtrFromString(abs)
	if err != nil {
		return err
	}
	fileInfo := winTrustFileInfo{cbStruct: uint32(unsafe.Sizeof(winTrustFileInfo{})), pcwszFilePath: name}
	data := winTrustData{cbStruct: uint32(unsafe.Sizeof(winTrustData{})), dwUIChoice: 2, dwUnionChoice: 1, pFile: &fileInfo}
	status, _, callErr := wintrust.Call(0, uintptr(unsafe.Pointer(&wintrustActionGenericVerifyV2)), uintptr(unsafe.Pointer(&data)))
	if status != 0 {
		return fmt.Errorf("WinVerifyTrust failed: 0x%08x", uint32(status))
	}
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return nil
}

type UpdateLauncher struct{ HelperPath, RestartPath, CacheDir string }

func (l UpdateLauncher) Launch(ctx context.Context, installerPath string, handoff update.Handoff) error {
	if !filepath.IsAbs(installerPath) || filepath.Ext(installerPath) != ".exe" {
		return errors.New("invalid verified installer path")
	}
	if _, err := os.Stat(installerPath); err != nil {
		return err
	}
	if l.HelperPath == "" || l.RestartPath == "" || l.CacheDir == "" || !filepath.IsAbs(handoff.ResultPath) || handoff.TargetVersion == "" || handoff.Token == "" {
		return errors.New("update launcher is not configured")
	}
	if err := os.MkdirAll(l.CacheDir, 0o700); err != nil {
		return err
	}
	// The installed helper will be replaced by NSIS. Execute a private copy so
	// it cannot keep Program Files locked while the update is installed.
	copyPath := filepath.Join(l.CacheDir, "FluxDM.UpdateLauncher.exe")
	if err := copyFile(l.HelperPath, copyPath); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, copyPath, "-parent-pid", strconv.Itoa(os.Getpid()), "-installer", installerPath, "-restart", l.RestartPath, "-target-version", handoff.TargetVersion, "-token", handoff.Token, "-result", handoff.ResultPath)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return command.Start()
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
