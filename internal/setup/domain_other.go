//go:build !windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

// DetectDomain simula detección de dominio en plataformas no-Windows.
func DetectDomain() (bool, string, error) {
	return false, "WORKGROUP", nil
}
