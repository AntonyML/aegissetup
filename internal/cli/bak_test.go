// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aegis-setup/internal/config"
)

// El respaldo de SIDC pesa ~39 MB y por eso NO viaja dentro del EXE: viaja como asset
// del Release y el operador lo baja a mano. La decisión de producto es que Aegis no
// descarga nada; "aegis bak" existe para que el operador no tenga que adivinar la URL,
// el nombre del archivo ni la carpeta donde dejarlo.
func TestBakSinRespaldoDiceDondeBajarloYFalla(t *testing.T) {
	cfg := config.Default()

	var buf bytes.Buffer
	err := escribirBak(&buf, cfg, "", errors.New("no hay .bak en ninguna de estas carpetas: X"))
	if !errors.Is(err, ErrCheckFail) {
		t.Fatalf("err = %v, quiero ErrCheckFail (sin respaldo no se puede correr Setup DB)", err)
	}

	out := buf.String()
	for _, esperado := range []string{
		"== AEGIS BAK ==",
		"github.com/AntonyML/aegissetup/releases", // de dónde bajarlo
		"SIDC_2014.bak",                           // el nombre del asset
		config.DirBackups(),                       // dónde dejarlo
		"Setup DB",                                // para qué sirve
	} {
		if !strings.Contains(out, esperado) {
			t.Errorf("no aparece %q en:\n%s", esperado, out)
		}
	}
	// El texto tiene que decir explícito que Aegis no descarga: si el operador espera
	// que el comando baje el archivo, se queda mirando una pantalla que no hace nada.
	if !strings.Contains(out, "NO descarga") {
		t.Errorf("no aclaró que Aegis no baja el respaldo:\n%s", out)
	}
}

// Con el respaldo ya en su lugar el comando no es un error: informa cuál se va a
// restaurar y sale con 0. Un script de instalación lo usa como guarda antes del paso 1.
func TestBakConRespaldoDiceCualUsaYNoFalla(t *testing.T) {
	cfg := config.Default()
	bak := filepath.Join(config.DirBackups(), "SIDC_2014.bak")

	var buf bytes.Buffer
	if err := escribirBak(&buf, cfg, bak, nil); err != nil {
		t.Fatalf("con respaldo disponible no puede fallar: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, bak) {
		t.Errorf("no nombró el respaldo que se va a usar:\n%s", out)
	}
	if !strings.Contains(out, "LISTO") {
		t.Errorf("no dio un veredicto positivo:\n%s", out)
	}
}

// El comando tiene que estar colgado del root: un archivo con la función y sin
// registrar en Cobra compila, pasa los tests de la función y no existe para el usuario.
func TestBakEstaRegistradoEnElRoot(t *testing.T) {
	cmd := NewRootCmd("", func(string) (config.Config, error) { return config.Default(), nil })

	var nombres []string
	for _, c := range cmd.Commands() {
		nombres = append(nombres, c.Name())
	}
	if !contains(nombres, "bak") {
		t.Errorf("'bak' no está registrado; comandos: %v", nombres)
	}
}

// Triangulación: la corrida real por Cobra, con un .bak de verdad en disco. Cubre el
// cableado completo (comando registrado → loader → búsqueda → salida), que es donde un
// comando recién agregado se rompe sin que la función suelta falle.
func TestBakEncuentraElRespaldoDeVerdad(t *testing.T) {
	t.Setenv("AEGIS_PROGRAMDATA", t.TempDir())
	cfg := config.Default()
	cfg.BackupDir = t.TempDir()
	bak := filepath.Join(cfg.BackupDir, "SIDC_2014.bak")
	if err := os.WriteFile(bak, []byte("respaldo de mentira"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd("", func(string) (config.Config, error) { return cfg, nil })
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--config", filepath.Join(t.TempDir(), "config.json"), "bak"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("bak con respaldo disponible no puede fallar: %v\n%s", err, buf.String())
	}
	if out := buf.String(); !strings.Contains(out, bak) || !strings.Contains(out, "LISTO") {
		t.Errorf("no reportó el respaldo que encontró:\n%s", out)
	}
}

func contains(xs []string, x string) bool {
	for _, s := range xs {
		if s == x {
			return true
		}
	}
	return false
}
