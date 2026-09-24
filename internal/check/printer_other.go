//go:build !windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import "context"

func checkPrinterEnvironment(ctx context.Context) []Result {
	return []Result{{"Spooler", true, "solo Windows: no aplica fuera de Windows"}, {"Impresora predeterminada", true, "solo Windows: no aplica fuera de Windows"}}
}
