//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"fmt"
	"strings"

	"aegis-setup/internal/config"
	"aegis-setup/internal/securestore"

	"golang.org/x/sys/windows/registry"
)

// WriteDSN crea/actualiza el System DSN 32-bit SIDC_SQL y realiza readback verification.
func WriteDSN(cfg config.Config, appPass string, savePWD bool, out func(string)) error {
	base := `SOFTWARE\WOW6432Node\ODBC\ODBC.INI\` + cfg.DsnName
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, base, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("registro HKLM (corre como Admin): %w", err)
	}
	defer k.Close()

	// Sanitizar entradas
	driverPath := driverDLL(strings.TrimSpace(cfg.Driver))
	server := strings.TrimSpace(cfg.Server)
	database := strings.TrimSpace(cfg.Database)
	user := strings.TrimSpace(cfg.SQLUser)
	if user == "" {
		user = "sidc"
	}
	pwd := strings.TrimSpace(appPass)
	if pwd == "" {
		pwd = securestore.ResolvePassword("", func() string { return DSNPassword(cfg.DsnName) })
	}

	set := func(n, v string) error { return k.SetStringValue(n, v) }
	if err := set("Driver", driverPath); err != nil {
		return err
	}
	if err := set("Server", server); err != nil {
		return err
	}
	if err := set("Database", database); err != nil {
		return err
	}
	if err := set("Language", "us_english"); err != nil {
		return err
	}

	if cfg.UseWinAuth {
		if err := set("Trusted_Connection", "Yes"); err != nil {
			return err
		}
		_ = k.DeleteValue("LastUser")
		_ = k.DeleteValue("PWD")
	} else {
		if err := set("Trusted_Connection", "No"); err != nil {
			return err
		}
		if err := set("LastUser", user); err != nil {
			return err
		}
		if savePWD || pwd != "" {
			if pwd != "" {
				if err := set("PWD", pwd); err != nil {
					return err
				}
				// Persistir también en almacén seguro local
				_ = securestore.SavePassword(pwd)
			}
		}
	}

	if err := ensureUserDSNEntry(cfg.DsnName); err != nil {
		return err
	}

	// Readback verification: verificar byte a byte contra el registro
	raw, err := ReadDSNRaw(cfg.DsnName)
	if err != nil {
		return fmt.Errorf("readback DSN falló: %w", err)
	}
	if raw["Driver"] != driverPath {
		return fmt.Errorf("readback DSN: Driver esperado %q, encontrado %q", driverPath, raw["Driver"])
	}
	if raw["Server"] != server {
		return fmt.Errorf("readback DSN: Server esperado %q, encontrado %q", server, raw["Server"])
	}
	if raw["Database"] != database {
		return fmt.Errorf("readback DSN: Database esperada %q, encontrada %q", database, raw["Database"])
	}
	expectedTrusted := "No"
	if cfg.UseWinAuth {
		expectedTrusted = "Yes"
	}
	if !strings.EqualFold(raw["Trusted_Connection"], expectedTrusted) {
		return fmt.Errorf("readback DSN: Trusted_Connection esperada %q, encontrada %q", expectedTrusted, raw["Trusted_Connection"])
	}
	if !cfg.UseWinAuth {
		if raw["LastUser"] != user {
			return fmt.Errorf("readback DSN: LastUser esperado %q, encontrado %q", user, raw["LastUser"])
		}
		if (savePWD || pwd != "") && pwd != "" && raw["PWD"] != pwd {
			return fmt.Errorf("readback DSN: PWD guardado no coincide con el valor esperado")
		}
	}

	if cfg.UseWinAuth {
		out("DSN OK (Windows Auth): " + cfg.DsnName + " -> " + server)
	} else {
		out("DSN OK (SQL Auth): " + cfg.DsnName + " -> " + server + " UID=" + user)
	}
	return nil
}

func driverDLL(driver string) string {
	switch driver {
	case "SQL Server":
		return `C:\Windows\SysWOW64\SQLSRV32.dll`
	default:
		return `C:\Windows\SysWOW64\msodbcsql17.dll`
	}
}

func ensureUserDSNEntry(dsn string) error {
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\ODBC\ODBC.INI\ODBC Data Sources`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(dsn, "SQL Server")
}

