// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"fmt"
	"io"

	"aegis-setup/internal/config"
	"aegis-setup/internal/setup"

	"github.com/spf13/cobra"
)

// newUninstallCmd arma "aegis uninstall".
//
// El default es NO borrar: sin --yes solamente imprime el plan. Un desinstalador que
// ejecuta con solo escribir el nombre del comando es un pie de banco, sobre todo en una
// herramienta que se corre en la PC de producción de un cliente.
func newUninstallCmd(res func() (config.Config, string, error)) *cobra.Command {
	var yes, keepDSN bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Saca de la PC lo que instaló Aegis (no toca SIDC ni la base)",
		// Igual que check y checklist: el reporte es la salida, no un error de uso.
		SilenceUsage: true,
		Long: `Saca de la PC lo que instaló Aegis: config, datos de máquina y DSN.

Sin --yes solo muestra el plan y no borra nada. Con --yes ejecuta.
Este comando pide permisos de administrador: borra el DSN de HKLM.

Si querés que SIDC siga funcionando y solo querés sacar Aegis, usá --keep-dsn. El DSN
SIDC_SQL es lo que usa la app para llegar a la base: sin él, SIDC falla con error 3146.

NO toca: la base SIDC, el login app del motor, ni la carpeta de SIDC (reportes, fotos,
ejecutable). Nada de eso lo puso Aegis. Tampoco borra los OCX ni el runtime de Crystal de
SysWOW64: los comparte Windows y otros programas VB6, así que sacarlos puede romper
software que no es SIDC.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := res()
			if err != nil {
				return err
			}
			// exeDir() y no el segundo valor de res(): ese es la ruta del config.json
			// resuelto, no la carpeta del ejecutable. Usarlo armaba
			// "...\config.json\config.json" y el paso nunca borraba nada.
			pasos := setup.PlanUninstall(cfg, exeDir())
			if keepDSN {
				pasos = setup.SinDSN(pasos)
			}
			return escribirUninstall(cmd.OutOrStdout(), pasos, yes)
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Ejecuta el borrado de verdad (sin esto solo muestra el plan)")
	cmd.Flags().BoolVar(&keepDSN, "keep-dsn", false, "No borra el DSN: SIDC sigue andando después de sacar Aegis")
	return cmd
}

// dejarEnPaz son los pasos que la desinstalación NO da. Va como dato y no como texto
// suelto para que el motivo esté pegado a lo que se deja: "no se toca" sin el por qué se
// lee como una tarea pendiente.
var dejarEnPaz = []struct{ Que, PorQue string }{
	{
		"Los OCX y el runtime de Crystal en SysWOW64",
		"no se tocan: los comparte Windows y otros programas VB6, borrarlos puede romper software que no es SIDC",
	},
	{
		"La base SIDC y el login app",
		"los creó SQL, no Aegis",
	},
	{
		"La carpeta de SIDC (reportes, fotos, ejecutable)",
		"es del cliente",
	},
}

// escribirUninstall imprime el plan y, si yes es true, lo ejecuta. Devuelve error solo
// cuando el borrado de verdad dejó pasos fallados: un plan nunca es un error.
func escribirUninstall(w io.Writer, pasos []setup.UninstallStep, yes bool) error {
	fmt.Fprintln(w, "== AEGIS UNINSTALL ==")
	fmt.Fprintln(w, "Se saca SOLO lo que instaló Aegis. SIDC, la base y la carpeta de la app quedan igual.")
	fmt.Fprintln(w)

	// Aviso antes de la lista y no después: si el operador ya tiene el respaldo en otro
	// lado no le importa, y si no lo tiene, esto es lo único que le evita perder el
	// punto de restauración.
	if n, bytes := setup.FaltaElRespaldo(pasos); n > 0 {
		fmt.Fprintf(w, "OJO: este plan se lleva %d respaldo(s) .bak (%s).\n", n, setup.HumanBytes(bytes))
		fmt.Fprintln(w, "     Si no lo tenés en otro lado, bajalo de nuevo con \"aegis bak\" si volvés a instalar.")
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Se va a borrar:")
	for i, p := range pasos {
		fmt.Fprintf(w, "  %d. %s\n", i+1, p.Title)
		fmt.Fprintf(w, "     %s%s\n", p.Path, medida(p))
	}
	fmt.Fprintln(w)

	// El DSN es lo que mantiene viva a la app. Si se va, SIDC deja de llegar a la base y
	// el operador va a culpar a la desinstalación, con razón.
	if tieneDSN(pasos) {
		fmt.Fprintln(w, "OJO: sacar el DSN deja a SIDC sin llegar a la base (error 3146).")
		fmt.Fprintln(w, "     Si querés que SIDC siga andando y solo sacar Aegis, corré con --keep-dsn.")
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Queda en la máquina (a propósito):")
	for _, d := range dejarEnPaz {
		fmt.Fprintf(w, "  - %s: %s.\n", d.Que, d.PorQue)
	}
	fmt.Fprintln(w)

	if !yes {
		fmt.Fprintln(w, "== PLAN: no se borró nada. Volvé a correrlo con --yes para ejecutar. ==")
		return nil
	}

	fallos := setup.Desinstalar(pasos, func(s string) { fmt.Fprintln(w, s) })
	if len(fallos) > 0 {
		fmt.Fprintf(w, "== Terminó con %d paso(s) fallado(s). ==\n", len(fallos))
		// Error común y no ErrCheckFail: "la máquina no está lista" es del diagnóstico.
		// Acá el comando corrió y no pudo terminar, que es otra cosa.
		return fmt.Errorf("no se pudo desinstalar todo: %d pasos fallados", len(fallos))
	}
	fmt.Fprintln(w, "== Listo: Aegis quedó desinstalado. ==")
	return nil
}

// tieneDSN dice si el plan se lleva el DSN. Se pregunta por el plan y no por el flag para
// que el aviso también salga cuando el plan viene armado por otro camino.
func tieneDSN(pasos []setup.UninstallStep) bool {
	for _, p := range pasos {
		if p.Kind == setup.StepDSN {
			return true
		}
	}
	return false
}

// medida muestra el tamaño de lo que se va a borrar y, en la entrada del DSN, el valor.
// La clave sola no alcanza: "ODBC Data Sources" la comparten todos los DSN de la máquina,
// así que sin el nombre del valor el operador no sabe qué se lleva.
func medida(p setup.UninstallStep) string {
	var s string
	if p.Value != "" {
		s = " \\ " + p.Value
	}
	if p.Files > 0 {
		s += fmt.Sprintf("  (%d archivos, %s)", p.Files, setup.HumanBytes(p.Bytes))
	}
	return s
}
