// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReportPathUsaRptCajaChicaYNoLaPrimeraOrdenada(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"A_Primero.rpt", reportUnderCheck} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("rpt"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := reportPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, reportUnderCheck); got != want {
		t.Fatalf("reportPath = %q, want %q", got, want)
	}
}

func TestReportPathFallaSiFaltaRptCajaChica(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "A_Primero.rpt"), []byte("rpt"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := reportPath(dir); err == nil {
		t.Fatal("reportPath aceptó una muestra distinta del reporte objetivo")
	}
}
