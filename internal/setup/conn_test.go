// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"aegis-setup/internal/config"
)

func TestIsServerLocal(t *testing.T) {
	locals := []string{
		"localhost",
		"localhost,1433",
		"localhost,14333",
		"127.0.0.1",
		"127.0.0.1,1433",
		"::1",
		".",
		`.\SQLEXPRESS`,
		`localhost\SQLEXPRESS`,
	}
	for _, s := range locals {
		if !IsServerLocal(s) {
			t.Errorf("IsServerLocal(%q) = false, want true", s)
		}
	}

	remotes := []string{
		"192.168.2.145",
		"192.168.2.145,1433",
		"10.0.0.15",
		"SERVER_REMOTO",
		`SERVER_REMOTO\SQLEXPRESS`,
	}
	for _, s := range remotes {
		if IsServerLocal(s) {
			t.Errorf("IsServerLocal(%q) = true, want false", s)
		}
	}
}

func TestSQLConnStringNoSilentFallback(t *testing.T) {
	cfg := config.Config{
		Server:     "192.168.2.145,1433",
		Database:   "SIDC",
		UseWinAuth: false,
		SQLUser:    "sidc",
	}

	// Sin clave: NUNCA debe contener trusted_connection=yes
	connStr := SQLConnString(cfg, "")
	if strings.Contains(connStr, "trusted+connection=yes") {
		t.Errorf("SQLConnString con UseWinAuth=false y pass vacía cayó a trusted connection: %q", connStr)
	}
	if !strings.Contains(connStr, "sqlserver://sidc@192.168.2.145:1433") {
		t.Errorf("SQLConnString inesperada: %q", connStr)
	}
}

func TestSQLConnStringSpecialCharactersSanitized(t *testing.T) {
	cfg := config.Config{
		Server:     "192.168.2.145,1433",
		Database:   "SIDC",
		UseWinAuth: false,
		SQLUser:    "sidc",
	}

	// Clave real con caracteres especiales y espacio al final que debe ser sanitizado
	rawPass := "  dwHmNy+rx+Ehu#q%EwW*vBrp  "
	cleanPass := strings.TrimSpace(rawPass)
	connStr := SQLConnString(cfg, rawPass)

	// El espacio inicial y final debe eliminarse
	if strings.Contains(connStr, " ") {
		t.Errorf("SQLConnString contiene espacios no sanitizados: %q", connStr)
	}

	// La URL debe ser parseable y la clave decodificada debe coincidir con cleanPass
	u, err := url.Parse(connStr)
	if err != nil {
		t.Fatalf("URL inválida en SQLConnString: %v", err)
	}
	pass, _ := u.User.Password()
	if pass != cleanPass {
		t.Errorf("Password decodificada = %q, want %q", pass, cleanPass)
	}
}

func TestSQLConnStringWindowsAuth(t *testing.T) {
	cfg := config.Config{
		Server:     "CONTABILIDAD",
		Database:   "SIDC",
		UseWinAuth: true,
	}
	connStr := SQLConnString(cfg, "ignorado")
	if !strings.Contains(connStr, "trusted+connection=yes") {
		t.Errorf("SQLConnString con Windows Auth no incluye trusted connection: %q", connStr)
	}
	if strings.Contains(connStr, "sidc:") {
		t.Errorf("SQLConnString con Windows Auth no debe incluir usuario SQL: %q", connStr)
	}
}

func TestDetectBestAuthWorkgroupRemoteServer(t *testing.T) {
	// 192.168.2.145 es remoto. En una máquina fuera de dominio (o detectada como WORKGROUP),
	// NUNCA debe seleccionar Windows Auth
	winAuth, reason := DetectBestAuth(context.Background(), "192.168.2.145,1433", "SIDC")
	inDomain, _, _ := DetectDomain()
	if !inDomain {
		if winAuth {
			t.Errorf("DetectBestAuth seleccionó Windows Auth en máquina fuera de dominio contra servidor remoto")
		}
		if !strings.Contains(reason, "SQL Server") {
			t.Errorf("Razón inesperada: %q", reason)
		}
	}
}

func TestSanitizeSQLError(t *testing.T) {
	cfg := config.Config{
		Server:  "192.168.2.145",
		SQLUser: "sidc",
	}

	// Error 18452 Untrusted Domain
	err18452 := errors.New("mssql: Login failed. The login is from an untrusted domain and cannot be used with Windows authentication. (18452)")
	sErr := SanitizeSQLError(err18452, cfg)
	if !strings.Contains(sErr.Error(), "no pertenece al dominio") {
		t.Errorf("SanitizeSQLError 18452 no explica el problema de dominio: %v", sErr)
	}

	// Error 18456 Login Failed
	err18456 := errors.New("mssql: Login failed for user 'sidc'. (18456)")
	sErr = SanitizeSQLError(err18456, cfg)
	if !strings.Contains(sErr.Error(), "Verifique la contraseña") {
		t.Errorf("SanitizeSQLError 18456 no menciona verificar contraseña: %v", sErr)
	}

	// Timeout
	errTimeout := errors.New("dial tcp 192.168.2.145:1433: i/o timeout")
	sErr = SanitizeSQLError(errTimeout, cfg)
	if !strings.Contains(sErr.Error(), "no se pudo alcanzar") {
		t.Errorf("SanitizeSQLError timeout no menciona conectividad: %v", sErr)
	}
}
