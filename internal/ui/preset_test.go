// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"path/filepath"
	"testing"

	"aegis-setup/internal/config"
)

// El flag --server del CLI solo cambia el preset "prod server": deja elegir el
// servidor sin editar config.json a mano.
func TestPresetProdServerConFlagUsaEseServer(t *testing.T) {
	cfg, err := presetConfig(devCfg(), actPresetProdServer, "MI_SERVIDOR", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server != "MI_SERVIDOR" {
		t.Errorf("server = %q, quiero MI_SERVIDOR", cfg.Server)
	}
	if cfg.Env != "prod" || cfg.DbMode != config.DbServer {
		t.Errorf("env/db_mode = %s/%s, quiero prod/server", cfg.Env, cfg.DbMode)
	}
	if !cfg.UseWinAuth || cfg.Driver != "SQL Server" {
		t.Errorf("auth/driver = %v/%s, quiero Windows Auth + SQL Server", cfg.UseWinAuth, cfg.Driver)
	}
}

// Sin flag, el preset "prod server" conserva el default histórico: si la config
// viene de dev (localhost), salta a CONTABILIDAD; si ya trae un server remoto,
// lo respeta.
func TestPresetProdServerSinFlag(t *testing.T) {
	cfg, err := presetConfig(devCfg(), actPresetProdServer, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server != "CONTABILIDAD" {
		t.Errorf("desde dev: server = %q, quiero CONTABILIDAD", cfg.Server)
	}

	remota := prodCfg()
	remota.DbMode = config.DbServer
	remota.Server = "OTRO_SERVIDOR"
	cfg2, err := presetConfig(remota, actPresetProdServer, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Server != "OTRO_SERVIDOR" {
		t.Errorf("server remoto previo: server = %q, quiero OTRO_SERVIDOR", cfg2.Server)
	}
}

// Los otros presets no se tocan por el flag: dev y prod local siguen fijos.
func TestPresetDevYProdLocalInmutables(t *testing.T) {
	dev, err := presetConfig(prodCfg(), actPresetDev, "IGNORADO", "")
	if err != nil {
		t.Fatal(err)
	}
	if dev.Env != "dev" || dev.Server != "localhost,14333" || dev.DbMode != config.DbDocker {
		t.Errorf("dev = %s/%s/%s, quiero dev/localhost,14333/docker", dev.Env, dev.Server, dev.DbMode)
	}
	if dev.UseWinAuth || dev.Driver != "ODBC Driver 17 for SQL Server" {
		t.Errorf("dev auth/driver = %v/%s, quiero SQL Auth + ODBC 17", dev.UseWinAuth, dev.Driver)
	}

	local, err := presetConfig(devCfg(), actPresetProdLocal, "IGNORADO", "")
	if err != nil {
		t.Fatal(err)
	}
	if local.Env != "prod" || local.Server != "localhost" || local.DbMode != config.DbLocal {
		t.Errorf("local = %s/%s/%s, quiero prod/localhost/local", local.Env, local.Server, local.DbMode)
	}
	if !local.UseWinAuth || local.Driver != "SQL Server" {
		t.Errorf("local auth/driver = %v/%s, quiero Windows Auth + SQL Server", local.UseWinAuth, local.Driver)
	}
}

// SetPresetServer deja el valor en el modelo, para que el TUI lo use.
func TestSetPresetServer(t *testing.T) {
	m := NewModel(devCfg(), "").SetPresetServer("MI_SERVIDOR")
	if m.presetServer != "MI_SERVIDOR" {
		t.Errorf("presetServer = %q, quiero MI_SERVIDOR", m.presetServer)
	}
}

// La tecla 6 con --server guarda la config en disco Y actualiza el modelo en
// memoria: una instalación completa lanzada justo después tiene que ver la
// config nueva, no la vieja.
func TestTecla6ConServerGuardaYActualiza(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := NewModel(devCfg(), path).SetPresetServer("MI_SERVIDOR").SetPresetAppDir(`C:\SIDC`)

	m = pulsar(m, "4") // Configuración avanzada
	m = pulsar(m, "4") // Preset prod server
	if m.screen != screenDone {
		t.Fatalf("pantalla = %v, quiero screenDone", m.screen)
	}
	if m.taskErr != nil {
		t.Fatalf("error inesperado: %v", m.taskErr)
	}
	if m.cfg.Server != "MI_SERVIDOR" {
		t.Errorf("cfg en memoria = %q, quiero MI_SERVIDOR", m.cfg.Server)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Server != "MI_SERVIDOR" {
		t.Errorf("config en disco = %q, quiero MI_SERVIDOR", loaded.Server)
	}
	if loaded.Env != "prod" || loaded.DbMode != config.DbServer {
		t.Errorf("disco env/db_mode = %s/%s, quiero prod/server", loaded.Env, loaded.DbMode)
	}
}

// En modo interactivo, el preset prod server pide toda la información:
// Server -> Port -> Database -> Auth -> SQLUser (si SQL Auth) -> AppDir.
func TestPresetProdServerInteractivoConSQLAuth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := NewModel(devCfg(), path)

	// Avanzada -> Preset prod server
	m = pulsar(m, "4")
	m = pulsar(m, "4")
	if m.screen != screenServer {
		t.Fatalf("pantalla = %v, quiero screenServer", m.screen)
	}

	// 1. Servidor
	m = escribir(m, "192.168.1.100")
	if m.screen != screenPort {
		t.Fatalf("pantalla = %v, quiero screenPort", m.screen)
	}

	// 2. Puerto
	m = escribir(m, "14333")
	if m.screen != screenDatabase {
		t.Fatalf("pantalla = %v, quiero screenDatabase", m.screen)
	}

	// 3. Database: custom
	m.dbInput.SetValue("SIDC_REMOTO")
	m = pulsar(m, "enter")
	if m.screen != screenAuth {
		t.Fatalf("pantalla = %v, quiero screenAuth", m.screen)
	}

	// 4. Autenticación SQL ("2")
	m = pulsar(m, "2")
	if m.screen != screenSQLUser {
		t.Fatalf("pantalla = %v, quiero screenSQLUser", m.screen)
	}

	// 5. Usuario SQL
	m.userInput.SetValue("operador_sidc")
	m = pulsar(m, "enter")
	if m.screen != screenAppDir {
		t.Fatalf("pantalla = %v, quiero screenAppDir", m.screen)
	}

	// 6. Carpeta SIDC
	m.dirInput.SetValue(`C:\SIDC`)
	m = pulsar(m, "enter")
	if m.screen != screenDone {
		t.Fatalf("pantalla = %v, quiero screenDone", m.screen)
	}

	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("error al leer config: %v", err)
	}
	if saved.Server != "192.168.1.100,14333" {
		t.Errorf("server = %q, quiero 192.168.1.100,14333", saved.Server)
	}
	if saved.Database != "SIDC_REMOTO" {
		t.Errorf("database = %q, quiero SIDC_REMOTO", saved.Database)
	}
	if saved.UseWinAuth {
		t.Errorf("use_win_auth = true, quiero false para SQL Auth")
	}
	if saved.SQLUser != "operador_sidc" {
		t.Errorf("sql_user = %q, quiero operador_sidc", saved.SQLUser)
	}
	if saved.AppDir != `C:\SIDC` {
		t.Errorf("app_dir = %q, quiero C:\\SIDC", saved.AppDir)
	}
}

// Si la información ya está guardada en la PC, el TUI no skipea ningún paso:
// precarga la información en los inputs para que el operador pueda simplemente
// dar Enter para conservarla o editarla.
func TestPresetProdServerConservaDatosGuardadosSinSkipear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfgPrevia := config.Config{
		Env:        "prod",
		DbMode:     config.DbServer,
		Server:     "SERVIDOR_PREVIO,54321",
		Database:   "SIDC_PREVIO",
		DsnName:    "SIDC_SQL",
		Driver:     "SQL Server",
		UseWinAuth: false,
		SQLUser:    "usuario_previo",
		Compat:     120,
		AppDir:     `D:\SIDC_PREVIO`,
	}
	if err := cfgPrevia.Save(path); err != nil {
		t.Fatal(err)
	}

	m := NewModel(cfgPrevia, path)

	// Avanzada -> Preset prod server
	m = pulsar(m, "4")
	m = pulsar(m, "4")

	// 1. Servidor: debe pedirlo y venir precargado con SERVIDOR_PREVIO
	if m.screen != screenServer {
		t.Fatalf("pantalla = %v, quiero screenServer (no debe skipear)", m.screen)
	}
	if m.srvInput.Value() != "SERVIDOR_PREVIO" {
		t.Errorf("srvInput = %q, quiero SERVIDOR_PREVIO precargado", m.srvInput.Value())
	}
	m = pulsar(m, "enter") // Da enter con la misma info

	// 2. Puerto: debe pedirlo y venir precargado con 54321
	if m.screen != screenPort {
		t.Fatalf("pantalla = %v, quiero screenPort", m.screen)
	}
	if m.portInput.Value() != "54321" {
		t.Errorf("portInput = %q, quiero 54321 precargado", m.portInput.Value())
	}
	m = pulsar(m, "enter") // Da enter

	// 3. Database: debe pedirla y venir precargada con SIDC_PREVIO
	if m.screen != screenDatabase {
		t.Fatalf("pantalla = %v, quiero screenDatabase", m.screen)
	}
	if m.dbInput.Value() != "SIDC_PREVIO" {
		t.Errorf("dbInput = %q, quiero SIDC_PREVIO precargado", m.dbInput.Value())
	}
	m = pulsar(m, "enter")

	// 4. Autenticación: cursor posicionado en SQL Auth (índice 1) porque estaba en SQL Auth
	if m.screen != screenAuth {
		t.Fatalf("pantalla = %v, quiero screenAuth", m.screen)
	}
	if m.authCursor != 1 {
		t.Errorf("authCursor = %d, quiero 1 (SQL Auth seleccionado por defecto)", m.authCursor)
	}
	m = pulsar(m, "enter") // Conserva SQL Auth

	// 5. Usuario SQL: debe pedirlo y venir precargado con usuario_previo
	if m.screen != screenSQLUser {
		t.Fatalf("pantalla = %v, quiero screenSQLUser", m.screen)
	}
	if m.userInput.Value() != "usuario_previo" {
		t.Errorf("userInput = %q, quiero usuario_previo precargado", m.userInput.Value())
	}
	m = pulsar(m, "enter")

	// 6. Carpeta SIDC: debe pedirla y venir precargada con D:\SIDC_PREVIO
	if m.screen != screenAppDir {
		t.Fatalf("pantalla = %v, quiero screenAppDir", m.screen)
	}
	if m.dirInput.Value() != `D:\SIDC_PREVIO` {
		t.Errorf("dirInput = %q, quiero D:\\SIDC_PREVIO precargado", m.dirInput.Value())
	}
	m = pulsar(m, "enter")

	if m.screen != screenDone {
		t.Fatalf("pantalla = %v, quiero screenDone", m.screen)
	}

	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("error al leer config: %v", err)
	}
	if saved.Server != "SERVIDOR_PREVIO,54321" {
		t.Errorf("server = %q, quiero SERVIDOR_PREVIO,54321", saved.Server)
	}
	if saved.Database != "SIDC_PREVIO" {
		t.Errorf("database = %q, quiero SIDC_PREVIO", saved.Database)
	}
	if saved.UseWinAuth {
		t.Errorf("use_win_auth = true, quiero false")
	}
	if saved.SQLUser != "usuario_previo" {
		t.Errorf("sql_user = %q, quiero usuario_previo", saved.SQLUser)
	}
	if saved.AppDir != `D:\SIDC_PREVIO` {
		t.Errorf("app_dir = %q, quiero D:\\SIDC_PREVIO", saved.AppDir)
	}
}
