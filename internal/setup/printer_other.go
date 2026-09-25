//go:build !windows

// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

// IsPrintToPDFInstalled en plataformas no Windows retorna true por no aplicar.
func IsPrintToPDFInstalled() bool {
	return true
}

// EnsurePrintToPDF en plataformas no Windows informa que no aplica y finaliza con éxito.
func EnsurePrintToPDF(out func(string)) error {
	if out != nil {
		out("Microsoft Print to PDF no aplica fuera de Windows")
	}
	return nil
}
