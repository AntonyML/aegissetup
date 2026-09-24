//go:build !windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"context"

	"aegis-setup/internal/config"
)

func openReportSample(ctx context.Context, repDir string, cfg config.Config, appPass, appConnStr string) (bool, string) {
	return true, reportUnderCheck + ": verificación Crystal solo disponible en Windows"
}
