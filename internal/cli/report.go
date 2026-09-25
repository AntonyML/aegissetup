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

func newExportPDFCmd(res func() (config.Config, string, error)) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:          "export-pdf --output <ruta>",
		Short:        "Genera un PDF moderno de Rpt_Caja_Chica sin usar el exportador PDF de Crystal",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if output == "" {
				return fmt.Errorf("falta --output: indicá dónde guardar el PDF")
			}
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
			outputPath, absErr := filepath.Abs(output)
			if absErr != nil {
				return fmt.Errorf("ruta de salida inválida: %w", absErr)
			}
			ok, info := check.ExportReportPDF(context.Background(), filepath.Join(cfg.AppDir, "Reportes"), cfg, password, appConn, outputPath)
			fmt.Fprintln(cmd.OutOrStdout(), info)
			if !ok {
				return fmt.Errorf("exportación PDF: %s", info)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "ruta del PDF de salida")
	return cmd
}
