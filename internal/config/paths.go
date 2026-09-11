// © Antony Monge López — Costa Rica — Céd. 604700548
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Layout de Aegis en una PC instalada.
//
// Son dos árboles distintos y a propósito:
//
//	ProgramData  → datos de máquina: assets, runtime extraído y respaldos.
//	               El instalador corre elevado, así que "Documentos" apuntaría
//	               al perfil del administrador en vez del usuario real.
//	%APPDATA%    → configuración por usuario: config.json.
//
// AegisSetup asume un solo operador por PC, por lo que el config también podría
// vivir en ProgramData; se deja en %APPDATA% por convención de Windows y porque
// la desinstalación tiene que borrar ambos árboles de todas formas (F8).
const (
	nombreProducto = "AegisSetup"
	subBackups     = "sqlserver2014"

	// Overrides para pruebas y para instalaciones no estándar.
	envProgramData = "AEGIS_PROGRAMDATA"
	envAppData     = "AEGIS_APPDATA"
)

// DirProgramData devuelve C:\ProgramData\AegisSetup.
func DirProgramData() string {
	return filepath.Join(baseProgramData(), nombreProducto)
}

// DirAssets devuelve C:\ProgramData\AegisSetup\assets.
func DirAssets() string {
	return filepath.Join(DirProgramData(), "assets")
}

// DirBackups devuelve C:\ProgramData\AegisSetup\assets\backups\sqlserver2014,
// donde el operador deja el .bak en la PC destino.
func DirBackups() string {
	return filepath.Join(DirAssets(), "backups", subBackups)
}

// DirDocker devuelve C:\ProgramData\AegisSetup\docker, donde Aegis deja el compose del
// SQL de pruebas. Va acá y no en la carpeta del proyecto porque en la PC destino no hay
// proyecto: el compose viaja dentro del EXE y se extrae antes de levantarlo.
func DirDocker() string {
	return filepath.Join(DirProgramData(), "docker")
}

// DirConfig devuelve %APPDATA%\AegisSetup.
func DirConfig() string {
	return filepath.Join(baseAppData(), nombreProducto)
}

// RutaConfig devuelve %APPDATA%\AegisSetup\config.json.
func RutaConfig() string {
	return filepath.Join(DirConfig(), "config.json")
}

// AsegurarRutas crea los árboles de datos y de configuración. Es idempotente:
// se llama en cada arranque sin importar si ya existen.
func AsegurarRutas() error {
	for _, dir := range []string{DirBackups(), DirConfig()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("config: crear %s: %w", dir, err)
		}
	}
	return nil
}

// BackupDirs ordena, por prioridad, los directorios donde puede estar el .bak:
// el configurado primero y después los dos layouts "junto al binario".
//
// Los dos últimos existen por dos casos concretos: el operador que copia el
// respaldo al lado del EXE en la PC destino, y el build de desarrollo, que sale
// a bin/ mientras los assets del proyecto quedan un nivel arriba.
func BackupDirs(cfg Config, exeDir string) []string {
	var dirs []string
	add := func(p string) {
		if strings.TrimSpace(p) == "" {
			return
		}
		for _, previo := range dirs {
			if strings.EqualFold(filepath.Clean(previo), filepath.Clean(p)) {
				return
			}
		}
		dirs = append(dirs, p)
	}
	add(cfg.BackupDir)
	add(filepath.Join(exeDir, "assets", "backups", subBackups))
	add(filepath.Join(exeDir, "..", "assets", "backups", subBackups))
	return dirs
}

// baseProgramData resuelve la raíz de datos de máquina. El override va primero
// para que las pruebas nunca toquen el ProgramData real.
func baseProgramData() string {
	if v := strings.TrimSpace(os.Getenv(envProgramData)); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("ProgramData")); v != "" {
		return v
	}
	return `C:\ProgramData`
}

func baseAppData() string {
	if v := strings.TrimSpace(os.Getenv(envAppData)); v != "" {
		return v
	}
	if v, err := os.UserConfigDir(); err == nil && v != "" {
		return v
	}
	return "."
}
