// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"fmt"
	"strings"

	"aegis-setup/internal/config"
)

// dsnConnectionSettings reúne los valores que instala y repara Aegis.
func dsnConnectionSettings(cfg config.Config) map[string]string {
	return map[string]string{
		"Driver":   driverDLL(strings.TrimSpace(cfg.Driver)),
		"Server":   strings.TrimSpace(cfg.Server),
		"Database": strings.TrimSpace(cfg.Database),
		// SIDC envía fechas día/mes/año como texto desde VB6.
		"Language": "Spanish",
	}
}

func driverDLL(driver string) string {
	switch driver {
	case "SQL Server":
		return `C:\Windows\SysWOW64\SQLSRV32.dll`
	default:
		return `C:\Windows\SysWOW64\msodbcsql17.dll`
	}
}

// validateDSNSettings compara el contenido leído sin exponer la contraseña.
func validateDSNSettings(cfg config.Config, values map[string]string, appPass string) (bool, []string) {
	var diffs []string
	expected := dsnConnectionSettings(cfg)
	if !strings.EqualFold(values["Driver"], expected["Driver"]) {
		diffs = append(diffs, fmt.Sprintf("Driver: esperado %q, actual %q", expected["Driver"], values["Driver"]))
	}
	if values["Server"] != expected["Server"] {
		diffs = append(diffs, fmt.Sprintf("Server: esperado %q, actual %q", expected["Server"], values["Server"]))
	}
	if values["Database"] != expected["Database"] {
		diffs = append(diffs, fmt.Sprintf("Database: esperada %q, actual %q", expected["Database"], values["Database"]))
	}
	if !strings.EqualFold(values["Language"], expected["Language"]) {
		diffs = append(diffs, fmt.Sprintf("Language: esperado %q (día/mes/año), actual %q", expected["Language"], values["Language"]))
	}
	expectedTrusted := "No"
	if cfg.UseWinAuth {
		expectedTrusted = "Yes"
	}
	if !strings.EqualFold(values["Trusted_Connection"], expectedTrusted) {
		diffs = append(diffs, fmt.Sprintf("Trusted_Connection: esperada %q, actual %q", expectedTrusted, values["Trusted_Connection"]))
	}
	if !cfg.UseWinAuth {
		expectedUser := strings.TrimSpace(cfg.SQLUser)
		if expectedUser == "" {
			expectedUser = "sidc"
		}
		if values["LastUser"] != expectedUser {
			diffs = append(diffs, fmt.Sprintf("LastUser: esperado %q, actual %q", expectedUser, values["LastUser"]))
		}
		if pwd := strings.TrimSpace(appPass); pwd != "" && values["PWD"] != pwd {
			diffs = append(diffs, "PWD: la clave almacenada en el registro tiene discrepancias o espacios residuales")
		}
	}
	return len(diffs) == 0, diffs
}
