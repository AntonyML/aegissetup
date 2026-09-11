//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func checkCrystalSys() []string {
	var missing []string
	for _, f := range CrystalFiles {
		if _, err := os.Stat(filepath.Join(SysWOW64, f)); err != nil {
			missing = append(missing, f)
		}
	}
	return missing
}

// checkOCXSys reporta qué OCX faltan en SysWOW64. Vive acá, junto a
// checkCrystalSys, porque las dos son la misma pregunta: qué hay ya puesto en
// el subsistema de 32 bits.
func checkOCXSys() []string {
	var missing []string
	for _, f := range RequiredOCX {
		if _, err := os.Stat(filepath.Join(SysWOW64, f)); err != nil {
			missing = append(missing, f)
		}
	}
	return missing
}

// RegisterCOM registra un componente COM de 32 bits.
//
// Se usa el regsvr32 de SysWOW64 y no el de System32 a propósito: el de System32 es
// de 64 bits y rechaza (o registra en el lugar equivocado) un control de 32, que es
// lo que son todos los de esta aplicación. /s lo deja sin carteles: la instalación no
// se detiene a esperar que alguien apriete Aceptar en cada uno de los 43 archivos.
func RegisterCOM(dll string) error {
	reg := filepath.Join(SysWOW64, "regsvr32.exe")
	out, err := exec.Command(reg, "/s", dll).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
