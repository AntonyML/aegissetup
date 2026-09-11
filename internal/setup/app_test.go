// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"os"
	"path/filepath"
	"testing"
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
