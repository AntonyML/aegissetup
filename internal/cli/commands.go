// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"aegis-setup/internal/check"
	"aegis-setup/internal/config"
	"aegis-setup/internal/precheck"
	"aegis-setup/internal/securestore"
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
			return ejecutarSetupDB(cfg, bak, saPass, appPass, out)
		},
	}
	cmd.Flags().StringVar(&bak, "bak", "", ".bak a restaurar (default: el más nuevo de backup_dir)")
	cmd.Flags().StringVar(&saPass, "sa-password", "", "clave SA (o env AEGIS_SA_PASSWORD; vacío = Windows Auth)")
	cmd.Flags().StringVar(&appPass, "app-password", "", "clave del login app SQL Auth (o env AEGIS_SQL_PASSWORD)")
	return cmd
}

// ejecutarSetupDB es el paso 1 completo: deja el kit de Docker en la PC (solo db_mode=docker)
// y restaura el .bak. Está aparte del RunE para poder probar el orden de esos dos pasos sin
// abrir la terminal ni restaurar una base de verdad.
func ejecutarSetupDB(cfg config.Config, bak, saPass, appPass string, out func(string)) error {
	for _, f := range InstalarCompose(cfg.DbMode, out) {
		out("DOCKER PENDIENTE: " + f)
	}
	return setup.SetupDB(context.Background(), cfg, bak, saPass, appPass, out)
}

