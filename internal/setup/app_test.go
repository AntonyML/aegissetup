// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aegis-setup/internal/config"
)

// El presupuesto del parche no es un numero suelto: sale de que "Initial Catalog=SIDC"
// mide 20 chars y el reemplazo es "UID=<u>;PWD=<p>".
func TestDockerPatchBudget(t *testing.T) {
	cases := []struct {
		user string
		want int
	}{
		{"dev", 8},              // 20 - 4 - 5 - 3
		{"sidc_dev", 3},         // por esto el login de dev no puede llamarse sidc_dev
		{"", 11},                // 20 - 4 - 5
		{"unNombrMuyLargo", -4}, // 15 chars: no cabe nada, ni una clave vacia
	}
	for _, tc := range cases {
		if got := DockerPatchBudget(tc.user); got != tc.want {
			t.Errorf("DockerPatchBudget(%q) = %d, quiero %d", tc.user, got, tc.want)
		}
	}
}

// La propiedad que importa: aceptar por presupuesto <=> aceptar por el parche.
func TestDockerPatchBudgetEsLaReglaDelParche(t *testing.T) {
	users := []string{"dev", "sidc_dev", "sa", ""}
	passes := []string{"", "a", "Dv*123", "123456789", "12345678901", "unaClaveLarguisima"}
	for _, u := range users {
		for _, p := range passes {
			cabe := len("UID="+u+";PWD="+p) <= 20
			quiero := len(p) <= DockerPatchBudget(u)
			if cabe != quiero {
				t.Errorf("user=%q pass=%q: parche acepta=%v, presupuesto acepta=%v",
					u, p, cabe, quiero)
			}
		}
	}
}

// exeFalso arma un binario minimo que contiene el string real que busca el parche.
func exeFalso(t *testing.T, extra string) (dir, nombre string, tamano int) {
	t.Helper()
	dir = t.TempDir()
	nombre = "Sistema Intergrado de Controles y Presupuesto.exe"
	data := append([]byte("MZ...relleno..."), utf16le("Initial Catalog=SIDC")...)
	data = append(data, []byte("...cola...")...)
	data = append(data, utf16le(extra)...)
	if err := os.WriteFile(filepath.Join(dir, nombre), data, 0644); err != nil {
		t.Fatal(err)
	}
	return dir, nombre, len(data)
}

func TestPatchDockerExeNoAlargaElBinario(t *testing.T) {
	dir, nombre, tamano := exeFalso(t, "")
	var lines []string
	if err := PatchDockerExe(dir, "dev", "Dv*123", func(s string) { lines = append(lines, s) }); err != nil {
		t.Fatalf("parche fallo: %v", err)
	}

	dst := filepath.Join(dir, "Sistema Intergrado de Controles y Presupuesto_DOCKER.exe")
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("no se creo el _DOCKER.exe: %v", err)
	}
	if len(data) != tamano {
		t.Errorf("el binario cambio de tamano: %d, era %d", len(data), tamano)
	}
	if !contieneBytes(data, utf16le("UID=dev;PWD=Dv*123")) {
		t.Error("no quedo embebido UID=dev;PWD=Dv*123")
	}
	if contieneBytes(data, utf16le("Initial Catalog=SIDC")) {
		t.Error("quedo el string original sin parchear")
	}

	// El original no se toca: es la unica copia buena de la app.
	orig, err := os.ReadFile(filepath.Join(dir, nombre))
	if err != nil {
		t.Fatal(err)
	}
	if len(orig) != tamano || contieneBytes(orig, utf16le("UID=dev")) {
		t.Error("se modifico el exe original")
	}
	if len(lines) == 0 {
		t.Error("el parche no reporto nada")
	}
}

func TestPatchDockerExeRechazaClaveLargaSinEscribirNada(t *testing.T) {
	dir, _, _ := exeFalso(t, "")
	// 9 chars con user dev = 21 > 20.
	if err := PatchDockerExe(dir, "dev", "123456789", func(string) {}); err == nil {
		t.Fatal("acepto una clave que no cabe")
	}
	dst := filepath.Join(dir, "Sistema Intergrado de Controles y Presupuesto_DOCKER.exe")
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Error("escribio el _DOCKER.exe aunque rechazo la clave")
	}
}

