// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

func TestPatchReportConnectionBytesCubreFormatosYNormalizaUsuario(t *testing.T) {
	data := []byte("TrustedConnection=Yes;Integrated Security=SSPI;UID=" + strings.Repeat("x", 8) + ";")
	patched, n, err := PatchReportConnectionBytesForUser(data, "sidc")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 || len(patched) != len(data) {
		t.Fatalf("n=%d len=%d want 3 y %d", n, len(patched), len(data))
	}
	if bytes.Contains(patched, []byte("TrustedConnection=Yes")) || bytes.Contains(patched, []byte("Integrated Security=SSPI")) {
		t.Fatal("quedó autenticación integrada en un formato alternativo")
	}
	if !bytes.Contains(patched, []byte("TrustedConnection=No\x00")) || !bytes.Contains(patched, []byte("Integrated Security=No\x00")) {
		t.Fatal("no se conservaron los tamaños de los bloques alternativos")
	}
	if !bytes.Contains(patched, []byte("UID=sidc    ")) {
		t.Fatal("no se normalizó UID con relleno de igual longitud")
	}
}

func TestPatchRptCajaChicaUsaCopiaYNoTocaElOriginal(t *testing.T) {
	src := rutaFixtureCajaChica(t)
	original, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	dst := filepath.Join(dir, "Rpt_Caja_Chica.rpt")
	if err := os.WriteFile(dst, original, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := PatchReportsConnectionsForUser(dir, "sidc", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Files != 0 && (got.Files != 1 || got.Occurrences < 6) {
		t.Fatalf("patch de copia = %+v; esperaba los seis bloques de conexión", got)
	}

	unchanged, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(unchanged, original) {
		t.Fatal("el fixture original fue modificado")
	}

	patched, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if got.Files == 0 {
		if !bytes.Equal(original, patched) {
			t.Fatal("una plantilla ya normalizada cambió al pasar por el parche")
		}
		return
	}
	if len(patched) != len(original) || bytes.Contains(patched, []byte("Trusted_Connection=Yes")) || !bytes.Contains(patched, []byte("UID=sidc")) {
		t.Fatalf("la copia no quedó parcheada con tamaño constante: patch=%+v len=%v yes=%v uid=%v", got, len(patched) == len(original), bytes.Contains(patched, []byte("Trusted_Connection=Yes")), bytes.Contains(patched, []byte("UID=sidc")))
	}
}

func rutaFixtureCajaChica(t *testing.T) string {
	t.Helper()
	candidates := []string{
		os.Getenv("SIDC_RPT_CAJACHICA_FIXTURE"),
		`C:\DEV\SIDC\Reportes\Rpt_Caja_Chica.rpt`,
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	t.Skip("no hay una copia local de Reportes/Rpt_Caja_Chica.rpt para la regresión")
	return ""
}
