//go:build !windows

// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

// isAdminOS fuera de Windows dice que sí a propósito: no hay UAC ni permisos que
// pedir, y el instalador no corre ahí. Decir que no dejaría al CI de Linux pidiendo
// una elevación imposible en cada prueba.
func isAdminOS() bool { return true }

// launchElevated no hace nada fuera de Windows. El flujo de permisos es de Windows;
// esto existe para que el paquete compile y las pruebas corran en Linux.
func launchElevated(string, []string, bool) (int, error) { return 0, nil }
