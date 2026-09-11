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

	m = pulsar(m, "6")
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
