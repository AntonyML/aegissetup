//go:build windows
// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

import (
	"fmt"

	"aegis-setup/internal/config"

	"golang.org/x/sys/windows/registry"
)

// WriteDSN crea/actualiza el System DSN 32-bit SIDC_SQL.
// En prod local/server usa Windows Auth (Trusted_Connection=Yes).
// En dev docker usa SQL Auth y guarda LastUser (+PWD solo dev, nunca prod).
func WriteDSN(cfg config.Config, appPass string, savePWD bool, out func(string)) error {
	base := `SOFTWARE\WOW6432Node\ODBC\ODBC.INI\` + cfg.DsnName
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, base, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("registro HKLM (corre como Admin): %w", err)
	}
	defer k.Close()
	set := func(n, v string) error { return k.SetStringValue(n, v) }
	if err := set("Driver", driverDLL(cfg.Driver)); err != nil {
		return err
	}
	if err := set("Server", cfg.Server); err != nil {
		return err
	}
	if err := set("Database", cfg.Database); err != nil {
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
		out("DSN OK (Windows Auth): " + cfg.DsnName + " -> " + cfg.Server)
		return ensureUserDSNEntry(cfg.DsnName)
	}
	if err := set("Trusted_Connection", "No"); err != nil {
		return err
	}
	if err := set("LastUser", cfg.SQLUser); err != nil {
		return err
	}
	if savePWD {
		if appPass == "" {
			return fmt.Errorf("falta AEGIS_SQL_PASSWORD para guardar PWD del DSN (solo dev)")
		}
		if err := set("PWD", appPass); err != nil {
			return err
		}
	}
	out("DSN OK (SQL Auth): " + cfg.DsnName + " -> " + cfg.Server + " UID=" + cfg.SQLUser)
	return ensureUserDSNEntry(cfg.DsnName)
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

// ReadDSN lee el DSN 32-bit para check/dashboard.
func ReadDSN(dsn string) (map[string]string, error) {
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
			if n == "PWD" {
				m[n] = "***"
			} else {
				m[n] = v
			}
		}
	}
	return m, nil
}
