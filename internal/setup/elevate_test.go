// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"unsafe"
)

// Relanzar el proceso con los mismos argumentos es lo que hace que la elevación no
// cambie el trabajo: el proceso elevado recibe exactamente lo mismo, sin el nombre
// del programa (que ahí es el ejecutable, no un argumento).
func TestElevatedArgsSacaElPrograma(t *testing.T) {
	casos := []struct {
		in   []string
		want []string
	}{
		{[]string{"aegis.exe", "setup-db"}, []string{"setup-db"}},
		{[]string{"aegis.exe", "setup-app", "--config", "x.json"}, []string{"setup-app", "--config", "x.json"}},
		{[]string{"aegis.exe"}, nil},
		{nil, nil},
	}
	for _, c := range casos {
		got := ElevatedArgs(c.in)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("ElevatedArgs(%v) = %v, quiero %v", c.in, got, c.want)
		}
	}
}

// La copia no puede compartir el arreglo con os.Args: el llamador podría reusarlo.
func TestElevatedArgsCopia(t *testing.T) {
	orig := []string{"aegis.exe", "setup-db"}
	got := ElevatedArgs(orig)
	if len(got) == 0 {
		t.Fatal("no devolvió argumentos")
	}
	got[0] = "otro"
	if orig[1] != "setup-db" {
		t.Errorf("escribir en el resultado tocó el original: %v", orig)
	}
}

// Sin esta marca, un proceso elevado que no se vea elevado reintentaría para
// siempre: el operador vería una sucesión de carteles de UAC.
func TestSkipElevationPorVariableDeEntorno(t *testing.T) {
	t.Setenv(envNoElevar, "")
	if SkipElevation() {
		t.Error("sin la variable dijo que no hay que elevar")
	}
	t.Setenv(envNoElevar, "1")
	if !SkipElevation() {
		t.Error("con la variable en 1 siguió queriendo elevar")
	}
}

// La tabla que decide si hay que relanzar. Son dos banderas y una sola respuesta:
// tenerla en una función es lo que permite probarla sin un UAC de por medio.
func TestNeedsElevation(t *testing.T) {
	casos := []struct {
		admin, skip, want bool
		por               string
	}{
		{false, false, true, "es el caso normal: sin permisos y sin marca"},
		{true, false, false, "ya está elevado: no hay nada que pedir"},
		{false, true, false, "el padre elevado ya lo intentó: no se reintenta"},
		{true, true, false, "elevado y marcado: tampoco"},
	}
	for _, c := range casos {
		if got := NeedsElevation(c.admin, c.skip); got != c.want {
			t.Errorf("NeedsElevation(%v, %v) = %v, quiero %v — %s", c.admin, c.skip, got, c.want, c.por)
		}
	}
}

// La línea de parámetros la parsea CommandLineToArgvW: si un argumento tiene
// espacios y no se comilla, el proceso elevado recibe dos argumentos distintos.
// Con --config en "C:\Program Files\..." eso es un config que no se encuentra.
func TestQuoteArgs(t *testing.T) {
	casos := []struct {
		in   []string
		want string
		por  string
	}{
		{[]string{"setup-db"}, "setup-db", "sin espacios va tal cual"},
		{nil, "", "sin argumentos no hay línea"},
		{[]string{"a b", "c"}, `"a b" c`, "el que tiene espacios se comilla"},
		{[]string{"--config", `C:\Program Files\Aegis\config.json`},
			`--config "C:\Program Files\Aegis\config.json"`, "ruta con espacios"},
		{[]string{`a"b`}, `"a\"b"`, "comilla embebida: se escapa"},
		{[]string{`a b\`}, `"a b\\"`, "barra final: se duplica para que no escape la comilla de cierre"},
		{[]string{""}, `""`, "argumento vacío: se comilla para que no desaparezca"},
		{[]string{`--bak=a.bak`, `C:\d\e.bak`}, `--bak=a.bak C:\d\e.bak`, "sin espacios, sin comillas"},
	}
	for _, c := range casos {
		if got := quoteArgs(c.in); got != c.want {
			t.Errorf("quoteArgs(%q) = %q, quiero %q — %s", c.in, got, c.want, c.por)
		}
	}
}

// SHELLEXECUTEINFOW en x64 mide 112 bytes y sus campos van en este orden exacto.
//
// Es el único riesgo real de esta parte y no se puede probar abriendo un cartel de
// UAC: un ShellExecuteExW con cbSize mal armado no devuelve "estructura inválida",
// devuelve un código de error de shell genérico que manda a buscar el problema al
// lugar equivocado. Los desplazamientos los fija el compilador; esto los contrasta
// con el layout documentado (incluido el relleno después de nShow y de dwHotKey).
func TestShellExecuteInfoLayout(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("el layout documentado acá es el de 64 bits")
	}
	var s shellExecuteInfo
	casos := []struct {
		campo string
		off   uintptr
	}{
		{"cbSize", unsafe.Offsetof(s.cbSize)},
		{"fMask", unsafe.Offsetof(s.fMask)},
		{"hwnd", unsafe.Offsetof(s.hwnd)},
		{"lpVerb", unsafe.Offsetof(s.lpVerb)},
		{"lpFile", unsafe.Offsetof(s.lpFile)},
		{"lpParameters", unsafe.Offsetof(s.lpParameters)},
		{"lpDirectory", unsafe.Offsetof(s.lpDirectory)},
		{"nShow", unsafe.Offsetof(s.nShow)},
		{"hInstApp", unsafe.Offsetof(s.hInstApp)},
		{"lpIDList", unsafe.Offsetof(s.lpIDList)},
		{"lpClass", unsafe.Offsetof(s.lpClass)},
		{"hkeyClass", unsafe.Offsetof(s.hkeyClass)},
		{"dwHotKey", unsafe.Offsetof(s.dwHotKey)},
		{"hIcon", unsafe.Offsetof(s.hIcon)},
		{"hProcess", unsafe.Offsetof(s.hProcess)},
	}
	quiero := map[string]uintptr{
		"cbSize": 0, "fMask": 4, "hwnd": 8, "lpVerb": 16, "lpFile": 24,
		"lpParameters": 32, "lpDirectory": 40, "nShow": 48, "hInstApp": 56,
		"lpIDList": 64, "lpClass": 72, "hkeyClass": 80, "dwHotKey": 88,
		"hIcon": 96, "hProcess": 104,
	}
	if got := unsafe.Sizeof(shellExecuteInfo{}); got != 112 {
		t.Errorf("tamaño = %d, quiero 112", got)
	}
	for _, c := range casos {
		if c.off != quiero[c.campo] {
			t.Errorf("%s en %d, quiero %d (layout de SHELLEXECUTEINFOW x64)", c.campo, c.off, quiero[c.campo])
		}
	}
}

// Fuera de Windows no hay permisos que pedir: elevarse ahí no aplica y no puede
// quedarse intentando lanzar algo que no existe.
func TestLaunchElevatedFueraDeWindowsNoHaceNada(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("en Windows esto sí lanza el proceso")
	}
	code, err := launchElevated(os.Args[0], []string{"check"}, true)
	if err != nil || code != 0 {
		t.Errorf("launchElevated fuera de Windows = (%d, %v), quiero (0, nil)", code, err)
	}
}
