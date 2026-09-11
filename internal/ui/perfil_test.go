// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aegis-setup/internal/config"
	"aegis-setup/internal/precheck"
)

// hayArchivo mira el disco: estos tests verifican que NO se escribió un config, y
// eso solo se puede saber mirando.
func hayArchivo(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// nuevoEnPrimeraVez arma el TUI como en una PC sin config: sin perfil elegido
// todavía y con un config.json que no existe.
func nuevoEnPrimeraVez(t *testing.T, cfg config.Config) (Model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	return NewModel(cfg, path).SetPrimeraVez(true), path
}

// La primera pregunta en una PC sin config no es "¿qué querés hacer?" sino "¿en qué
// PC estamos?". Antes el TUI abría el menú con el perfil dev/Docker por defecto y el
// operador de FEMUCARIBE veía "Docker en marcha" en rojo antes de entender que eso
// no aplica a su máquina.
func TestPrimeraVezPideElPerfilAntesQueElMenu(t *testing.T) {
	m, _ := nuevoEnPrimeraVez(t, devCfg())
	if m.screen != screenPerfil {
		t.Fatalf("pantalla = %v, quiero screenPerfil", m.screen)
	}
	if len(perfiles) != 3 {
		t.Errorf("perfiles = %d, quiero 3 (pruebas, prod local, prod server)", len(perfiles))
	}
}

// Mientras no se eligió perfil no hay nada que medir: correr el checklist del
// perfil por defecto (dev/Docker) es medir una máquina que no es la de destino.
func TestPrimeraVezNoMideElPerfilPorDefecto(t *testing.T) {
	m, _ := nuevoEnPrimeraVez(t, devCfg())
	if m.verificando {
		t.Error("arrancó midiendo antes de saber qué PC es")
	}
	if m.checks != nil {
		t.Errorf("tenía checklist del perfil por defecto: %v", m.checks)
	}
	if cmd := m.Init(); cmd != nil {
		t.Error("Init corrió las sondas antes de elegir perfil")
	}
}

// Con config en disco no se pregunta nada: el perfil ya está elegido.
func TestConPerfilElegidoNoSePregunta(t *testing.T) {
	m := NewModel(devCfg(), "")
	if m.screen != screenMenu {
		t.Fatalf("pantalla = %v, quiero screenMenu", m.screen)
	}
	if m.primeraVez {
		t.Error("se marcó primera vez con un perfil ya cargado")
	}
}

// Elegir "Pruebas" tiene que escribir el config, no solo cambiar la pantalla: el
// operador que cierra el TUI y vuelve no quiere que le pregunten de nuevo.
func TestPerfilPruebasGuardaLaConfig(t *testing.T) {
	m, path := nuevoEnPrimeraVez(t, config.Default())
	m = pulsar(m, "1")

	if m.taskErr != nil {
		t.Fatalf("error inesperado: %v", m.taskErr)
	}
	guardada, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if guardada.Env != "dev" || guardada.DbMode != config.DbDocker {
		t.Errorf("disco = %s/%s, quiero dev/docker", guardada.Env, guardada.DbMode)
	}
	if guardada.Server != "localhost,14333" {
		t.Errorf("server = %q, quiero localhost,14333", guardada.Server)
	}
	if m.cfg.Env != "dev" || m.cfg.DbMode != config.DbDocker {
		t.Errorf("cfg en memoria = %s/%s, quiero dev/docker", m.cfg.Env, m.cfg.DbMode)
	}
	if m.primeraVez {
		t.Error("siguió en primera vez después de elegir")
	}
}

func TestPerfilProdLocalGuardaLaConfig(t *testing.T) {
	m, path := nuevoEnPrimeraVez(t, config.Default())
	m = pulsar(m, "2")

	guardada, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if guardada.Env != "prod" || guardada.DbMode != config.DbLocal || guardada.Server != "localhost" {
		t.Errorf("disco = %s/%s/%s, quiero prod/local/localhost", guardada.Env, guardada.DbMode, guardada.Server)
	}
	// Windows Auth es lo que hace que en prod no se guarden ni se pidan claves.
	if !guardada.UseWinAuth || guardada.Driver != "SQL Server" {
		t.Errorf("auth/driver = %v/%s, quiero Windows Auth + SQL Server", guardada.UseWinAuth, guardada.Driver)
	}
}

// "Producción en un servidor" no puede escribir un nombre inventado: guardarlo
// manda al operador a un "motor no alcanzable en CONTABILIDAD" que no tiene nada
// que ver con su problema. Se pregunta.
func TestPerfilProdServerPideElNombre(t *testing.T) {
	m, path := nuevoEnPrimeraVez(t, config.Default())
	m = pulsar(m, "3")

	if m.screen != screenServer {
		t.Fatalf("pantalla = %v, quiero screenServer (falta el nombre del servidor)", m.screen)
	}
	if m.srvInput.Placeholder == "" {
		t.Error("el campo no sugiere ningún nombre: el operador no sabe si tipea PC, IP o instancia")
	}

	m = escribir(m, "SIDC01")
	if m.taskErr != nil {
		t.Fatalf("error inesperado: %v", m.taskErr)
	}
	guardada, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if guardada.Server != "SIDC01" {
		t.Errorf("server = %q, quiero SIDC01", guardada.Server)
	}
	if guardada.DbMode != config.DbServer {
		t.Errorf("db_mode = %q, quiero server", guardada.DbMode)
	}
}

// Nombre vacío es un config que no arranca: se dice en la pantalla, no se escribe.
func TestPerfilProdServerNoAceptaVacio(t *testing.T) {
	m, path := nuevoEnPrimeraVez(t, config.Default())
	m = pulsar(m, "3")
	m.srvInput.SetValue("")
	m = pulsar(m, "enter")

	if m.screen != screenServer {
		t.Fatalf("pantalla = %v, se fue con el nombre vacío", m.screen)
	}
	if m.srvErr == "" {
		t.Error("no explicó por qué no acepta vacío")
	}
	if _, err := config.Load(path); err != nil {
		t.Fatal(err)
	}
	if hayArchivo(path) {
		t.Error("escribió un config con server vacío")
	}
}

// Con --server no se pregunta: es el camino no interactivo (scripts, instalación
// desatendida) y ahí nadie puede tipear.
func TestPerfilProdServerConFlagNoPregunta(t *testing.T) {
	m, path := nuevoEnPrimeraVez(t, config.Default())
	m = m.SetPresetServer("MI_SERVIDOR")
	m = pulsar(m, "3")

	if m.screen == screenServer {
		t.Fatal("preguntó el servidor teniendo --server")
	}
	guardada, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if guardada.Server != "MI_SERVIDOR" {
		t.Errorf("server = %q, quiero MI_SERVIDOR", guardada.Server)
	}
}

// Elegir perfil cambia el ambiente que se va a instalar: el checklist viejo, medido
// contra el perfil anterior, ya no describe esta máquina. Si no se limpia, el
// operador queda trabado por requisitos de un ambiente que acaba de abandonar.
func TestElegirPerfilDescartaElChecklistViejo(t *testing.T) {
	m, _ := nuevoEnPrimeraVez(t, config.Default())
	m.checks = []precheck.Requisito{}

	out, cmd := m.Update(tecla("2"))
	m = out.(Model)
	if cmd == nil {
		t.Fatal("no pidió recalcular el checklist")
	}
	if !m.verificando {
		t.Error("no quedó en 'verificando': la pantalla mentiría con el resultado viejo")
	}
	if m.checks != nil {
		t.Errorf("conservó el checklist del perfil anterior: %v", m.checks)
	}
}

// El mismo agujero existía en los presets del menú: guardaban la config nueva y
// dejaban el checklist del ambiente viejo.
func TestCambiarDePerfilDesdeElMenuTambienRecalcula(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := NewModel(prodCfg(), path).SetPrimeraVez(false)
	m.checks = []precheck.Requisito{}
	m.verificando = false

	out, cmd := m.Update(tecla("4")) // preset dev
	m = out.(Model)
	if cmd == nil {
		t.Fatal("el preset no pidió recalcular el checklist")
	}
	if !m.verificando || m.checks != nil {
		t.Errorf("verificando=%v checks=%d, quiero recalcular desde cero", m.verificando, len(m.checks))
	}
	if m.cfg.Env != "dev" || m.cfg.DbMode != config.DbDocker {
		t.Errorf("cfg = %s/%s, quiero dev/docker", m.cfg.Env, m.cfg.DbMode)
	}
}

// Elegir el perfil y usar el preset tienen que dejar la MISMA config: son la misma
// decisión en dos momentos distintos. Si fueran listas separadas, elegir en el
// arranque y elegir en el menú podrían dar resultados distintos.
func TestLosPerfilesSonLosPresetsDelMenu(t *testing.T) {
	delMenu := map[action]bool{}
	for _, it := range menuItems {
		delMenu[it.action] = true
	}
	quiero := map[action]bool{actPresetDev: true, actPresetProdLocal: true, actPresetProdServer: true}

	vistas := map[action]bool{}
	for _, p := range perfiles {
		if !delMenu[p.action] {
			t.Errorf("el perfil %q usa la acción %v, que no está en el menú: elegir en el "+
				"arranque y elegir en el menú dejarían configs distintas", p.name, p.action)
		}
		if !quiero[p.action] {
			t.Errorf("acción inesperada en perfiles: %v", p.action)
		}
		if vistas[p.action] {
			t.Errorf("acción repetida en perfiles: %v", p.action)
		}
		vistas[p.action] = true
	}
	if len(vistas) != 3 {
		t.Errorf("perfiles cubre %d acciones, quiero las 3 de ambiente", len(vistas))
	}
	for _, p := range perfiles {
		if p.name == "" || p.desc == "" || p.key == "" {
			t.Errorf("perfil incompleto: %+v", p)
		}
	}
}

// Después de elegir, el operador tiene que saber dónde quedó el config y qué sigue.
func TestAvisaDondeQuedoElConfig(t *testing.T) {
	m, path := nuevoEnPrimeraVez(t, config.Default())
	m = pulsar(m, "2")

	if m.screen != screenDone {
		t.Fatalf("pantalla = %v, quiero la confirmación", m.screen)
	}
	texto := strings.Join(m.lines, "\n")
	if !strings.Contains(texto, path) {
		t.Errorf("no dice dónde quedó el config (%s):\n%s", path, texto)
	}
}

// Esc en la primera pantalla sale sin escribir nada. Lo que NO puede pasar es que
// siga al menú con el perfil dev por defecto: ese es el bug que F4 vino a arreglar.
func TestEscEnElPerfilNoArrancaConElDefault(t *testing.T) {
	m, path := nuevoEnPrimeraVez(t, config.Default())
	m = pulsar(m, "esc")

	if m.screen == screenMenu {
		t.Fatal("esc llevó al menú con el perfil por defecto sin elegir nada")
	}
	if hayArchivo(path) {
		t.Error("escribió un config sin que el operador eligiera")
	}
}

// Las teclas de los perfiles son las que dice la pantalla y no se pisan entre sí.
func TestTeclasDeLosPerfiles(t *testing.T) {
	vistas := map[string]bool{}
	for _, p := range perfiles {
		if vistas[p.key] {
			t.Errorf("tecla repetida: %q", p.key)
		}
		vistas[p.key] = true
	}

	// Prod server pregunta el nombre antes de escribir nada: eso es aparte (y está
	// probado arriba).
	m, _ := nuevoEnPrimeraVez(t, config.Default())
	out, _ := m.Update(tecla("3"))
	if got := out.(Model); got.screen != screenServer {
		t.Errorf("tecla 3 -> pantalla %v, quiero screenServer", got.screen)
	}
}

// Cada tecla aplica SU perfil: si alguien reordena la lista o copia mal una acción,
// la pantalla diría una cosa y el config otra.
func TestTeclasDeLosPerfilesAplicanSuPerfil(t *testing.T) {
	for _, p := range perfiles {
		if p.action == actPresetProdServer {
			continue // pregunta el nombre; el mapeo se prueba con el flag --server
		}
		quiero, err := presetConfig(config.Default(), p.action, "")
		if err != nil {
			t.Fatal(err)
		}
		m, _ := nuevoEnPrimeraVez(t, config.Default())
		out, _ := m.Update(tecla(p.key))
		got := out.(Model)
		if got.cfg.Env != quiero.Env || got.cfg.DbMode != quiero.DbMode || got.cfg.Server != quiero.Server {
			t.Errorf("tecla %q -> %s/%s/%s, quiero %s/%s/%s", p.key,
				got.cfg.Env, got.cfg.DbMode, got.cfg.Server, quiero.Env, quiero.DbMode, quiero.Server)
		}
		if got.primeraVez {
			t.Errorf("tecla %q dejó la primera vez marcada", p.key)
		}
	}

	m, _ := nuevoEnPrimeraVez(t, config.Default())
	m = m.SetPresetServer("MI_SERVIDOR")
	out, _ := m.Update(tecla("3"))
	if got := out.(Model); got.cfg.Server != "MI_SERVIDOR" {
		t.Errorf("tecla 3 con --server -> %q, quiero MI_SERVIDOR", got.cfg.Server)
	}
}

// Si guardar el config falla (por ejemplo %APPDATA% sin permiso), volver al menú
// sería arrancar con el ambiente por defecto que nadie eligió: se vuelve a preguntar.
func TestSiFallaElGuardadoNoSigueAlMenu(t *testing.T) {
	dir := t.TempDir()
	// Un directorio donde no se puede escribir un archivo: el "config.json" es un
	// directorio, así que WriteFile falla.
	imposible := filepath.Join(dir, "config.json")
	if err := os.Mkdir(imposible, 0o755); err != nil {
		t.Fatal(err)
	}
	m := NewModel(devCfg(), imposible).SetPrimeraVez(true)
	m = pulsar(m, "2")

	if m.screen != screenDone || m.taskErr == nil {
		t.Fatalf("pantalla=%v err=%v, quiero la falla a la vista", m.screen, m.taskErr)
	}
	m = pulsar(m, "esc")
	if m.screen != screenPerfil {
		t.Fatalf("pantalla = %v, quiero volver a preguntar el perfil", m.screen)
	}
}
