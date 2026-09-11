// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"aegis-setup/internal/config"
	"aegis-setup/internal/precheck"
	"aegis-setup/internal/setup"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	ExitOK         = 0
	ExitGeneralErr = 1
	ExitConfigErr  = 2
	ExitCheckFail  = 3
)

var (
	ErrConfig = errors.New("error de configuración")
	ErrNoTTY  = errors.New("se requiere un subcomando explícito en entornos no interactivos")
	// ErrCheckFail marca que el diagnóstico corrió bien y encontró fallas. Es
	// distinto de un error de configuración (2) o de un problema del propio Aegis
	// (1): así un script puede distinguir "esta máquina no está lista" de
	// "Aegis se rompió", que se arreglan de maneras opuestas.
	ErrCheckFail = errors.New("la máquina no está lista")
)

var isTerminal = func(f *os.File) bool {
	if f == nil {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// La elevación sale a variables para poder probar el flujo sin un cartel de UAC de
// por medio: pedir permisos de verdad es una acción del operador, no algo que una
// prueba pueda apretar.
var (
	isAdmin       = setup.IsAdmin
	skipElevation = setup.SkipElevation
	elevate       = setup.Elevate
)

// comandosQueEscriben es la única lista de quién pide permisos. Sólo los que
// modifican la máquina: check, checklist y dashboard tienen que poder correr sin
// permisos, porque son justamente lo que se usa cuando algo está mal, y una PC rota
// suele ser una donde no hay forma de aceptar un UAC (sesión remota, script, otro
// usuario).
var comandosQueEscriben = map[string]bool{
	"setup-db":  true,
	"setup-app": true,
}

// writesToSystem dice si un subcomando modifica la máquina (y por lo tanto necesita
// permisos de administrador).
func writesToSystem(sub string) bool { return comandosQueEscriben[sub] }

// subcomandoDe dice qué subcomando se pidió, sin ejecutar nada. Se apoya en el
// buscador de cobra para no reimplementar el parseo de banderas (--config toma
// valor, --help no).
func subcomandoDe(root *cobra.Command, args []string) string {
	target, _, err := root.Find(args)
	if err != nil || target == nil {
		return ""
	}
	return target.Name()
}

// asegurarAdmin decide si este proceso tiene que relanzarse elevado antes de
// ejecutar el trabajo.
//
// Devuelve hecho=true cuando el trabajo lo hizo el proceso elevado: el padre no
// puede seguir, porque dos Aegis escribiendo la misma base al mismo tiempo se pisan.
// El código de salida del padre es el del proceso elevado (exit 3 tiene que seguir
// siendo exit 3), y por eso no se devuelve un error para el caso "terminó con
// código 3": cobra lo imprimiría como un fallo de Aegis, que no es lo que pasó.
func asegurarAdmin(w io.Writer, sub string, args []string) (bool, int, error) {
	if !writesToSystem(sub) {
		return false, 0, nil
	}
	if !setup.NeedsElevation(isAdmin(), skipElevation()) {
		return false, 0, nil
	}
	// El aviso va antes del cartel de UAC: un pedido de permisos que aparece sin
	// explicación en la consola se lee como si el programa estuviera haciendo algo
	// raro.
	fmt.Fprintf(w, "Para %q hacen falta permisos de administrador: se relanza el mismo comando elevado.\n", sub)
	code, err := elevate(args)
	if err != nil {
		return false, 0, fmt.Errorf("no se pudo pedir permisos de administrador: %w", err)
	}
	// El código se imprime siempre: es lo único que queda de la salida del proceso
	// elevado si la consola nueva no se hereda.
	fmt.Fprintf(w, "El comando elevado terminó con código %d.\n", code)
	return true, code, nil
}

// NewRootCmd crea el comando raíz y registra setup-db, setup-app, check, dashboard, menu, configure.
func NewRootCmd(exeDir string, cfgLoader func(cfgPath string) (config.Config, error)) *cobra.Command {
	var configPath string
	var presetServer string

	cmd := &cobra.Command{
		Use:   "aegis",
		Short: "Aegis Setup: instala SIDC (DB + App) sin pasos manuales",
		Long: `Aegis Setup parametriza el flujo SIDC en N PCs.

Modos:
  dev  -> Docker (SQL Auth) para pruebas de migración.
  prod -> local (misma PC, Windows Auth) | docker (Docker por red) | server (instancia xxxx).

Subcomandos:
  setup-db   Restaura el .bak como SIDC, compat, collation, logins.
  setup-app  DSN SIDC_SQL 32-bit + OCX legacy + verifica exe/reportes + parche _DOCKER.
  check      Verifica que App y DB se hablan (TCP + SQL + DSN + ficheros).
  checklist  Requisitos de la máquina en orden: qué falta, por qué y qué lo traba.
             Sale con código 3 si algo traba la instalación, y 0 si no traba nada.
  dashboard  Panel de estado no interactivo (para pegar en un correo de soporte).
  menu       Menú interactivo (0=instalación completa, 1=db, 2=app, 3=check, 4-6=presets).
  configure  Genera config.json inicial.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isTerminal(os.Stdin) {
				cfg, path, err := resolveCfg(exeDir, configPath, cfgLoader)
				return runMenu(cmd, cfg, path, err, presetServer)
			}
			_ = cmd.Help()
			return ErrNoTTY
		},
	}

	cmd.PersistentFlags().StringVar(&configPath, "config", "", "ruta a config.json (default: %APPDATA%\\AegisSetup\\config.json)")
	cmd.PersistentFlags().StringVar(&presetServer, "server", "", "server para el preset 'prod server' del TUI (ej. aegis --server MI_SERVIDOR)")

	cmd.AddCommand(newSetupDbCmd(func() (config.Config, string, error) {
		return resolveCfg(exeDir, configPath, cfgLoader)
	}))
	cmd.AddCommand(newSetupAppCmd(func() (config.Config, string, error) {
		return resolveCfg(exeDir, configPath, cfgLoader)
	}))
	cmd.AddCommand(newCheckCmd(func() (config.Config, string, error) {
		return resolveCfg(exeDir, configPath, cfgLoader)
	}))
	cmd.AddCommand(newChecklistCmd(func() (config.Config, string, error) {
		return resolveCfg(exeDir, configPath, cfgLoader)
	}, precheck.SondasReales))
	cmd.AddCommand(newDashboardCmd(func() (config.Config, string, error) {
		return resolveCfg(exeDir, configPath, cfgLoader)
	}))
	cmd.AddCommand(newMenuCmd(func() (config.Config, string, error) {
		return resolveCfg(exeDir, configPath, cfgLoader)
	}, func() string { return presetServer }))
	cmd.AddCommand(newConfigureCmd(exeDir))

	return cmd
}

func resolveCfg(exeDir, flag string, loader func(string) (config.Config, error)) (config.Config, string, error) {
	path := configPathToUse(exeDir, flag)
	cfg, err := loader(path)
	if err != nil {
		return config.Config{}, path, fmt.Errorf("%w: %v", ErrConfig, err)
	}
	if err := cfg.Validate(); err != nil {
		return config.Config{}, path, fmt.Errorf("%w: %v", ErrConfig, err)
	}
	return cfg, path, nil
}

// configPathToUse resuelve qué config.json se usa. El flag manda; si no, se
// prefiere el canónico de %APPDATA%\AegisSetup y se sigue aceptando el viejo
// junto al binario para no romper instalaciones previas a F3. Cuando no existe
// ninguno devuelve el canónico, que es donde se escribe un config nuevo.
func configPathToUse(exeDir, flag string) string {
	if flag != "" {
		return flag
	}
	canonico := config.RutaConfig()
	if existeArchivo(canonico) {
		return canonico
	}
	legado := filepath.Join(exeDir, "config.json")
	if existeArchivo(legado) {
		return legado
	}
	return canonico
}

func existeArchivo(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func Execute() int {
	dir := exeDir()

	// Las carpetas de datos (ProgramData) y de config (%APPDATA%) se crean acá.
	// Es best-effort a propósito: "aegis check" es la herramienta de diagnóstico
	// y tiene que poder correr justamente cuando algo está mal.
	if err := config.AsegurarRutas(); err != nil {
		fmt.Fprintf(os.Stderr, "aviso: %v\n", err)
	}

	cmd := NewRootCmd(dir, config.Load)

	// La elevación se decide ANTES de que cobra ejecute el trabajo: si hay que
	// relanzar, el proceso padre no llega a hacer nada y su código de salida es el del
	// proceso elevado. Resolverlo dentro del RunE obligaría a distinguir "terminé
	// bien" de "terminó allá" con un error, y cobra imprimiría un "Error:" que no lo es.
	if hecho, code, err := asegurarAdmin(os.Stdout, subcomandoDe(cmd, os.Args[1:]), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return ExitGeneralErr
	} else if hecho {
		return code
	}

	if err := cmd.Execute(); err != nil {
		switch {
		case errors.Is(err, ErrConfig):
			return ExitConfigErr
		case errors.Is(err, ErrCheckFail):
			return ExitCheckFail
		}
		return ExitGeneralErr
	}
	return ExitOK
}

func exeDir() string {
	if exe, err := os.Executable(); err == nil {
		if dir := filepath.Dir(exe); dir != "" {
			return dir
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}
