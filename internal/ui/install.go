// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"errors"
	"fmt"

	"aegis-setup/internal/config"
	"aegis-setup/internal/setup"
)

// Variables de entorno de las claves. No van a config.json a proposito.
const (
	envSAPassword  = "AEGIS_SA_PASSWORD"
	envAppPassword = "AEGIS_SQL_PASSWORD"
)

// action es lo que ejecuta una entrada del menu. Existe para que agregar una
// entrada nueva no dependa de su indice: atar el despacho al indice fue lo que
// dejo el menu documentado ("1=db 2=app 3=check 4=dashboard") desincronizado
// del menu real, en 3 lugares distintos.
type action int

const (
	actInstall action = iota
	actConfigManual
	actSetupApp
	actCheck
	actPresetDev
	actPresetProdLocal
	actPresetProdServer
	actRefreshLogos
	actSetupDB
)

// menuEntry es una entrada del menu del TUI.
type menuEntry struct {
	key    string
	name   string
	desc   string
	action action
}

// installSeq es el orden de la instalacion de puesto. No se elige ni se
// reordena: es un contrato (desacoplado de restaurar .bak).
func installSeq() []taskKind { return []taskKind{taskSetupApp, taskCheck} }

func stepTitle(k taskKind) string {
	switch k {
	case taskSetupDB:
		return "SETUP DB"
	case taskSetupApp:
		return "SETUP APP"
	case taskCheck:
		return "CHECK"
	case taskInstall:
		return "INSTALACIÓN COMPLETA"
	case taskRefreshLogos:
		return "REFRESCAR LOGOS"
	}
	return "?"
}

// stepLabel numera el paso: RESTORE + check puede tardar minutos y el operador
// necesita saber donde esta y cuantos le faltan.
func stepLabel(i, total int, k taskKind) string {
	return fmt.Sprintf("PASO %d/%d - %s", i+1, total, stepTitle(k))
}

// secretNeeds devuelve en orden las claves que faltan pedir antes de correr la
// instalacion completa. Lista vacia = no hay nada que pedir: prod con Windows
// Auth nunca pide, porque no guarda claves.
func secretNeeds(cfg config.Config, isSet func(string) bool) []string {
	if cfg.UseWinAuth {
		return nil
	}
	var out []string
	if !isSet(envAppPassword) {
		out = append(out, envAppPassword)
	}
	return out
}

// validateSecret valida una clave ya escrita, antes de arrancar el flujo.
func validateSecret(cfg config.Config, name, value string) error {
	if value == "" {
		return fmt.Errorf("%s no puede quedar vacía", name)
	}
	if name == envAppPassword {
		return validateAppPassword(cfg, value)
	}
	return nil
}

// validateAppPassword usa la MISMA regla que PatchDockerExe. Si el prompt
// aceptara una clave que el parche rechaza, el operador se enteraria recien al
// final de la instalacion, despues del RESTORE.
func validateAppPassword(cfg config.Config, pass string) error {
	if pass == "" {
		return errors.New("la clave del login de app no puede quedar vacía")
	}
	if cfg.UseWinAuth {
		return nil // prod no parchea _DOCKER, no hay tope que aplicar
	}
	max := setup.DockerPatchBudget(cfg.SQLUser)
	if max <= 0 {
		return fmt.Errorf("el usuario %q no deja espacio para la clave: UID=%s;PWD=... ya mide %d de 20",
			cfg.SQLUser, cfg.SQLUser, 20-max)
	}
	if len(pass) > max {
		return fmt.Errorf("clave de %d chars: con el usuario %q caben %d (UID=%s;PWD=... debe medir 20 en total)",
			len(pass), cfg.SQLUser, max, cfg.SQLUser)
	}
	return nil
}

// secretHint dice de donde sale la clave que se esta pidiendo, para que nadie
// tenga que adivinar ni buscar en el repo.
func secretHint(cfg config.Config, name string) string {
	switch name {
	case envSAPassword:
		return "Es la clave de 'sa' del motor (SA_PASSWORD en docker/.env)."
	case envAppPassword:
		if cfg.UseWinAuth {
			return "Prod usa Windows Auth: la clave no se guarda en ningun lado."
		}
		return fmt.Sprintf("Es la del login de app %q. Máximo %d chars: se embebe en el exe como UID=%s;PWD=...",
			cfg.SQLUser, setup.DockerPatchBudget(cfg.SQLUser), cfg.SQLUser)
	}
	return ""
}
