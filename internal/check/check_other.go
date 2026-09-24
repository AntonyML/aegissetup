//go:build !windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import "context"

func testMSDASQL32(ctx context.Context, connStr string) (bool, string) {
	return true, "verificación 32-bit MSDASQL solo disponible en Windows"
}
