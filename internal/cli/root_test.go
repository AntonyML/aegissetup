// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aegis-setup/internal/config"
	"aegis-setup/internal/setup"
)

// TestConfigPathToUseOrden fija la migración del config.json: el flag manda,
// después el canónico de %APPDATA%\AegisSetup y, si todavía no existe, el viejo
// junto al binario. Un config nuevo se escribe siempre en el canónico.
//
// Cada subtest arma su propio %APPDATA%: si compartieran el temporal, el
// canónico escrito en uno contaminaría al siguiente.
func TestConfigPathToUseOrden(t *testing.T) {
	escribir := func(t *testing.T, p string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	canonico := func(appData string) string {
		return filepath.Join(appData, "AegisSetup", "config.json")
	}

	t.Run("el flag manda sobre todo", func(t *testing.T) {
		appData := t.TempDir()
		t.Setenv("AEGIS_APPDATA", appData)
		exeDir := t.TempDir()

		escribir(t, canonico(appData))
		escribir(t, filepath.Join(exeDir, "config.json"))

		quiero := filepath.Join(t.TempDir(), "otro.json")
		if got := configPathToUse(exeDir, quiero); got != quiero {
			t.Errorf("got %q, quiero el flag %q", got, quiero)
		}
	})

	t.Run("sólo legado existe: se sigue usando", func(t *testing.T) {
		t.Setenv("AEGIS_APPDATA", t.TempDir())
		exeDir := t.TempDir()
		legado := filepath.Join(exeDir, "config.json")
		escribir(t, legado)

		if got := configPathToUse(exeDir, ""); got != legado {
			t.Errorf("got %q, quiero el legado %q", got, legado)
		}
	})

	t.Run("ninguno existe: se apunta al canónico", func(t *testing.T) {
		appData := t.TempDir()
		t.Setenv("AEGIS_APPDATA", appData)

		if got := configPathToUse(t.TempDir(), ""); got != canonico(appData) {
			t.Errorf("got %q, quiero el canónico %q", got, canonico(appData))
		}
	})

	t.Run("ambos existen: gana el canónico", func(t *testing.T) {
		appData := t.TempDir()
		t.Setenv("AEGIS_APPDATA", appData)
		exeDir := t.TempDir()

		escribir(t, canonico(appData))
		escribir(t, filepath.Join(exeDir, "config.json"))

		if got := configPathToUse(exeDir, ""); got != canonico(appData) {
			t.Errorf("got %q, quiero el canónico %q", got, canonico(appData))
		}
	})
}

// TestResolveCfgCargaElPathElegido verifica que resolveCfg realmente le pasa al
// loader el path resuelto (y no otro), que es donde se rompería en silencio.
func TestResolveCfgCargaElPathElegido(t *testing.T) {
	t.Setenv("AEGIS_APPDATA", t.TempDir())

	var visto string
	loader := func(p string) (config.Config, error) {
		visto = p
		return config.Default(), nil
	}

	_, path, err := resolveCfg(t.TempDir(), "", loader)
	if err != nil {
		t.Fatalf("resolveCfg: %v", err)
	}
	if visto != path {
		t.Errorf("el loader recibió %q pero resolveCfg devolvió %q", visto, path)
	}
}

// El perfil se pregunta solo cuando no hay config. Con config ya está dicho en qué
// PC estamos y volver a preguntar (o peor: reescribirla) sería perder lo que el
// operador eligió.
func TestPrimeraVezSoloSinConfig(t *testing.T) {
	dir := t.TempDir()
	falta := filepath.Join(dir, "config.json")
	if !primeraVez(falta) {
		t.Error("sin config en disco no dijo primera vez: el TUI abriría el menú con el perfil dev por defecto")
	}

	if err := os.WriteFile(falta, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if primeraVez(falta) {
		t.Error("con config en disco volvió a decir primera vez")
	}
}

// stubAdmin deja el flujo de elevación bajo control del test.
func stubAdmin(t *testing.T, admin, skip bool, code int, err error) *[]string {
	t.Helper()
	prevAdmin, prevSkip, prevElev := isAdmin, skipElevation, elevate
	t.Cleanup(func() { isAdmin, skipElevation, elevate = prevAdmin, prevSkip, prevElev })

	llamadas := &[]string{}
	isAdmin = func() bool { return admin }
	skipElevation = func() bool { return skip }
	elevate = func(args []string) (int, error) {
		*llamadas = append(*llamadas, strings.Join(args, " "))
		return code, err
	}
	return llamadas
}

// La lista de quién pide elevación es un dato, no una condición suelta en cada
// comando. Los tres de diagnóstico tienen que quedar afuera: check, checklist y
// dashboard son lo que el operador corre justamente cuando algo está mal, y una
// máquina rota suele ser una donde no hay forma de aceptar un UAC (sesión remota,
// otro usuario, un script).
func TestSoloLosQueEscribenPidenElevacion(t *testing.T) {
	escriben := map[string]bool{"setup-db": true, "setup-app": true}
	for _, sub := range []string{"setup-db", "setup-app", "check", "checklist", "dashboard", "menu", "configure"} {
		if got, want := writesToSystem(sub), escriben[sub]; got != want {
			t.Errorf("writesToSystem(%q) = %v, quiero %v", sub, got, want)
		}
	}
}

// El padre no sigue: el trabajo lo hace el proceso elevado y su código de salida es
// el nuestro. Si el padre siguiera, dos Aegis escribirían la misma base a la vez.
func TestAsegurarAdminEsperaYPropagaElCodigo(t *testing.T) {
	llamadas := stubAdmin(t, false, false, 3, nil)
	var salida strings.Builder

	hecho, code, err := asegurarAdmin(&salida, "setup-app", []string{"aegis.exe", "setup-app"})
	if err != nil {
		t.Fatalf("asegurarAdmin: %v", err)
	}
	if !hecho {
		t.Error("no dijo que ya está hecho: el padre seguiría corriendo el trabajo")
	}
	if code != 3 {
		t.Errorf("código = %d, quiero el del proceso elevado (3)", code)
	}
	if len(*llamadas) != 1 || (*llamadas)[0] != "aegis.exe setup-app" {
		t.Errorf("relanzó con %v, quiero el mismo argv (setup.Elevate saca el programa)", *llamadas)
	}
	if !strings.Contains(salida.String(), "3") {
		t.Errorf("salida = %q, quiero que diga con qué código terminó", salida.String())
	}
}

// El aviso va antes del UAC: un cartel de permisos que aparece de la nada, sin
// explicación en la consola, se lee como si el programa estuviera haciendo algo raro.
func TestAsegurarAdminAvisaAntesDelUac(t *testing.T) {
	stubAdmin(t, false, false, 0, nil)
	var salida strings.Builder

	if _, _, err := asegurarAdmin(&salida, "setup-db", []string{"aegis.exe", "setup-db"}); err != nil {
		t.Fatalf("asegurarAdmin: %v", err)
	}
	if !strings.Contains(salida.String(), "administrador") {
		t.Errorf("salida = %q, quiero el aviso de permisos", salida.String())
	}
}

func TestAsegurarAdminNoRelanzaSiNoHaceFalta(t *testing.T) {
	casos := []struct {
		sub         string
		admin, skip bool
		por         string
	}{
		{"setup-db", true, false, "ya está elevado"},
		{"setup-db", false, true, "el padre elevado ya lo intentó"},
		{"checklist", false, false, "solo lee: no pide permisos nunca"},
		{"check", false, false, "solo lee"},
		{"dashboard", false, false, "solo lee"},
	}
	for _, c := range casos {
		llamadas := stubAdmin(t, c.admin, c.skip, 0, nil)
		var salida strings.Builder

		hecho, code, err := asegurarAdmin(&salida, c.sub, []string{"aegis.exe", c.sub})
		if hecho || code != 0 || err != nil {
			t.Errorf("%s: (%v, %d, %v), quiero (false, 0, nil) — %s", c.sub, hecho, code, err, c.por)
		}
		if len(*llamadas) != 0 {
			t.Errorf("%s: relanzó (%v) — %s", c.sub, *llamadas, c.por)
		}
	}
}

// Cancelar el UAC tiene que leerse como cancelación, no como un fallo del instalador.
func TestAsegurarAdminDiceSiSeCancela(t *testing.T) {
	stubAdmin(t, false, false, 0, errors.New("pedido de permisos cancelado"))
	var salida strings.Builder

	hecho, _, err := asegurarAdmin(&salida, "setup-app", []string{"aegis.exe", "setup-app"})
	if err == nil {
		t.Fatal("no reportó la cancelación")
	}
	if hecho {
		t.Error("dijo que estaba hecho aunque no se elevó")
	}
}

// Qué subcomando se pidió, sin ejecutar nada. Se usa el buscador de cobra para no
// reimplementar el parseo de flags (--config toma valor, --help no).
func TestSubcomandoDe(t *testing.T) {
	loader := func(string) (config.Config, error) { return config.Default(), nil }
	root := NewRootCmd("", loader)

	casos := []struct {
		args []string
		want string
	}{
		{[]string{"setup-db"}, "setup-db"},
		{[]string{"setup-db", "--bak", "x.bak"}, "setup-db"},
		{[]string{"setup-app", "--config", "c.json"}, "setup-app"},
		{[]string{"checklist"}, "checklist"},
		{[]string{"--config", "c.json", "setup-app"}, "setup-app"},
		{[]string{"noexiste"}, ""},
		{nil, "aegis"},
	}
	for _, c := range casos {
		if got := subcomandoDe(root, c.args); got != c.want {
			t.Errorf("subcomandoDe(%v) = %q, quiero %q", c.args, got, c.want)
		}
	}
}

// La ruta del config no se pierde al relanzar: si el operador apuntó a un config
// con --config, el proceso elevado tiene que usar ese mismo.
func TestElevatedArgsMantieneLasBanderas(t *testing.T) {
	got := setup.ElevatedArgs([]string{"aegis.exe", "setup-app", "--config", `C:\Program Files\c.json`})
	// quoteArgs se prueba en setup; acá lo que importa es que los args viajen.
	if len(got) != 3 || got[0] != "setup-app" || got[2] != `C:\Program Files\c.json` {
		t.Errorf("args = %v, quiero el subcomando y su bandera", got)
	}
}
