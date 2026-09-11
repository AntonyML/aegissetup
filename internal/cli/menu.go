// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"context"
	"io"

	"aegis-setup/internal/config"
	"aegis-setup/internal/ui"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
)

// opcionesTUI son los datos que el operador ya dijo en la línea de comandos. Van juntos
// porque los dos son lo mismo: algo de esta PC que el TUI no tiene que volver a preguntar.
type opcionesTUI struct {
	server string // --server: nombre del servidor para el preset "prod server"
	appDir string // --app-dir: carpeta donde está SIDC
}

// runMenu abre el TUI Bubble Tea que EJECUTA el flujo (0=instalación completa,
// 1=db 2=app 3=check 4-6=presets).
func runMenu(cmd *cobra.Command, cfg config.Config, path string, err error, o opcionesTUI) error {
	if err != nil {
		return err
	}
	return runTUI(cmd.Context(), cfg, path, o, cmd.InOrStdin(), cmd.OutOrStdout())
}

// primeraVez dice si hay que preguntar el perfil antes del menú. No hay config en
// disco = todavía no está dicho en qué PC estamos. Lo mira el CLI y no el modelo
// porque el que sabe de archivos es este lado.
func primeraVez(path string) bool { return !existeArchivo(path) }

// modeloInicial arma el modelo del TUI con lo que el CLI ya resolvió. Es una función
// aparte de runTUI para poder medir el camino completo bandera -> modelo -> config.json
// sin abrir una terminal.
func modeloInicial(cfg config.Config, path string, o opcionesTUI) ui.Model {
	return ui.NewModel(cfg, path).
		SetPresetServer(o.server).
		SetPresetAppDir(o.appDir).
		SetPrimeraVez(primeraVez(path))
}

func runTUI(ctx context.Context, cfg config.Config, path string, o opcionesTUI, in io.Reader, out io.Writer) error {
	var opts []tea.ProgramOption
	if in != nil {
		opts = append(opts, tea.WithInput(in))
	}
	if out != nil {
		opts = append(opts, tea.WithOutput(out))
	}
	if ctx != nil {
		opts = append(opts, tea.WithContext(ctx))
	}
	model := modeloInicial(cfg, path, o)
	p := tea.NewProgram(model, opts...)
	_, err := p.Run()
	return err
}
