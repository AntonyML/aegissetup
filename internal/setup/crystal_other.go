//go:build !windows

// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

func checkCrystalSys() []string { return nil }

// checkOCXSys no aplica fuera de Windows: no hay SysWOW64 que revisar.
func checkOCXSys() []string { return nil }
