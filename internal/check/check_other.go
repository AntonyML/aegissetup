//go:build !windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import "context"

func testMSDASQL32(ctx context.Context, connStr string) msdasqlProbe {
	return msdasqlProbe{
		ConnectionInfo: "verificación 32-bit MSDASQL solo disponible en Windows",
		DatesInfo:      "fechas no verificadas: MSDASQL 32-bit solo disponible en Windows",
	}
}
