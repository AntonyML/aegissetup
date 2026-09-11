// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"aegis-setup/internal/config"

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
)

var isTerminal = func(f *os.File) bool {
	if f == nil {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// NewRootCmd crea el comando raíz y registra setup-db, setup-app, check, dashboard, menu, configure.
func NewRootCmd(exeDir string, cfgLoader func(cfgPath string) (config.Config, error)) *cobra.Command {
	var configPath string

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
  dashboard  Panel de estado no interactivo.
  menu       Menú interactivo (1=db 2=app 3=check 4=dashboard).
  configure  Genera config.json inicial.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isTerminal(os.Stdin) {
				cfg, path, err := resolveCfg(exeDir, configPath, cfgLoader)
				return runMenu(cmd, cfg, path, err)
			}
			_ = cmd.Help()
			return ErrNoTTY
		},
	}

	cmd.PersistentFlags().StringVar(&configPath, "config", "", "ruta a config.json (default: config.json junto al binario)")

	cmd.AddCommand(newSetupDbCmd(func() (config.Config, string, error) {
		return resolveCfg(exeDir, configPath, cfgLoader)
	}))
	cmd.AddCommand(newSetupAppCmd(func() (config.Config, string, error) {
		return resolveCfg(exeDir, configPath, cfgLoader)
	}))
	cmd.AddCommand(newCheckCmd(func() (config.Config, string, error) {
		return resolveCfg(exeDir, configPath, cfgLoader)
	}))
	cmd.AddCommand(newDashboardCmd(func() (config.Config, string, error) {
		return resolveCfg(exeDir, configPath, cfgLoader)
	}))
	cmd.AddCommand(newMenuCmd(func() (config.Config, string, error) {
		return resolveCfg(exeDir, configPath, cfgLoader)
	}))
	cmd.AddCommand(newConfigureCmd(exeDir))

	return cmd
}

func resolveCfg(exeDir, flag string, loader func(string) (config.Config, error)) (config.Config, string, error) {
	path := flag
	if path == "" {
		path = filepath.Join(exeDir, "config.json")
	}
	cfg, err := loader(path)
	if err != nil {
		return config.Config{}, path, fmt.Errorf("%w: %v", ErrConfig, err)
	}
	if err := cfg.Validate(); err != nil {
		return config.Config{}, path, fmt.Errorf("%w: %v", ErrConfig, err)
	}
	return cfg, path, nil
}

func Execute() int {
	dir := exeDir()
	cmd := NewRootCmd(dir, config.Load)
	if err := cmd.Execute(); err != nil {
		if errors.Is(err, ErrConfig) {
			return ExitConfigErr
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
