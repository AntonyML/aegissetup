// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"aegis-setup/internal/check"
	"aegis-setup/internal/config"
	"aegis-setup/internal/precheck"
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
	var o setupAppOpts
	cmd := &cobra.Command{
		Use:   "setup-app",
		Short: "Setup 2/2: DSN SIDC_SQL 32-bit + OCX legacy + Crystal + verifica app (+parche _DOCKER en dev)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := res()
			if err != nil {
				return err
			}
			if o.appPass == "" {
				o.appPass = os.Getenv("AEGIS_SQL_PASSWORD")
			}
			return ejecutarSetupApp(cfg, o, func(s string) { fmt.Fprintln(cmd.OutOrStdout(), s) })
		},
	}
	cmd.Flags().StringVar(&o.appPass, "app-password", "", "clave login app (o env AEGIS_SQL_PASSWORD)")
	cmd.Flags().BoolVar(&o.savePWD, "save-pwd", false, "guardar PWD en el DSN (SOLO dev/docker, nunca prod)")
	cmd.Flags().BoolVar(&o.patch, "patch-docker", false, "generar _DOCKER.exe con UID/PWD (solo dev/docker)")
	return cmd
}

// setupAppOpts son las banderas del paso 2.
type setupAppOpts struct {
	appPass string
	savePWD bool
	patch   bool
}

// ejecutarSetupApp es el paso 2 completo: DSN, OCX legacy, runtime de Crystal y el
// parche _DOCKER en dev. Está aparte del comando para poder probar el orden y el
// alcance de los pasos que tocan la máquina sin escribir en el registro ni reescribir
// el .exe de la app.
func ejecutarSetupApp(cfg config.Config, o setupAppOpts, out func(string)) error {
	for _, m := range setup.CheckAppFiles(cfg.AppDir) {
		out("FALTA: " + m)
	}
	if err := escribirDSN(cfg, o.appPass, o.savePWD, out); err != nil {
		return err
	}
	for _, f := range setup.InstallOCX(cfg.LegacyDir, out) {
		out("OCX PENDIENTE: " + f)
	}
	// El runtime de Crystal sale del propio binario: en una PC limpia no hay carpeta de
	// instalación de Crystal Reports que copiar.
	for _, f := range InstalarCrystal(out) {
		out("CRYSTAL PENDIENTE: " + f)
	}
	if o.patch && !cfg.UseWinAuth {
		// Mismo corte que el TUI: parchear con clave vacía deja un _DOCKER.exe que
		// arranca y falla al conectar, que es peor que no generarlo.
		if o.appPass == "" {
			return fmt.Errorf("falta AEGIS_SQL_PASSWORD para el parche _DOCKER")
		}
		return parcheDocker(cfg.AppDir, cfg.SQLUser, o.appPass, out)
	}
	return nil
}

func newCheckCmd(res func() (config.Config, string, error)) *cobra.Command {
	var appPass string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Verifica que App y DB se hablan (TCP + SQL + DSN + ficheros)",
		// Un reporte con código de salida no es un error de uso: volcar el "Usage"
		// acá tapa con 15 líneas de ayuda lo único que el operador quiere leer.
		SilenceUsage: true,
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
				return fmt.Errorf("%w: check: %d fallos", ErrCheckFail, fail)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&appPass, "app-password", "", "clave login app (o env AEGIS_SQL_PASSWORD)")
	return cmd
}

func newChecklistCmd(res func() (config.Config, string, error), sondas func() precheck.Sondas) *cobra.Command {
	var appPass string
	cmd := &cobra.Command{
		Use:   "checklist",
		Short: "Requisitos de la máquina en orden: qué falta, por qué y cómo resolverlo",
		// Igual que check: el reporte es la salida, no un error de uso.
		SilenceUsage: true,
		Long: `Recorre los requisitos en orden de resolución. Por cada uno que falte dice por
qué importa, qué hacer y qué opción del menú queda trabada.

Es el mismo checklist que muestra el menú interactivo con [C], en texto plano:
sirve para pegar la salida en un correo de soporte.

Sale con código 3 si algún requisito traba la instalación, y 0 si no traba ninguno.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := res()
			if err != nil {
				return err
			}
			if appPass == "" {
				appPass = os.Getenv("AEGIS_SQL_PASSWORD")
			}
			return escribirChecklist(cmd.OutOrStdout(), cfg, precheck.Run(cfg, appPass, sondas()))
		},
	}
	cmd.Flags().StringVar(&appPass, "app-password", "", "clave login app (o env AEGIS_SQL_PASSWORD)")
	return cmd
}

// marcaCLI usa ASCII a propósito. La TUI puede darse el lujo de los ✓/✗, pero
// esta salida puede terminar en el cmd.exe viejo de una PC del área, donde esos
// caracteres salen como basura.
func marcaCLI(e precheck.Estado) string {
	switch e {
	case precheck.EstadoOK:
		return "OK   "
	case precheck.EstadoAviso:
		return "AVISO"
	default:
		return "FALTA"
	}
}

func escribirChecklist(w io.Writer, cfg config.Config, rs []precheck.Requisito) error {
	fmt.Fprintln(w, "== AEGIS CHECKLIST ==")
	fmt.Fprintf(w, "env=%s db_mode=%s server=%s database=%s\n\n", cfg.Env, cfg.DbMode, cfg.Server, cfg.Database)

	for i, r := range rs {
		fmt.Fprintf(w, "%s %2d. %s\n", marcaCLI(r.Estado), i+1, r.Titulo)
		if r.Detalle != "" {
			fmt.Fprintf(w, "           %s\n", r.Detalle)
		}
		if r.Estado == precheck.EstadoOK {
			continue
		}
		fmt.Fprintf(w, "           por qué: %s\n", r.Motivo)
		fmt.Fprintf(w, "           arreglo: %s\n", r.Arreglo)
		if len(r.Traba) > 0 {
			fmt.Fprintf(w, "           traba:   %s\n", precheck.Etapas(r.Traba))
		}
		fmt.Fprintln(w)
	}

	bloquean := precheck.Bloqueantes(rs)
	if len(bloquean) == 0 {
		faltan := len(precheck.Faltantes(rs))
		switch {
		case faltan > 0:
			fmt.Fprintf(w, "== Nada traba la instalación. Hay %d requisitos sin cumplir que son resultado del flujo (la base y el DSN), no condición para empezar. ==\n", faltan)
		default:
			fmt.Fprintln(w, "== Todo en orden: el flujo completo está disponible. ==")
		}
		return nil
	}

	titulos := make([]string, 0, len(bloquean))
	for _, r := range bloquean {
		titulos = append(titulos, r.Titulo)
	}
	// "1 requisitos" se lee como un bug del programa y no como un dato.
	verbo := "requisitos traban"
	if len(bloquean) == 1 {
		verbo = "requisito traba"
	}
	fmt.Fprintf(w, "== %d %s la instalación: %s ==\n", len(bloquean), verbo, strings.Join(titulos, ", "))
	fmt.Fprintln(w, "== Resolvelos en el orden de la lista: los de arriba destraban a los de abajo. ==")
	return fmt.Errorf("%w: %d %s la instalación", ErrCheckFail, len(bloquean), verbo)
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

func newMenuCmd(res func() (config.Config, string, error), presetServer func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "menu",
		Short: "Menú interactivo (0=instalación completa 1=db 2=app 3=check 4-6=presets)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := res()
			return runMenu(cmd, cfg, path, err, presetServer())
		},
	}
}
