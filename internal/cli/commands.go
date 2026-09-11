// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"context"
	"fmt"
	"os"

	"aegis-setup/internal/check"
	"aegis-setup/internal/config"
	"aegis-setup/internal/setup"

	"github.com/spf13/cobra"
)

func newSetupDbCmd(res func() (config.Config, string, error)) *cobra.Command {
	var bak, saPass, appPass string
	cmd := &cobra.Command{
		Use:   "setup-db",
		Short: "Setup 1/2: restaura el .bak como SIDC (compat, collation, logins)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := res()
			if err != nil {
				return err
			}
			if saPass == "" {
				saPass = os.Getenv("AEGIS_SA_PASSWORD")
			}
			if appPass == "" {
				appPass = os.Getenv("AEGIS_SQL_PASSWORD")
			}
			out := func(s string) { fmt.Fprintln(cmd.OutOrStdout(), s) }
			return setup.SetupDB(context.Background(), cfg, bak, saPass, appPass, out)
		},
	}
	cmd.Flags().StringVar(&bak, "bak", "", ".bak a restaurar (default: el más nuevo de backup_dir)")
	cmd.Flags().StringVar(&saPass, "sa-password", "", "clave SA (o env AEGIS_SA_PASSWORD; vacío = Windows Auth)")
	cmd.Flags().StringVar(&appPass, "app-password", "", "clave del login app SQL Auth (o env AEGIS_SQL_PASSWORD)")
	return cmd
}

func newSetupAppCmd(res func() (config.Config, string, error)) *cobra.Command {
	var appPass string
	var savePWD, patch bool
	cmd := &cobra.Command{
		Use:   "setup-app",
		Short: "Setup 2/2: DSN SIDC_SQL 32-bit + OCX legacy + verifica app (+parche _DOCKER en dev)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := res()
			if err != nil {
				return err
			}
			if appPass == "" {
				appPass = os.Getenv("AEGIS_SQL_PASSWORD")
			}
			out := func(s string) { fmt.Fprintln(cmd.OutOrStdout(), s) }
			for _, m := range setup.CheckAppFiles(cfg.AppDir) {
				fmt.Fprintln(cmd.OutOrStdout(), "FALTA: "+m)
			}
			if err := setup.WriteDSN(cfg, appPass, savePWD, out); err != nil {
				return err
			}
			if failed := setup.InstallOCX(cfg.LegacyDir, out); len(failed) > 0 {
				for _, f := range failed {
					fmt.Fprintln(cmd.OutOrStdout(), "OCX PENDIENTE: "+f)
				}
			}
			if patch && !cfg.UseWinAuth {
				if err := setup.PatchDockerExe(cfg.AppDir, cfg.SQLUser, appPass, out); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&appPass, "app-password", "", "clave login app (o env AEGIS_SQL_PASSWORD)")
	cmd.Flags().BoolVar(&savePWD, "save-pwd", false, "guardar PWD en el DSN (SOLO dev/docker, nunca prod)")
	cmd.Flags().BoolVar(&patch, "patch-docker", false, "generar _DOCKER.exe con UID/PWD (solo dev/docker)")
	return cmd
}

func newCheckCmd(res func() (config.Config, string, error)) *cobra.Command {
	var appPass string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Verifica que App y DB se hablan (TCP + SQL + DSN + ficheros)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := res()
			if err != nil {
				return err
			}
			if appPass == "" {
				appPass = os.Getenv("AEGIS_SQL_PASSWORD")
			}
			rs := check.Run(context.Background(), cfg, appPass)
			fail := 0
			for _, r := range rs {
				mark := "OK  "
				if !r.OK {
					mark = "FAIL"
					fail++
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %-22s %s\n", mark, r.Name, r.Info)
			}
			if fail > 0 {
				return fmt.Errorf("check: %d fallos", fail)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&appPass, "app-password", "", "clave login app (o env AEGIS_SQL_PASSWORD)")
	return cmd
}

func newDashboardCmd(res func() (config.Config, string, error)) *cobra.Command {
	var appPass string
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Panel de estado (env, DSN, DB, app)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := res()
			if err != nil {
				return err
			}
			if appPass == "" {
				appPass = os.Getenv("AEGIS_SQL_PASSWORD")
			}
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "== AEGIS DASHBOARD ==")
			fmt.Fprintf(w, "config: %s  env=%s db_mode=%s\n", path, cfg.Env, cfg.DbMode)
			fmt.Fprintf(w, "server=%s database=%s dsn=%s driver=%s winAuth=%v\n", cfg.Server, cfg.Database, cfg.DsnName, cfg.Driver, cfg.UseWinAuth)
			for _, r := range check.Run(context.Background(), cfg, appPass) {
				mark := "OK  "
				if !r.OK {
					mark = "FAIL"
				}
				fmt.Fprintf(w, "%s %-22s %s\n", mark, r.Name, r.Info)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&appPass, "app-password", "", "clave login app (o env AEGIS_SQL_PASSWORD)")
	return cmd
}

func newMenuCmd(res func() (config.Config, string, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "menu",
		Short: "Menú interactivo (1=db 2=app 3=check 4=dashboard 0=salir)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := res()
			return runMenu(cmd, cfg, path, err)
		},
	}
}
