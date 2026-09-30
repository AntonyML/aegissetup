// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"strings"
	"testing"

	"aegis-setup/internal/config"
)

func TestDSNInstalacionYReparacionUsanIdiomaDMY(t *testing.T) {
	cfg := config.Config{Driver: " SQL Server ", Server: " localhost,1433 ", Database: " SIDC ", UseWinAuth: true}
	settings := dsnConnectionSettings(cfg)
	if settings["Language"] != "Spanish" {
		t.Fatalf("la instalación y reparación deben interpretar día/mes/año; Language=%q", settings["Language"])
	}
	if settings["Server"] != "localhost,1433" || settings["Database"] != "SIDC" {
		t.Fatal("los valores de conexión deben conservar la sanitización")
	}
}

func TestDSNRechazaIdiomaIncompatibleOAusente(t *testing.T) {
	cfg := config.Config{Driver: "SQL Server", Server: "localhost,1433", Database: "SIDC", UseWinAuth: true}
	for _, language := range []string{"us_english", "", "French"} {
		t.Run(language, func(t *testing.T) {
			values := dsnConnectionSettings(cfg)
			values["Language"] = language
			values["Trusted_Connection"] = "Yes"
			ok, diffs := validateDSNSettings(cfg, values, "")
			if ok || !strings.Contains(strings.Join(diffs, " "), "Language") {
				t.Fatalf("el DSN debe requerir reparación por idioma; ok=%v, diferencias=%v", ok, diffs)
			}
		})
	}
}

func TestDSNCanonicoEsValidoYConservaLasOtrasComprobaciones(t *testing.T) {
	cfg := config.Config{Driver: "SQL Server", Server: "localhost,1433", Database: "SIDC", UseWinAuth: true}
	values := dsnConnectionSettings(cfg)
	values["Language"] = "spanish"
	values["Trusted_Connection"] = "Yes"
	if ok, diffs := validateDSNSettings(cfg, values, ""); !ok {
		t.Fatalf("el idioma canónico no debe depender de mayúsculas: %v", diffs)
	}
	for _, field := range []string{"Driver", "Server", "Database", "Trusted_Connection"} {
		t.Run(field, func(t *testing.T) {
			broken := dsnConnectionSettings(cfg)
			broken["Trusted_Connection"] = "Yes"
			broken[field] = "valor incompatible"
			if ok, diffs := validateDSNSettings(cfg, broken, ""); ok || !strings.Contains(strings.Join(diffs, " "), field) {
				t.Fatalf("debe conservarse la comprobación de %s: %v", field, diffs)
			}
		})
	}
}
