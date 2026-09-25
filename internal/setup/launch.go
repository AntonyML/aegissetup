// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// LaunchSIDC abre la copia de SIDC preparada por Aegis y devuelve el control
// inmediatamente. Aegis no espera ni mata el proceso: SIDC es la aplicación
// interactiva que el operador seguirá usando después de cerrar el TUI.
func LaunchSIDC(appDir string) error {
	if appDir == "" {
		return fmt.Errorf("app_dir sin configurar")
	}

	exe := filepath.Join(appDir, SIDCExeName)
	if _, err := os.Stat(exe); err != nil {
		return fmt.Errorf("SIDC preparado no encontrado en %s: %w", exe, err)
	}

	cmd := exec.Command(exe)
	cmd.Dir = appDir
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("no se pudo abrir SIDC: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("SIDC arrancó pero no se pudo liberar su proceso: %w", err)
	}
	return nil
}
