//go:build !windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import "context"

func openReportSample(ctx context.Context, repDir string) (bool, string) {
	return true, "muestra .rpt: verificación Crystal solo disponible en Windows"
}
