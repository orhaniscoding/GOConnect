//go:build windows

package security

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Protect encrypts data using Windows DPAPI. It uses the CRYPTPROTECT_LOCAL_MACHINE
// flag, which means the encryption is bound to the machine, not the user.
// This allows a service running as LocalSystem to decrypt data that was
// encrypted by a user account (e.g., during an 'install' command).
func Protect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}
	in := toDataBlob(data)
	var out windows.DataBlob
	// Use CRYPTPROTECT_LOCAL_MACHINE to make it machine-wide and CRYPTPROTECT_UI_FORBIDDEN
	err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN|windows.CRYPTPROTECT_LOCAL_MACHINE, &out)
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return copyBlob(out), nil
}

// Unprotect decrypts DPAPI protected data. It uses the CRYPTPROTECT_LOCAL_MACHINE
// flag to allow decryption of machine-bound data.
func Unprotect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}
	in := toDataBlob(data)
	var out windows.DataBlob
	// Add an optional prompt struct to further ensure no UI is requested.
	prompt := windows.CryptProtectPromptStruct{
		Size: uint32(unsafe.Sizeof(windows.CryptProtectPromptStruct{})),
		// Flags: windows.CRYPTPROTECT_PROMPT_ON_UNPROTECT, // This flag is not available in the struct
		Prompt: nil,
	}
	// Use CRYPTPROTECT_LOCAL_MACHINE to decrypt machine-wide data
	err := windows.CryptUnprotectData(&in, nil, nil, 0, &prompt, windows.CRYPTPROTECT_UI_FORBIDDEN|windows.CRYPTPROTECT_LOCAL_MACHINE, &out)
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
