// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"aegis-setup/internal/check"
	"aegis-setup/internal/config"
	"aegis-setup/internal/securestore"
	"aegis-setup/internal/setup"

	"github.com/spf13/cobra"
)

func newViewReportCmd(res func() (config.Config, string, error)) *cobra.Command {
	return &cobra.Command{
		Use:          "view-report",
		Short:        "Abre Rpt_Caja_Chica en el visor Crystal x86",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := res()
			if err != nil {
				return err
			}
			if cfg.AppDir == "" {
				return fmt.Errorf("app_dir sin configurar")
			}
			password := ""
			if !cfg.UseWinAuth {
				password = securestore.ResolvePassword("", func() string { return setup.DSNPassword(cfg.DsnName) })
			}
			appConn := ""
			appExe := filepath.Join(cfg.AppDir, setup.SIDCExeName)
			if conn, readErr := setup.ReadExeConnString(appExe); readErr == nil {
				appConn = conn
			}
			ok, info := check.OpenReportVisual(context.Background(), filepath.Join(cfg.AppDir, "Reportes"), cfg, password, appConn)
			fmt.Fprintln(cmd.OutOrStdout(), info)
			if !ok {
				return fmt.Errorf("visor Crystal: %s", info)
			}
			return nil
		},
	}
}
