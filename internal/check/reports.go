// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"fmt"
	"os"
	"path/filepath"
)

const reportUnderCheck = "Rpt_Caja_Chica.rpt"

func reportPath(repDir string) (string, error) {
	if repDir == "" {
		return "", fmt.Errorf("carpeta Reportes vacía")
	}
	path := filepath.Join(repDir, reportUnderCheck)
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s es una carpeta", reportUnderCheck)
	}
	return path, nil
}
