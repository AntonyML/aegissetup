// © Antony Monge López — Costa Rica — Céd. 604700548
// Package check verifica que App y DB se hablan. Solo lectura.
package check

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aegis-setup/internal/config"
	"aegis-setup/internal/setup"

	_ "github.com/microsoft/go-mssqldb"
)

// Result es una línea del check.
type Result struct {
	Name string
	OK   bool
	Info string
}

// Run ejecuta TCP + SQL + DSN + ficheros. appPass solo para SQL Auth.
func Run(ctx context.Context, cfg config.Config, appPass string) []Result {
	var out []Result

	if strings.Contains(cfg.Server, "\\") && !setup.TienePuerto(cfg.Server) {
		out = append(out, Result{"TCP " + cfg.Server, true, "instancia con nombre (puerto dinámico vía SQL Browser)"})
	} else {
		host := setup.HostPuerto(cfg.Server)
		dialCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		var d net.Dialer
		conn, err := d.DialContext(dialCtx, "tcp", host)
		if err != nil {
			out = append(out, Result{"TCP " + cfg.Server, false, err.Error()})
		} else {
			conn.Close()
			out = append(out, Result{"TCP " + cfg.Server, true, "puerto abierto"})
		}
	}

	dsn := setup.AppDSN(cfg, appPass)
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		out = append(out, Result{"SQL open", false, err.Error()})
	} else {
		qctx, cancel2 := context.WithTimeout(ctx, 15*time.Second)
		defer cancel2()
		var dbname string
		var compat int
		err = db.QueryRowContext(qctx, `SELECT DB_NAME(), compatibility_level FROM sys.databases WHERE name = @p1`, cfg.Database).Scan(&dbname, &compat)
		db.Close()
		if err != nil {
			out = append(out, Result{"SQL " + cfg.Database, false, setup.SanitizeSQLError(err, cfg).Error()})
		} else {
			out = append(out, Result{"SQL " + cfg.Database, true, fmt.Sprintf("compat=%d", compat)})
		}
	}

	if valid, diffs := setup.ValidateDSN(cfg, appPass); !valid {
		out = append(out, Result{"DSN " + cfg.DsnName, false, strings.Join(diffs, "; ")})
	} else if m, err := setup.ReadDSN(cfg.DsnName); err != nil {
		out = append(out, Result{"DSN " + cfg.DsnName, false, err.Error()})
	} else {
		out = append(out, Result{"DSN " + cfg.DsnName, true, fmt.Sprintf("Server=%s Database=%s", m["Server"], m["Database"])})
	}

	if miss := setup.CheckCrystal(); len(miss) > 0 {
		out = append(out, Result{"CRYSTAL runtime", false, "faltan en SysWOW64: " + strings.Join(miss, ", ")})
	} else {
		out = append(out, Result{"CRYSTAL runtime", true, "crpe32 + craxdrt + crviewer presentes"})
	}

	if cfg.AppDir != "" {
		bridgeDll := filepath.Join(cfg.AppDir, "p2sodbc.dll")
		if _, err := os.Stat(bridgeDll); err == nil {
			out = append(out, Result{"CRYSTAL bridge", false, "se encontró p2sodbc.dll local; el runtime de fábrica debe vivir en SysWOW64"})
		} else if !os.IsNotExist(err) {
			out = append(out, Result{"CRYSTAL bridge", false, "no se pudo verificar la ausencia de p2sodbc.dll local"})
		} else {
			out = append(out, Result{"CRYSTAL bridge", true, "sin DLL local: usa p2sodbc.dll del runtime instalado"})
		}
	}

	for _, miss := range setup.CheckAppFiles(cfg.AppDir) {
		out = append(out, Result{"App " + miss, false, "falta"})
	}
	if len(out) == 0 || allOK(out) {
		out = append(out, Result{"App ficheros", true, "exe + ~57 rpt + Principal.jpg"})
	}

	// Verificación de conexión real de la aplicación SIDC (MSDASQL 32-bit + ejecutable)
	appExe := filepath.Join(cfg.AppDir, setup.SIDCExeName)
	var appConnStr string
	if _, err := os.Stat(appExe); err != nil {
		out = append(out, Result{"SIDC app", false, "ejecutable principal no encontrado (" + setup.SIDCExeName + ")"})
	} else if connStr, err := setup.ReadExeConnString(appExe); err != nil {
		out = append(out, Result{"SIDC app", false, "error leyendo cadena del ejecutable: " + err.Error()})
	} else if !cfg.UseWinAuth && (!strings.Contains(connStr, "UID=") || !strings.Contains(connStr, "PWD=")) {
		out = append(out, Result{"SIDC app", false, "ejecutable sin credenciales embebidas (fallaría como usuario '')"})
	} else if ok, info := testMSDASQL32(ctx, connStr); !ok {
		out = append(out, Result{"SIDC app", false, info})
	} else {
		appConnStr = connStr
		out = append(out, Result{"SIDC app", true, "conexión 32-bit MSDASQL verificada"})
	}

	if cfg.AppDir == "" {
		out = append(out, Result{"Reportes muestra", false, "app_dir sin configurar; no se puede abrir un .rpt"})
	} else {
		repDir := filepath.Join(cfg.AppDir, "Reportes")
		files, occurrences, err := setup.CountUnpatchedReports(repDir)
		if err != nil {
			out = append(out, Result{"Reportes conexión", false, "no se pudo leer Reportes: " + err.Error()})
		} else if files > 0 {
			out = append(out, Result{"Reportes conexión", false, fmt.Sprintf("%d plantillas aún fuerzan autenticación integrada (%d ocurrencias)", files, occurrences)})
		} else {
			out = append(out, Result{"Reportes conexión", true, "sin Trusted_Connection=Yes en las plantillas"})
		}
		ok, info := openReportSample(ctx, repDir, cfg, appPass, appConnStr)
		out = append(out, Result{"Reportes Rpt_Caja_Chica", ok, info})
	}

	out = append(out, checkPrinterEnvironment(ctx)...)

	return out
}

func allOK(rs []Result) bool {
	for _, r := range rs {
		if !r.OK {
			return false
		}
	}
	return true
}
