//go:build windows

package security

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Protect encrypts data using Windows DPAPI bound to the current user profile.
// UI interaction is disabled so the service can run unattended.
func Protect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}
	in := toDataBlob(data)
	var out windows.DataBlob
	err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return copyBlob(out), nil
}

// Unprotect decrypts DPAPI protected data for the current user context.
func Unprotect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}
	in := toDataBlob(data)
	var out windows.DataBlob
	err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return copyBlob(out), nil
}

func toDataBlob(data []byte) windows.DataBlob {
	return windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
}

func copyBlob(blob windows.DataBlob) []byte {
	buf := unsafe.Slice((*byte)(unsafe.Pointer(blob.Data)), int(blob.Size))
	cp := make([]byte, len(buf))
	copy(cp, buf)
	return cp
}