// ReadDSNRaw lee el DSN 32-bit sin enmascarar valores.
func ReadDSNRaw(dsn string) (map[string]string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\ODBC\ODBC.INI\`+dsn, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer k.Close()
	names, err := k.ReadValueNames(0)
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, n := range names {
		v, _, err := k.GetStringValue(n)
		if err == nil {
			m[n] = v
		}
	}
	return m, nil
}

// ReadDSN lee el DSN 32-bit para check/dashboard con PWD enmascarada.
func ReadDSN(dsn string) (map[string]string, error) {
	m, err := ReadDSNRaw(dsn)
	if err != nil {
		return nil, err
	}
	if _, ok := m["PWD"]; ok {
		m["PWD"] = "***"
	}
	return m, nil
}

// DSNPassword devuelve la clave guardada en el DSN ODBC de 32 bits, sanitizada.
func DSNPassword(dsn string) string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\ODBC\ODBC.INI\`+dsn, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	pwd, _, err := k.GetStringValue("PWD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(pwd)
}

// ValidateDSN verifica si el DSN existente coincide byte a byte con la configuración esperada.
func ValidateDSN(cfg config.Config, appPass string) (bool, []string) {
	m, err := ReadDSNRaw(cfg.DsnName)
	if err != nil {
		return false, []string{"el DSN " + cfg.DsnName + " no existe en el registro"}
	}
	var diffs []string
	expectedDriver := driverDLL(strings.TrimSpace(cfg.Driver))
	if !strings.EqualFold(m["Driver"], expectedDriver) {
		diffs = append(diffs, fmt.Sprintf("Driver: esperado %q, actual %q", expectedDriver, m["Driver"]))
	}
	expectedServer := strings.TrimSpace(cfg.Server)
	if m["Server"] != expectedServer {
		diffs = append(diffs, fmt.Sprintf("Server: esperado %q, actual %q", expectedServer, m["Server"]))
	}
	expectedDB := strings.TrimSpace(cfg.Database)
	if m["Database"] != expectedDB {
		diffs = append(diffs, fmt.Sprintf("Database: esperada %q, actual %q", expectedDB, m["Database"]))
	}
	expectedTrusted := "No"
	if cfg.UseWinAuth {
		expectedTrusted = "Yes"
	}
	if !strings.EqualFold(m["Trusted_Connection"], expectedTrusted) {
		diffs = append(diffs, fmt.Sprintf("Trusted_Connection: esperada %q, actual %q", expectedTrusted, m["Trusted_Connection"]))
	}
	if !cfg.UseWinAuth {
		expectedUser := strings.TrimSpace(cfg.SQLUser)
		if expectedUser == "" {
			expectedUser = "sidc"
		}
		if m["LastUser"] != expectedUser {
			diffs = append(diffs, fmt.Sprintf("LastUser: esperado %q, actual %q", expectedUser, m["LastUser"]))
		}
		pwd := strings.TrimSpace(appPass)
		if pwd == "" {
			pwd = securestore.ResolvePassword("", nil)
		}
		if pwd != "" && m["PWD"] != pwd {
			diffs = append(diffs, "PWD: la clave almacenada en el registro tiene discrepancias o espacios residuales")
		}
	}
	return len(diffs) == 0, diffs
}

// RepairDSN repara el DSN escribiendo la configuración canónica limpia.
func RepairDSN(cfg config.Config, appPass string, out func(string)) error {
	out("Corrigiendo DSN " + cfg.DsnName + "...")
	return WriteDSN(cfg, appPass, !cfg.UseWinAuth, out)
}
