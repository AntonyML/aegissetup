//go:build !windows

// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

func checkCrystalSys() []string { return nil }

// checkOCXSys no aplica fuera de Windows: no hay SysWOW64 que revisar.
func checkOCXSys() []string { return nil }

// RegisterCOM no aplica fuera de Windows: sin registro COM no hay componentes que
// registrar. Devuelve nil para que la instalación no finja una falla que no existe.
func RegisterCOM(string) error { return nil }

// PatchCrystalODBCBridge no aplica fuera de Windows.
func PatchCrystalODBCBridge(string, string, func(string)) error { return nil }
