// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"bytes"
	"encoding/json"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aegis-setup/internal/config"
)

func TestPatchSIDCAppAppliesAndRepairsKnownPerformance(t *testing.T) {
	path := filepath.Join("..", "..", "..", "referencia", "SIDC.original.bin")
	if explicit := os.Getenv("AEGIS_LEGACY_PERF_FIXTURE"); explicit != "" {
		path = explicit
	}
	orig, err := os.ReadFile(path)
	if os.IsNotExist(err) && os.Getenv("AEGIS_LEGACY_PERF_FIXTURE") == "" {
		t.Skip("no hay fixture legacy local; fijá AEGIS_LEGACY_PERF_FIXTURE para la integración")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !IsThirdReportCacheSupported(orig) {
		t.Fatal("fixture no compatible con CacheIII")
	}
	dir := t.TempDir()
	origPath := filepath.Join(dir, SIDCOriginalExeName)
	if err := os.WriteFile(origPath, orig, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "Fotos"), 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "Fotos", "Principal.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	logo := creaImagenPrueba(300, 150)
	if err := jpeg.Encode(f, logo, nil); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	// Decodificá el mismo JPEG que usa el flujo, para que el único cambio
	// pendiente de la salida preexistente sea CacheIII.
	lf, err := os.Open(filepath.Join(dir, "Fotos", "Principal.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.Decode(lf)
	lf.Close()
	if err != nil {
		t.Fatal(err)
	}
	// Windows Auth evita credenciales y cambios de reportes/puente en esta prueba.
	cfg := config.Config{AppDir: dir, DsnName: "SIDC_QA", Database: "SIDC", UseWinAuth: true}
	conn, err := BuildExeConnString(cfg.DsnName, cfg.Database, "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	noPerf, _, err := PatchExeBuffer(orig, conn, decoded)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, SIDCExeName)
	// Una conexión/logos ya correctos no deben impedir instalar CacheIII.
	if err := os.WriteFile(target, noPerf, 0600); err != nil {
		t.Fatal(err)
	}
	var logs []string
	stops := 0
	stop := func(string) { stops++ }
	emit := func(s string) { logs = append(logs, s) }
	if err := patchSIDCApp(cfg, "", emit, stop); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	perf, err := PatchThirdReportCache(orig, 100)
	if err != nil {
		t.Fatal(err)
	}
	want, _, err := PatchExeBuffer(perf, conn, decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) || bytes.Equal(got, noPerf) || stops != 1 {
		t.Fatal("la instalación no generó rendimiento, conexión y logos desde el original")
	}
	readConn, err := ReadExeConnString(target)
	if err != nil || readConn != conn {
		t.Fatal("la conexión QA no coincide")
	}
	manifestData, err := os.ReadFile(filepath.Join(dir, "AegisSetup.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest generationManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.LegacyPerformanceProfile != "third-report-cache-100" || manifest.SourceSHA256 != sha256Hex(orig) || len(manifest.Executables) != 1 || manifest.Executables[0].SHA256 != sha256Hex(got) {
		t.Fatal("el manifiesto no acredita el perfil y los hashes reales")
	}
	logs = nil
	if err := patchSIDCApp(cfg, "", emit, stop); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(second, got) || stops != 1 || !strings.Contains(strings.Join(logs, "\n"), "alineado") {
		t.Fatal("reparar cambió una salida ya alineada")
	}
	// Reparar un daño en el trampoline debe recuperar el parche desde el original.
	damaged := append([]byte(nil), got...)
	damaged[thirdReportCave+6] ^= 1
	if err := os.WriteFile(target, damaged, 0600); err != nil {
		t.Fatal(err)
	}
	if err := patchSIDCApp(cfg, "", emit, stop); err != nil {
		t.Fatal(err)
	}
	repaired, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(repaired, want) || stops != 2 {
		t.Fatal("reparar no recuperó exactamente la salida desde original")
	}
	unchanged, err := os.ReadFile(origPath)
	if err != nil || !bytes.Equal(unchanged, orig) {
		t.Fatal("se modificó _ORIGINAL")
	}
}

func TestPatchSIDCAppUnknownVersionWarnsWithoutPerformance(t *testing.T) {
	orig := append([]byte("MZ...HEADER..."), utf16le("Provider=MSDASQL.1;Persist Security Info=False;Data Source=SIDC_SQL;Initial Catalog=SIDC")...)
	orig = append(orig, []byte("...TAIL...")...)
	if IsThirdReportCacheSupported(orig) {
		t.Fatal("se aceptó una versión desconocida")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, SIDCOriginalExeName), orig, 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{AppDir: dir, DsnName: "SIDC_QA", Database: "SIDC", UseWinAuth: true}
	var logs []string
	if err := patchSIDCApp(cfg, "", func(s string) { logs = append(logs, s) }, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(logs, "\n"), "AVISO CACHÉ III") {
		t.Fatal("faltó el aviso de versión no soportada")
	}
	got, err := os.ReadFile(filepath.Join(dir, SIDCExeName))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := BuildExeConnString(cfg.DsnName, cfg.Database, "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	want, _, err := PatchExeBuffer(orig, conn, nil)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("la versión desconocida recibió cambios de rendimiento")
	}
	md, err := os.ReadFile(filepath.Join(dir, "AegisSetup.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest generationManifest
	if err := json.Unmarshal(md, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.LegacyPerformanceProfile != "" {
		t.Fatal("el manifiesto declara un parche no aplicado")
	}
}
