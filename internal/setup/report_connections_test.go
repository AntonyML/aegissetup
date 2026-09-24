// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPatchReportConnectionBytesPreservaTamanoYAmbasCodificaciones(t *testing.T) {
	data := append([]byte("prefix Trusted_Connection=Yes suffix\x00"), utf16le("Trusted_Connection=Yes")...)
	patched, n, err := PatchReportConnectionBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(patched) != len(data) {
		t.Fatalf("n=%d len=%d want 2 y %d", n, len(patched), len(data))
	}
	if bytes.Contains(patched, trustedYesASCII) || bytes.Contains(patched, trustedYesUTF16) {
		t.Fatal("quedó autenticación integrada")
	}
	if !bytes.Contains(patched, trustedNoASCII) || !bytes.Contains(patched, trustedNoUTF16) {
		t.Fatal("no se reemplazaron ambas representaciones")
	}
}

func TestPatchReportsConnectionsEsAtomicoYRepetible(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "muestra.rpt")
	data := append([]byte("Trusted_Connection=Yes"), bytes.Repeat([]byte{0xA5}, 64)...)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	got, err := PatchReportsConnections(dir, nil)
	if err != nil || got.Files != 1 || got.Occurrences != 1 {
		t.Fatalf("patch = %+v err=%v", got, err)
	}
	patched, _ := os.ReadFile(path)
	if len(patched) != len(data) || bytes.Contains(patched, trustedYesASCII) {
		t.Fatal("patch no preservó tamaño o dejó la cadena vieja")
	}
	got, err = PatchReportsConnections(dir, nil)
	if err != nil || got.Files != 0 || got.Occurrences != 0 {
		t.Fatalf("segunda corrida = %+v err=%v", got, err)
	}
}
