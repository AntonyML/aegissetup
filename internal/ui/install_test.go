// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"strconv"
	"strings"
	"testing"

	"aegis-setup/internal/config"
)

// devCfg es el preset dev/docker: SQL Auth, así que las dos claves se piden.
func devCfg() config.Config {
	cfg := config.Default()
	cfg.Env, cfg.DbMode, cfg.Server = "dev", config.DbDocker, "localhost,14333"
	cfg.UseWinAuth, cfg.SQLUser = false, "dev"
	return cfg
}

// prodCfg es prod con Windows Auth: no hay ninguna clave que pedir.
func prodCfg() config.Config {
	cfg := config.Default()
	cfg.Env, cfg.DbMode, cfg.Server = "prod", config.DbLocal, "localhost"
	cfg.UseWinAuth = true
	return cfg
}

// El orden de la instalación completa es un contrato: db -> app -> check.
func TestInstallSeqOrdenFijo(t *testing.T) {
	got := installSeq()
	want := []taskKind{taskSetupDB, taskSetupApp, taskCheck}
	if len(got) != len(want) {
		t.Fatalf("pasos = %d, quiero %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("paso %d = %v, quiero %v", i, got[i], want[i])
		}
	}
}

func TestSecretNeeds(t *testing.T) {
	none := func(string) bool { return false }
	all := func(string) bool { return true }
	only := func(set ...string) func(string) bool {
		return func(n string) bool {
			for _, s := range set {
				if s == n {
					return true
				}
			}
			return false
		}
	}

	cases := []struct {
		name string
		cfg  config.Config
		set  func(string) bool
		want []string
	}{
		{"dev sin nada -> pide las dos, en orden", devCfg(), none,
			[]string{envSAPassword, envAppPassword}},
		{"dev con SA -> pide solo la de la app", devCfg(), only(envSAPassword),
			[]string{envAppPassword}},
		{"dev con la app -> pide solo la SA", devCfg(), only(envAppPassword),
			[]string{envSAPassword}},
		{"dev con las dos -> no pide nada", devCfg(), all, nil},
		{"prod Windows Auth -> nunca pide nada", prodCfg(), none, nil},
		{"prod Windows Auth con claves de sobra -> tampoco", prodCfg(), all, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := secretNeeds(tc.cfg, tc.set)
			if len(got) != len(tc.want) {
				t.Fatalf("pidio %v, quiero %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("posicion %d = %q, quiero %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// Guarda contra el bug real ya documentado: el menú "1=db 2=app 3=check 4=dashboard"
// quedó escrito en 3 lugares y ninguno coincidía con el menú de verdad. Este test
// falla si alguien vuelve a atar el despacho al índice en vez de a la acción.
func TestMenuItemsContrato(t *testing.T) {
	if len(menuItems) != 7 {
		t.Fatalf("entradas = %d, quiero 7 (0..6)", len(menuItems))
	}

	keys := map[string]bool{}
	actions := map[action]bool{}
	for _, it := range menuItems {
		if it.key == "" {
			t.Error("hay una entrada sin tecla")
		}
		if keys[it.key] {
			t.Errorf("tecla repetida: %q", it.key)
		}
		keys[it.key] = true
		if actions[it.action] {
			t.Errorf("accion repetida en la tecla %q", it.key)
		}
		actions[it.action] = true
		if it.name == "" || it.desc == "" {
			t.Errorf("entrada %q sin nombre o sin descripcion", it.key)
		}
	}

	first := menuItems[0]
	if first.key != "0" {
		t.Errorf("la primera entrada es la tecla %q, quiero 0", first.key)
	}
	if first.action != actInstall {
		t.Errorf("la tecla 0 ejecuta %v, quiero actInstall", first.action)
	}
	if !actions[actInstall] {
		t.Error("no existe ninguna entrada con actInstall")
	}
}

func TestStepTitle(t *testing.T) {
	cases := map[taskKind]string{
		taskSetupDB:  "SETUP DB",
		taskSetupApp: "SETUP APP",
		taskCheck:    "CHECK",
	}
	for k, want := range cases {
		if got := stepTitle(k); got != want {
			t.Errorf("stepTitle(%v) = %q, quiero %q", k, got, want)
		}
	}
}

// El paso de la instalación completa lleva el número del total, para que el
// operador sepa dónde está cuando algo tarda o falla.
func TestStepLabel(t *testing.T) {
	seq := installSeq()
	for i, k := range seq {
		want := stepTitle(k)
		got := stepLabel(i, len(seq), k)
		if !strings.Contains(got, want) {
			t.Errorf("stepLabel(%d) = %q, no contiene %q", i, got, want)
		}
		if !strings.Contains(got, strconv.Itoa(i+1)) || !strings.Contains(got, strconv.Itoa(len(seq))) {
			t.Errorf("stepLabel(%d) = %q, no muestra %d/%d", i, got, i+1, len(seq))
		}
	}
}

// La clave que se pide en el prompt tiene que ser la misma que el parche acepta:
// si esto falla, el operador escribe algo que se rechaza recien al final del RESTORE.
func TestPromptValidaConLaMismaReglaQueElParche(t *testing.T) {
	cfg := devCfg()
	if err := validateAppPassword(cfg, "Dv*123"); err != nil {
		t.Errorf("clave de 6 chars rechazada: %v", err)
	}
	if err := validateAppPassword(cfg, ""); err == nil {
		t.Error("clave vacia aceptada")
	}
	// 9 chars con user dev = "UID=dev;PWD=" (12) + 9 = 21 > 20 -> el parche la rechaza.
	err := validateAppPassword(cfg, "123456789")
	if err == nil {
		t.Fatal("clave de 9 chars aceptada para user dev, pero el parche la rechaza")
	}
	if !strings.Contains(err.Error(), "8") {
		t.Errorf("el error no dice el maximo (8): %v", err)
	}
	// prod no parchea: no hay tope que aplicar.
	if err := validateAppPassword(prodCfg(), "unaClaveLarguisimaDeProd"); err != nil {
		t.Errorf("prod no deberia validar tope: %v", err)
	}
}
