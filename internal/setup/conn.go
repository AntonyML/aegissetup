// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"aegis-setup/internal/config"
	"aegis-setup/internal/securestore"

	_ "github.com/microsoft/go-mssqldb"
)

// IsServerLocal determina si el host especificado apunta a la máquina local.
func IsServerLocal(server string) bool {
	s := strings.TrimSpace(server)
	if s == "::1" || strings.HasPrefix(s, "::1,") || strings.HasPrefix(s, "[::1]") {
		return true
	}
	host, _ := SepararHostPuerto(server)
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" || h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "." {
		return true
	}
	if strings.HasPrefix(h, `.\`) || strings.HasPrefix(h, `./`) {
		return true
	}
	if strings.Contains(h, `\`) {
		parts := strings.Split(h, `\`)
		first := strings.ToLower(strings.TrimSpace(parts[0]))
		if first == "" || first == "localhost" || first == "127.0.0.1" || first == "." {
			return true
		}
		if hostname, err := os.Hostname(); err == nil && strings.EqualFold(first, hostname) {
			return true
		}
	}
	if hostname, err := os.Hostname(); err == nil && strings.EqualFold(h, hostname) {
		return true
	}
	return false
}

// DetectBestAuth evalúa el entorno de red y el servidor para seleccionar
// de forma automática y segura el método de autenticación viable.
// NUNCA asume Windows Auth por defecto en servidores remotos fuera de dominio.
func DetectBestAuth(ctx context.Context, server, database string) (bool, string) {
	isLocal := IsServerLocal(server)
	inDomain, domainName, _ := DetectDomain()

	if !isLocal && !inDomain {
		groupName := domainName
		if groupName == "" {
			groupName = "WORKGROUP"
		}
		return false, fmt.Sprintf("Equipo en grupo de trabajo (%s) con servidor remoto (%s). Windows Auth requiere dominio; usando autenticación SQL Server.", groupName, server)
	}

	testCfg := config.Config{
		Server:     server,
		Database:   database,
		UseWinAuth: true,
	}
	tCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	if err := TestSQLConn(tCtx, testCfg, ""); err == nil {
		return true, "Autenticación Windows verificada exitosamente con el servidor."
	}
	if isLocal || inDomain {
		return true, "Equipo en dominio o servidor local; usando Windows Authentication."
	}
	return false, "Autenticación Windows no pudo conectar con el servidor; usando autenticación SQL Server."
}

// SQLConnString es la FUENTE ÚNICA de verdad para la cadena de conexión contra SQL Server.
func SQLConnString(cfg config.Config, pass string) string {
	srv := URLHost(cfg.Server)
	dbName := strings.TrimSpace(cfg.Database)
	if cfg.UseWinAuth {
		return fmt.Sprintf("sqlserver://%s?database=%s&dial+timeout=10&encrypt=disable&trusted+connection=yes", srv, dbName)
	}

	user := strings.TrimSpace(cfg.SQLUser)
	if user == "" {
		user = "sidc"
	}

	resolvedPass := securestore.ResolvePassword(pass, func() string {
		return DSNPassword(cfg.DsnName)
	})
	cleanPass := strings.TrimSpace(resolvedPass)

	// PROHIBIDO el fallback silencioso a Windows Auth
	if cleanPass == "" {
		return fmt.Sprintf("sqlserver://%s@%s?database=%s&dial+timeout=10&encrypt=disable", user, srv, dbName)
	}
	return fmt.Sprintf("sqlserver://%s:%s@%s?database=%s&dial+timeout=10&encrypt=disable", user, urlEscape(cleanPass), srv, dbName)
}

// TestSQLConn prueba la conectividad y autenticación contra la base de datos.
func TestSQLConn(ctx context.Context, cfg config.Config, pass string) error {
	dsn := SQLConnString(cfg, pass)
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return fmt.Errorf("error al abrir conexión: %w", err)
	}
	defer db.Close()

	qctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	var dbName string
	err = db.QueryRowContext(qctx, `SELECT DB_NAME()`).Scan(&dbName)
	if err != nil {
		return SanitizeSQLError(err, cfg)
	}
	return nil
}

// SanitizeSQLError transforma errores técnicos en explicaciones claras y accionables.
func SanitizeSQLError(err error, cfg config.Config) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "18452") || strings.Contains(msg, "untrusted domain"):
		return fmt.Errorf("autenticación rechazada: el equipo no pertenece al dominio del servidor SQL (%s). Debe utilizar autenticación SQL Server (usuario y contraseña)", cfg.Server)
	case strings.Contains(msg, "18456") || strings.Contains(msg, "Login failed"):
		return fmt.Errorf("inicio de sesión fallido para el usuario %q en %s. Verifique la contraseña", cfg.SQLUser, cfg.Server)
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "connection refused") || strings.Contains(msg, "network"):
		return fmt.Errorf("no se pudo alcanzar el servidor %s por TCP. Verifique que el servicio esté activo y el puerto abierto", cfg.Server)
	default:
		return err
	}
}
