//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package securestore

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"aegis-setup/internal/config"

	"golang.org/x/sys/windows"
)

func storePath() string {
	return filepath.Join(config.DirConfig(), "auth.dat")
}

func storeEncrypted(data []byte) error {
	if len(data) == 0 {
		return removeStore()
	}
	if err := config.AsegurarRutas(); err != nil {
		return err
	}

	var inBlob windows.DataBlob
	inBlob.Size = uint32(len(data))
	inBlob.Data = &data[0]

	var outBlob windows.DataBlob
	// CRYPTPROTECT_UI_FORBIDDEN = 0x1
	err := windows.CryptProtectData(&inBlob, nil, nil, 0, nil, 0x1, &outBlob)
	if err != nil {
		return fmt.Errorf("DPAPI proteger credencial: %w", err)
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(outBlob.Data))))

	enc := make([]byte, outBlob.Size)
	copy(enc, unsafe.Slice(outBlob.Data, outBlob.Size))

	return os.WriteFile(storePath(), enc, 0600)
}

func readEncrypted() ([]byte, error) {
	p := storePath()
	enc, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("leer credencial cifrada: %w", err)
	}
	if len(enc) == 0 {
		return nil, nil
	}

	var inBlob windows.DataBlob
	inBlob.Size = uint32(len(enc))
	inBlob.Data = &enc[0]

	var outBlob windows.DataBlob
	err = windows.CryptUnprotectData(&inBlob, nil, nil, 0, nil, 0x1, &outBlob)
	if err != nil {
		return nil, fmt.Errorf("DPAPI descifrar credencial: %w", err)
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(outBlob.Data))))

	plain := make([]byte, outBlob.Size)
	copy(plain, unsafe.Slice(outBlob.Data, outBlob.Size))
	return plain, nil
}

func removeStore() error {
	p := storePath()
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
