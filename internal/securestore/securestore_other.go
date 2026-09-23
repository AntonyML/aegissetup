//go:build !windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package securestore

import (
	"encoding/base64"
	"os"
	"path/filepath"

	"aegis-setup/internal/config"
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
	enc := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(enc, data)
	return os.WriteFile(storePath(), enc, 0600)
}

func readEncrypted() ([]byte, error) {
	p := storePath()
	enc, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(enc) == 0 {
		return nil, nil
	}
	data := make([]byte, base64.StdEncoding.DecodedLen(len(enc)))
	n, err := base64.StdEncoding.Decode(data, enc)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func removeStore() error {
	p := storePath()
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
