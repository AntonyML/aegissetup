//go:build !windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package precheck

// Fuera de Windows estas tres sondas no aplican. Se devuelven OK con el motivo
// dicho en el detalle, en vez de false: si devolvieran false, en la máquina de
// desarrollo Linux el checklist mostraría tres requisitos rojos que no existen
// y el operador aprendería a ignorar el checklist.
//
// El caso real de Linux es correr los tests y compilar; el instalador corre en
// Windows.

func esAdmin() (bool, string) { return true, "no aplica fuera de Windows" }

func maquinaApta() (bool, string) { return true, "no aplica fuera de Windows" }

func driverODBC(driver string) (bool, string) {
	return true, "no aplica fuera de Windows (el DSN se escribe en la PC destino)"
}