func newSetupAppCmd(res func() (config.Config, string, error)) *cobra.Command {
	var o setupAppOpts
	cmd := &cobra.Command{
		Use:   "setup-app",
		Short: "Setup 2/2: DSN 32-bit + OCX/Crystal + regenera EXE/RPT y verifica app",
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
	cmd.Flags().BoolVar(&o.patch, "patch-docker", false, "generar ejecutable Docker versionado con UID/PWD (solo dev/docker)")
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
	// Los controles de VB6 salen del propio binario: en una PC limpia no hay carpeta
	// de la PC vieja de dónde copiarlos.
	for _, f := range InstalarOCX(cfg.LegacyDir, out) {
		out("OCX PENDIENTE: " + f)
	}
	// El runtime de Crystal sale del propio binario: en una PC limpia no hay carpeta de
	// instalación de Crystal Reports que copiar.
	for _, f := range InstalarCrystal(out) {
		out("CRYSTAL PENDIENTE: " + f)
	}
	// Parchear ejecutables y logos desde _ORIGINAL.exe
	if cfg.AppDir != "" {
		if err := setup.PatchSIDCApp(cfg, o.appPass, out); err != nil {
			out("AVISO APP: " + err.Error())
		}
	}
	if o.patch && !cfg.UseWinAuth {
		if o.appPass == "" {
			return fmt.Errorf("falta AEGIS_SQL_PASSWORD para el parche _DOCKER")
		}
		return parcheDocker(cfg.AppDir, cfg.SQLUser, o.appPass, out)
	}
	return nil
}

// ejecutarRepair corrige DSN corrupto, instala Crystal y OCX faltantes y aplica parche de logos y ejecutables.
func ejecutarRepair(cfg config.Config, appPass string, out func(string)) error {
	out("== AEGIS REPARACIÓN ==")
	// 1. DSN
	valid, diffs := setup.ValidateDSN(cfg, appPass)
	if !valid {
		out(fmt.Sprintf("DSN desalineado (%s). Reparando...", strings.Join(diffs, "; ")))
		if err := setup.RepairDSN(cfg, appPass, out); err != nil {
			out("AVISO DSN: " + err.Error())
		}
	} else {
		out("DSN SIDC_SQL: OK")
	}

	// 2. Crystal Reports
	if miss := setup.CheckCrystal(); len(miss) > 0 {
		out(fmt.Sprintf("Crystal Reports incompleto (%d componentes faltantes). Instalando...", len(miss)))
		for _, f := range InstalarCrystal(out) {
			out("CRYSTAL: " + f)
		}
	} else {
		out("Crystal Reports runtime: OK")
	}

	// 3. OCX
	out("Verificando controles OCX...")
	for _, f := range InstalarOCX(cfg.LegacyDir, out) {
		out("OCX: " + f)
	}

	// 4. Ejecutables y Logos SIDC (regenerados desde _ORIGINAL.exe)
	if cfg.AppDir != "" {
		if err := setup.PatchSIDCApp(cfg, appPass, out); err != nil {
			out("AVISO APP: " + err.Error())
		} else {
			out("SIDC App: OK (ejecutable y logos sincronizados)")
		}
	}

	return nil
}

func newCheckCmd(res func() (config.Config, string, error)) *cobra.Command {
	var appPass string
	var fix bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Verifica TCP + SQL + DSN + MSDASQL + Crystal + Spooler + impresora",
		// Un reporte con código de salida no es un error de uso: volcar el "Usage"
		// acá tapa con 15 líneas de ayuda lo único que el operador quiere leer.
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := res()
			if err != nil {
				return err
			}
			if appPass == "" {
				appPass = securestore.ResolvePassword("", func() string { return setup.DSNPassword(cfg.DsnName) })
			}
			if fix {
				out := func(s string) { fmt.Fprintln(cmd.OutOrStdout(), s) }
				_ = ejecutarRepair(cfg, appPass, out)
				out("")
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
	cmd.Flags().BoolVarP(&fix, "fix", "f", false, "intenta reparar automáticamente DSN, Crystal, OCX y logos si fallan")
	return cmd
}

func newRepairCmd(res func() (config.Config, string, error)) *cobra.Command {
	var appPass string
	cmd := &cobra.Command{
		Use:          "repair",
		Short:        "Autodetecta y corrige DSN, Crystal runtime, controles OCX y logos de SIDC",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := res()
			if err != nil {
				return err
			}
			if appPass == "" {
				appPass = os.Getenv("AEGIS_SQL_PASSWORD")
			}
			out := func(s string) { fmt.Fprintln(cmd.OutOrStdout(), s) }
			if err := ejecutarRepair(cfg, appPass, out); err != nil {
				return err
			}
			out("\n== VERIFICACIÓN POST-REPARACIÓN ==")
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
				return fmt.Errorf("%w: repair: persisten %d fallos", ErrCheckFail, fail)
			}
			out("\nREPARACIÓN COMPLETADA EXITOSAMENTE: todos los chequeos OK.")
			return nil
		},
	}
	cmd.Flags().StringVar(&appPass, "app-password", "", "clave login app (o env AEGIS_SQL_PASSWORD)")
	return cmd
}

type installCmdOpts struct {
	server   string
	appDir   string
	database string
	sqlUser  string
	appPass  string
	winAuth  bool
	yes      bool
}