func TestPatchDockerExeFallaSiNoEncuentraElString(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Sistema Intergrado de Controles y Presupuesto.exe"),
		[]byte("binario sin el string"), 0644); err != nil {
		t.Fatal(err)
	}
	err := PatchDockerExe(dir, "dev", "Dv*123", func(string) {})
	if err == nil {
		t.Fatal("no fallo con un exe que no tiene el string")
	}
}

func contieneBytes(hay, aguja []byte) bool {
	if len(aguja) == 0 {
		return true
	}
	for i := 0; i+len(aguja) <= len(hay); i++ {
		ok := true
		for j := range aguja {
			if hay[i+j] != aguja[j] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func TestBuildExeConnString(t *testing.T) {
	// 1. Windows Auth
	cs, err := BuildExeConnString("SIDC_SQL", "SIDC", "sidc", "", true)
	if err != nil {
		t.Fatalf("BuildExeConnString win-auth falló: %v", err)
	}
	if len(cs) > MaxExeConnBudget {
		t.Errorf("len(cs) = %d > %d", len(cs), MaxExeConnBudget)
	}
	if !strings.Contains(cs, "Initial Catalog=SIDC;") {
		t.Errorf("win-auth debe incluir Initial Catalog: %s", cs)
	}

	// 2. SQL Auth con clave corta (cabe Initial Catalog)
	csShort, err := BuildExeConnString("SIDC_SQL", "SIDC", "sidc", "ClaveCorta1", false)
	if err != nil {
		t.Fatalf("BuildExeConnString clave corta falló: %v", err)
	}
	if len(csShort) > MaxExeConnBudget {
		t.Errorf("len(csShort) = %d > %d", len(csShort), MaxExeConnBudget)
	}
	if !strings.Contains(csShort, "Initial Catalog=SIDC;") {
		t.Errorf("clave corta debe incluir Initial Catalog: %s", csShort)
	}
	if !strings.Contains(csShort, "UID=sidc;PWD=ClaveCorta1;") {
		t.Errorf("debe incluir credenciales: %s", csShort)
	}

	// 3. SQL Auth con clave de producción (24 chars) -> omite Initial Catalog y cabe en <= 88
	prodPass := "dwHmNy+rx+Ehu#q%EwW*vBrp" // 24 chars
	csProd, err := BuildExeConnString("SIDC_SQL", "SIDC", "sidc", prodPass, false)
	if err != nil {
		t.Fatalf("BuildExeConnString clave prod falló: %v", err)
	}
	if len(csProd) > MaxExeConnBudget {
		t.Errorf("len(csProd) = %d > %d", len(csProd), MaxExeConnBudget)
	}
	if strings.Contains(csProd, "Initial Catalog=") {
		t.Errorf("con clave de 24 chars no debe incluir Initial Catalog para no desbordar: %s", csProd)
	}
	if !strings.Contains(csProd, "UID=sidc;PWD="+prodPass+";") {
		t.Errorf("debe incluir credenciales completas: %s", csProd)
	}

	// 4. Clave que excede el límite absoluto (> 35 chars para sidc)
	_, err = BuildExeConnString("SIDC_SQL", "SIDC", "sidc", "unaClaveGigantescaDeMasDeCuarentaCaracteresSuperandoElPresupuesto", false)
	if err == nil {
		t.Fatalf("debió fallar con clave excesiva")
	}
}

func TestPatchExeBuffer(t *testing.T) {
	fakeHeader := []byte("MZ...HEADER...")
	fakeConnSlot := utf16le("Provider=MSDASQL.1;Persist Security Info=False;Data Source=SIDC_SQL;Initial Catalog=SIDC")
	fakeTail := []byte("...TAIL...")

	orig := append(fakeHeader, fakeConnSlot...)
	orig = append(orig, fakeTail...)
	origLen := len(orig)

	newConn := "Provider=MSDASQL.1;Data Source=SIDC_SQL;UID=sidc;PWD=SecretPass123;"
	patched, _, err := PatchExeBuffer(orig, newConn, nil)
	if err != nil {
		t.Fatalf("PatchExeBuffer falló: %v", err)
	}
	if len(patched) != origLen {
		t.Fatalf("len(patched) = %d != %d", len(patched), origLen)
	}

	// Verificar que el slot contiene la nueva cadena
	if !contieneBytes(patched, utf16le("UID=sidc;PWD=SecretPass123;")) {
		t.Errorf("no se encontró la nueva credencial en el buffer parchado")
	}
	// Y que no queda Persist Security Info
	if contieneBytes(patched, utf16le("Persist Security Info=False")) {
		t.Errorf("quedó Persist Security Info en el buffer")
	}
}

func TestFindOriginalExe(t *testing.T) {
	dir := t.TempDir()

	// 1. Sin archivo
	_, err := FindOriginalExe(dir)
	if err == nil {
		t.Fatal("debió fallar si no existe el archivo")
	}

	// 2. Con _ORIGINAL.exe directo en appDir
	p1 := filepath.Join(dir, SIDCOriginalExeName)
	if err := os.WriteFile(p1, []byte("ORIGINAL_DATA"), 0644); err != nil {
		t.Fatal(err)
	}
	found, err := FindOriginalExe(dir)
	if err != nil || found != p1 {
		t.Fatalf("FindOriginalExe = %q, want %q, err: %v", found, p1, err)
	}

	// 3. Con _ORIGINAL.exe en Respaldo_SIDC
	_ = os.Remove(p1)
	respDir := filepath.Join(dir, "Respaldo_SIDC")
	_ = os.MkdirAll(respDir, 0755)
	p2 := filepath.Join(respDir, SIDCOriginalExeName)
	if err := os.WriteFile(p2, []byte("ORIGINAL_BACKUP_DATA"), 0644); err != nil {
		t.Fatal(err)
	}
	found2, err := FindOriginalExe(dir)
	if err != nil || found2 != p2 {
		t.Fatalf("FindOriginalExe = %q, want %q, err: %v", found2, p2, err)
	}
}

func TestReadExeConnStringAndIdempotency(t *testing.T) {
	fakeHeader := []byte("MZ...HEADER...")
	fakeConnSlot := utf16le("Provider=MSDASQL.1;Persist Security Info=False;Data Source=SIDC_SQL;Initial Catalog=SIDC")
	fakeTail := []byte("...TAIL...")
	orig := append(fakeHeader, fakeConnSlot...)
	orig = append(orig, fakeTail...)

	dir := t.TempDir()
	origPath := filepath.Join(dir, SIDCOriginalExeName)
	if err := os.WriteFile(origPath, orig, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		AppDir:   dir,
		DsnName:  "SIDC_SQL",
		Database: "SIDC",
		SQLUser:  "sidc",
	}

	var logs []string
	emit := func(s string) { logs = append(logs, s) }

	// Corrida 1: genera el ejecutable
	if err := PatchSIDCApp(cfg, "dwHmNy+rx+Ehu#q%EwW*vBrp", emit); err != nil {
		t.Fatalf("PatchSIDCApp corrida 1 falló: %v", err)
	}

	target := filepath.Join(dir, SIDCExeName)
	readCS, err := ReadExeConnString(target)
	if err != nil {
		t.Fatalf("ReadExeConnString falló: %v", err)
	}
	if !strings.Contains(readCS, "UID=sidc;") {
		t.Errorf("ReadExeConnString no contiene UID=sidc: %s", readCS)
	}

	// Corrida 2: idempotente, no reescribe
	logs = nil
	if err := PatchSIDCApp(cfg, "dwHmNy+rx+Ehu#q%EwW*vBrp", emit); err != nil {
		t.Fatalf("PatchSIDCApp corrida 2 falló: %v", err)
	}
	foundAlineado := false
	for _, l := range logs {
		if strings.Contains(l, "alineado") {
			foundAlineado = true
			break
		}
	}
	if !foundAlineado {
		t.Errorf("corrida 2 debió reportar idempotencia (alineado): %v", logs)
	}
}

