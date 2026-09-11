// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"fmt"
	"path/filepath"

	"aegis-setup/internal/config"

	"github.com/spf13/cobra"
)

func newConfigureCmd(exeDir string) *cobra.Command {
	var env, dbMode, server, out string
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
			if cfg.Env == "prod" && (cfg.DbMode == config.DbLocal || cfg.DbMode == config.DbServer) {
				cfg.UseWinAuth = true
				cfg.Driver = "SQL Server"
				if cfg.Server == "localhost,14333" {
					cfg.Server = "localhost"
				}
			}
			path := out
			if path == "" {
				path = filepath.Join(exeDir, "config.json")
			}
			if err := cfg.Save(path); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "config escrita en %s (env=%s db_mode=%s server=%s)\n", path, cfg.Env, cfg.DbMode, cfg.Server)
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "", "dev|prod")
	cmd.Flags().StringVar(&dbMode, "db-mode", "", "docker|local|server")
	cmd.Flags().StringVar(&server, "server", "", "ej localhost,14333 | localhost | CONTABILIDAD | MI_SERVIDOR")
	cmd.Flags().StringVar(&out, "out", "", "ruta de salida (default: config.json junto al binario)")
	return cmd
}
