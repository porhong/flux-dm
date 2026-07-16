package secrets

import (
	"fmt"
	"golang.org/x/sys/windows"
	"unsafe"
)

type Protector interface {
	Protect([]byte) ([]byte, error)
	Unprotect([]byte) ([]byte, error)
}
type DPAPI struct{}

func (DPAPI) Protect(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}
	input := windows.DataBlob{Size: uint32(len(plaintext)), Data: &plaintext[0]}
	var output windows.DataBlob
	description, _ := windows.UTF16PtrFromString("FluxDM protected data")
	if err := windows.CryptProtectData(&input, description, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, fmt.Errorf("protect secret: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	return append([]byte(nil), unsafe.Slice(output.Data, output.Size)...), nil
}
func (DPAPI) Unprotect(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}
	input := windows.DataBlob{Size: uint32(len(ciphertext)), Data: &ciphertext[0]}
	var output windows.DataBlob
	var description *uint16
	if err := windows.CryptUnprotectData(&input, &description, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, fmt.Errorf("unprotect secret: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	if description != nil {
		defer windows.LocalFree(windows.Handle(unsafe.Pointer(description)))
	}
	return append([]byte(nil), unsafe.Slice(output.Data, output.Size)...), nil
}
