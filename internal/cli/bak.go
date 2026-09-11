// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"fmt"
	"io"

	"aegis-setup/internal/config"

	"github.com/spf13/cobra"
)

// repoReleases es el Release público donde vive el respaldo. Aegis NO lo descarga: la
// decisión es que el operador lo baje a mano, así que lo único que el programa puede
// hacer es decirle la URL exacta, el nombre del asset y la carpeta donde dejarlo. Sin
// esto, el operador tiene que preguntar por chat dónde está el .bak.
const repoReleases = "https://github.com/AntonyML/aegissetup/releases"

// bakAssetName es el nombre con el que el Release publica el respaldo. Es estable a
// propósito: el workflow copia el .bak a este nombre antes de adjuntarlo, así la URL
// .../releases/latest/download/SIDC_2014.bak no cambia entre releases aunque el archivo
// de origen lleve la fecha en el nombre.
const bakAssetName = "SIDC_2014.bak"

// newBakCmd arma "aegis bak". La búsqueda del respaldo se inyecta para poder probar la
// salida sin un .bak de 39 MB en disco.
func newBakCmd(res func() (config.Config, string, error), buscar func(config.Config) (string, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "bak",
		Short: "Dónde conseguir y dejar el .bak de SIDC (Aegis no lo descarga)",
		// Igual que check y checklist: el reporte es la salida, no un error de uso.
		SilenceUsage: true,
		Long: `Dice de dónde bajar el respaldo de SIDC y en qué carpeta dejarlo.

El .bak pesa ~39 MB, así que no viaja dentro de Aegis.exe: se publica como asset del
Release y lo bajás vos. Aegis NO descarga nada, ni siquiera con este comando.

Sale con código 3 si todavía no hay respaldo (Setup DB no puede correr sin él) y con 0
si ya lo encontró.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := res()
			if err != nil {
				return err
			}
			bak, errBak := buscar(cfg)
			return escribirBak(cmd.OutOrStdout(), cfg, bak, errBak)
		},
	}
}

// escribirBak imprime la ruta completa: de dónde bajar el respaldo, dónde dejarlo y si
// ya está. Devuelve ErrCheckFail cuando falta, para que un script pueda usarlo de guarda
// antes del paso 1 (mismo código que "checklist": la máquina todavía no está lista).
func escribirBak(w io.Writer, cfg config.Config, bak string, errBak error) error {
	fmt.Fprintln(w, "== AEGIS BAK ==")
	fmt.Fprintln(w, "Aegis NO descarga el respaldo: lo bajás vos del Release y lo dejás en la carpeta de abajo.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "De dónde bajarlo (asset del Release):")
	fmt.Fprintf(w, "  %s/latest/download/%s\n", repoReleases, bakAssetName)
	fmt.Fprintf(w, "  (o elegí el Release que corresponda en %s)\n", repoReleases)
	fmt.Fprintln(w)
	// El orden importa: el primero es el que gana, y es el que Setup DB va a mirar
	// primero. Decir "la carpeta" en singular cuando hay tres candidatas haría que el
	// operador deje el archivo en una que pierde contra otra.
	fmt.Fprintln(w, "Dónde dejarlo (Aegis lo busca en este orden, el primero gana):")
	for i, dir := range config.BackupDirs(cfg, exeDir()) {
		fmt.Fprintf(w, "  %d. %s\n", i+1, dir)
	}
	fmt.Fprintln(w)

	if bak == "" {
		fmt.Fprintln(w, "Estado: FALTA el respaldo — Setup DB no puede restaurar nada sin él.")
		if errBak != nil {
			fmt.Fprintf(w, "         %v\n", errBak)
		}
		fmt.Fprintf(w, "== FALTA el respaldo .bak: bajalo del Release y dejalo en %s. ==\n", config.DirBackups())
		return fmt.Errorf("%w: falta el respaldo .bak", ErrCheckFail)
	}

	fmt.Fprintf(w, "Estado: LISTO — Setup DB va a restaurar %s.\n", bak)
	return nil
}
