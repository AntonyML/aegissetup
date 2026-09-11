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

// runMenu abre el TUI Bubble Tea que EJECUTA el flujo (1=db 2=app 3=check).
func runMenu(cmd *cobra.Command, cfg config.Config, path string, err error) error {
	if err != nil {
		return err
	}
	return runTUI(cmd.Context(), cfg, path, cmd.InOrStdin(), cmd.OutOrStdout())
}

func runTUI(ctx context.Context, cfg config.Config, path string, in io.Reader, out io.Writer) error {
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
	p := tea.NewProgram(ui.NewModel(cfg, path), opts...)
	_, err := p.Run()
	return err
}
