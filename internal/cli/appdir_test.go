// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"aegis-setup/internal/config"
	"aegis-setup/internal/ui"

	tea "charm.land/bubbletea/v2"
)

func init() {
	authManagerFactory = nil
}

// teclaTUI arma la pulsación como la manda Bubble Tea.
func teclaTUI(s string) tea.KeyPressMsg {
	r := []rune(s)
	return tea.KeyPressMsg(tea.Key{Text: s, Code: r[0]})
}

// pulsarTUI devuelve el modelo después de una tecla (el cmd se descarta: acá se mira la
// pantalla y el disco, no el trabajo de fondo).
func pulsarTUI(m ui.Model, s string) ui.Model {
	out, _ := m.Update(teclaTUI(s))
	got, ok := out.(ui.Model)
	if !ok {
		panic("el Update del TUI dejó de devolver ui.Model")
	}
	return got
}

// El --app-dir de la línea de comandos tiene que llegar hasta el config.json: es el
// camino desatendido, donde no hay nadie para ver la pantalla ni para responder. Si se
// quedara en la bandera, el TUI preguntaría igual y el script se trabaría esperando.
func TestElFlagAppDirLlegaAlConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	m := pulsarTUI(modeloInicial(config.Default(), path, opcionesTUI{appDir: `E:\SIDC`}), "2")

	guardada, err := config.Load(path)
	if err != nil {
		t.Fatalf("no quedó config escrita: %v", err)
	}
	if guardada.AppDir != `E:\SIDC` {
		t.Errorf("disco app_dir = %q, quiero E:\\SIDC (el flag tiene que viajar al modelo)", guardada.AppDir)
	}
	if guardada.Env != "prod" {
		t.Errorf("env = %q, quiero prod (el perfil de la tecla 2)", guardada.Env)
	}
	if m.View().Content == "" {
		t.Error("el TUI quedó sin pantalla después de aplicar el perfil")
	}
}

// Sin --app-dir el CLI no inventa carpeta: el TUI la pregunta. Es la PC limpia, donde
// nadie sabe todavía dónde va a quedar SIDC y un default silencioso haría medir el repo
// del desarrollador (o una carpeta que no existe en esa máquina) y reportar OK.
func TestSinFlagAppDirElTUILaPregunta(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	m := pulsarTUI(modeloInicial(config.Default(), path, opcionesTUI{}), "2")

	if c := m.View().Content; !strings.Contains(c, "CARPETA DE SIDC") {
		t.Errorf("no pidió la carpeta de SIDC; pantalla:\n%s", c)
	}
	if existeArchivo(path) {
		t.Error("escribió config sin saber dónde está SIDC")
	}
}

// El config que el CLI carga ya puede traer app_dir (lo escribió un operador antes o un
// configure --app-dir). Aun así la carpeta se confirma: el campo viene con ese valor, así
// que Enter la conserva y el operador ve qué carpeta se va a medir sin abrir el JSON.
// Confirmar y preguntar de nuevo es una tecla; una carpeta equivocada pegada para siempre
// no se arregla desde ningún lado.
func TestConAppDirEnElConfigElCampoVienePrecargado(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Default()
	cfg.AppDir = `D:\SIDC`
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}

	// La config existe: no es primera vez, así que se entra por el menú (Avanzada -> preset dev).
	m := pulsarTUI(modeloInicial(cfg, path, opcionesTUI{}), "4")
	m = pulsarTUI(m, "2")

	if c := m.View().Content; !strings.Contains(c, `D:\SIDC`) {
		t.Errorf("el prompt no muestra la carpeta que ya estaba configurada; pantalla:\n%s", c)
	}
	m = pulsarTUI(m, "enter")
	guardada, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if guardada.AppDir != `D:\SIDC` {
		t.Errorf("disco app_dir = %q, quiero conservar D:\\SIDC", guardada.AppDir)
	}
}

// configure --app-dir deja la carpeta escrita sin pasar por el TUI: es el camino de un
// script que prepara la PC (o de un operador que arregla una carpeta mal puesta antes de
// abrir el menú).
func TestConfigureGuardaAppDir(t *testing.T) {
	out := filepath.Join(t.TempDir(), "config.json")
	cmd := newConfigureCmd("")
	cmd.SetOut(&strings.Builder{})
	cmd.SetArgs([]string{"--env", "prod", "--db-mode", "local", "--app-dir", `C:\SIDC`, "--out", out})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("configure: %v", err)
	}

	cfg, err := config.Load(out)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppDir != `C:\SIDC` {
		t.Errorf("app_dir = %q, quiero C:\\SIDC", cfg.AppDir)
	}
	if cfg.Env != "prod" || cfg.DbMode != config.DbLocal || !cfg.UseWinAuth {
		t.Errorf("cfg = %s/%s win=%v, quiero prod/local con Windows Auth", cfg.Env, cfg.DbMode, cfg.UseWinAuth)
	}
}
