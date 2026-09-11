// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"fmt"

	"aegis-setup/internal/config"

	"github.com/spf13/cobra"
)

func newConfigureCmd(_ string) *cobra.Command {
	var env, dbMode, server, appDir, out string
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Genera config.json inicial (dev docker / prod local|docker|server)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Default()
			if env != "" {
				cfg.Env = env
			}
			switch dbMode {
			case "":
			case "docker":
				cfg.DbMode = config.DbDocker
			case "local":
				cfg.DbMode = config.DbLocal
			case "server":
				cfg.DbMode = config.DbServer
			default:
				return fmt.Errorf("db-mode debe ser docker|local|server")
			}
			if server != "" {
				cfg.Server = server
			}
			if appDir != "" {
				cfg.AppDir = appDir
			}
			if cfg.Env == "prod" && (cfg.DbMode == config.DbLocal || cfg.DbMode == config.DbServer) {
				cfg.UseWinAuth = true
				cfg.Driver = "SQL Server"
				if cfg.Server == "localhost,14333" {
					cfg.Server = "localhost"
				}
			}
			path := out
			if path == "" {
				path = config.RutaConfig()
				if err := config.AsegurarRutas(); err != nil {
					return err
				}
			}
			if err := cfg.Save(path); err != nil {
				return err
			}
			// El kit de Docker es parte del modo docker, no del paso 1: si se deja recién ahí,
			// el arreglo que el checklist da para levantar el motor apunta a un archivo que
			// todavía no existe. Se deja acá y también en el paso 1 (config editada a mano).
			out := func(s string) { fmt.Fprintln(cmd.OutOrStdout(), s) }
			for _, f := range InstalarCompose(cfg.DbMode, out) {
				out("DOCKER PENDIENTE: " + f)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "config escrita en %s (env=%s db_mode=%s server=%s app_dir=%s)\n", path, cfg.Env, cfg.DbMode, cfg.Server, cfg.AppDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "", "dev|prod")
	cmd.Flags().StringVar(&dbMode, "db-mode", "", "docker|local|server")
	cmd.Flags().StringVar(&server, "server", "", "ej localhost,14333 | localhost | CONTABILIDAD | MI_SERVIDOR")
	cmd.Flags().StringVar(&appDir, "app-dir", "", "carpeta donde está SIDC en esta PC (ej. C:\\SIDC); vacío = el TUI la pregunta")
	cmd.Flags().StringVar(&out, "out", "", "ruta de salida (default: %APPDATA%\\AegisSetup\\config.json)")
	return cmd
}
