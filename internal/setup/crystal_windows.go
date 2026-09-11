//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

import (
	"os"
	"path/filepath"
)

func checkCrystalSys() []string {
	var missing []string
	for _, f := range CrystalFiles {
		if _, err := os.Stat(filepath.Join(`C:\Windows\SysWOW64`, f)); err != nil {
			missing = append(missing, f)
		}
	}
	return missing
}
