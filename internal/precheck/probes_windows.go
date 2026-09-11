//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package precheck

import (
	"os"

	"aegis-setup/internal/setup"

	"golang.org/x/sys/windows/registry"
)

// esAdmin mira el token del proceso. Casi todo lo que hace Aegis toca HKLM y
// SysWOW64, así que sin elevación el operador vería errores de permisos recién
// al final, después de minutos de RESTORE.
//
// La consulta vive en setup.IsAdmin, que es la misma que usa setup.Elevate para
// decidir si hay que relanzar: dos formas de preguntar lo mismo terminan en un
// checklist que dice "falta admin" mientras la elevación no hace nada (o al revés).
func esAdmin() (bool, string) {
	if setup.IsAdmin() {
		return true, "sesión elevada"
	}
	return false, "sesión sin elevar"
}

// maquinaApta verifica que exista el subsistema de 32 bits. Una PC con Windows
// de 32 bits no tiene SysWOW64 y SIDC (VB6) no corre ahí de ninguna forma.
func maquinaApta() (bool, string) {
	if _, err := os.Stat(`C:\Windows\SysWOW64`); err != nil {
		return false, "no se encontró C:\\Windows\\SysWOW64"
	}
	return true, "Windows de 64 bits con subsistema de 32 bits"
}

// driverODBC revisa la lista de drivers de 32 bits. Se lee WOW6432Node a
// propósito: un proceso de 64 bits que leyera ODBCINST.INI sin ese prefijo
// vería los drivers de 64 bits, y el DSN de SIDC es de 32.
func driverODBC(driver string) (bool, string) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\WOW6432Node\ODBC\ODBCINST.INI\ODBC Drivers`, registry.QUERY_VALUE)
	if err != nil {
		return false, "no se pudo leer la lista de drivers ODBC de 32 bits"
	}
	defer k.Close()
	estado, _, err := k.GetStringValue(driver)
	if err != nil {
		return false, "no está instalado entre los drivers ODBC de 32 bits"
	}
	return true, "instalado en ODBC de 32 bits (" + estado + ")"
}