func newInstallCmd(res func() (config.Config, string, error)) *cobra.Command {
	var o installCmdOpts
	cmd := &cobra.Command{
		Use:          "install",
		Short:        "Instalación completa no interactiva para terminales SIDC (DSN, OCX, Crystal, Logos, Check)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := res()
			if err != nil && !errors.Is(err, ErrConfig) {
				return err
			}
			if cfg.DsnName == "" {
				cfg = config.Default()
			}
			if o.server != "" {
				cfg.Server = o.server
			}
			if o.appDir != "" {
				cfg.AppDir = o.appDir
			}
			if o.database != "" {
				cfg.Database = o.database
			}
			if o.sqlUser != "" {
				cfg.SQLUser = o.sqlUser
			}
			if cmd.Flags().Changed("win-auth") {
				cfg.UseWinAuth = o.winAuth
			} else if o.server != "" || cfg.Server != "" {
				bestAuth, reason := setup.DetectBestAuth(context.Background(), cfg.Server, cfg.Database)
				cfg.UseWinAuth = bestAuth
				fmt.Fprintln(cmd.OutOrStdout(), "AUTODETECCIÓN:", reason)
			}

			if o.appPass == "" {
				o.appPass = os.Getenv("AEGIS_SQL_PASSWORD")
			}
			if !cfg.UseWinAuth && o.appPass == "" {
				o.appPass = securestore.ResolvePassword("", func() string { return setup.DSNPassword(cfg.DsnName) })
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("configuración no válida: %w", err)
			}

			if err := cfg.Save(path); err != nil {
				return fmt.Errorf("guardar config en %s: %w", path, err)
			}

			out := func(s string) { fmt.Fprintln(cmd.OutOrStdout(), s) }
			out("== AEGIS INSTALL (NO INTERACTIVO) ==")
			out(fmt.Sprintf("Server: %s | DB: %s | Auth: %s | AppDir: %s", cfg.Server, cfg.Database, cfg.AuthLabel(), cfg.AppDir))

			appOpts := setupAppOpts{
				appPass: o.appPass,
				savePWD: !cfg.UseWinAuth,
				patch:   cfg.DbMode == config.DbDocker && !cfg.UseWinAuth,
			}
			if err := ejecutarSetupApp(cfg, appOpts, out); err != nil {
				return fmt.Errorf("falló setup-app: %w", err)
			}

			out("\n== VERIFICACIÓN FINAL ==")
			rs := check.Run(context.Background(), cfg, o.appPass)
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
				return fmt.Errorf("%w: instalación completada pero check reportó %d fallos", ErrCheckFail, fail)
			}
			out("\nINSTALACIÓN COMPLETADA EXITOSAMENTE.")
			return nil
		},
	}
	cmd.Flags().StringVar(&o.server, "server", "", "servidor o IP de SQL Server")
	cmd.Flags().StringVar(&o.appDir, "app-dir", "", "directorio de la aplicación SIDC (ej. C:\\SIDC)")
	cmd.Flags().StringVar(&o.database, "database", "", "nombre de la base de datos (default SIDC)")
	cmd.Flags().StringVar(&o.sqlUser, "sql-user", "", "usuario SQL Server (default sidc)")
	cmd.Flags().StringVar(&o.appPass, "app-password", "", "contraseña SQL Server (o env AEGIS_SQL_PASSWORD)")
	cmd.Flags().BoolVar(&o.winAuth, "win-auth", false, "forzar uso de Windows Authentication")
	cmd.Flags().BoolVarP(&o.yes, "yes", "y", false, "confirmar automáticamente sin preguntar")
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
			// app_dir va vacío hasta que alguien diga dónde está SIDC; decirlo explícito
			// evita que un soporte lea "App OK" y crea que se midió la carpeta correcta.
			appDir := cfg.AppDir
			if appDir == "" {
				appDir = "(sin configurar)"
			}
			fmt.Fprintf(w, "app_dir=%s\n", appDir)
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

func newMenuCmd(res func() (config.Config, string, error), opciones func() opcionesTUI) *cobra.Command {
	return &cobra.Command{
		Use:   "menu",
		Short: "Menú interactivo (0=instalación completa 1=db 2=app 3=check 4-6=presets)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := res()
			return runMenu(cmd, cfg, path, err, opciones())
		},
	}
}

func newPatchLogosCmd(res func() (config.Config, string, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "patch-logos",
		Short: "Actualiza logos en ejecutables AegisSetup y reportes (.rpt) desde Fotos/Principal.jpg",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := res()
			if err != nil {
				return err
			}
			if cfg.AppDir == "" {
				return fmt.Errorf("app_dir sin configurar (elegí perfil o pasá --app-dir)")
			}
			out := func(s string) { fmt.Fprintln(cmd.OutOrStdout(), s) }
			return setup.PatchAppAndReports(cfg.AppDir, out)
		},
	}
}
