// © Antony Monge López — Costa Rica — Céd. 604700548
// Package check verifica que App y DB se hablan. Solo lectura.
package check

import (
	"context"
	"database/sql"
	"fmt"
	"net"
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
			out = append(out, Result{"SQL " + cfg.Database, false, err.Error()})
		} else {
			out = append(out, Result{"SQL " + cfg.Database, true, fmt.Sprintf("compat=%d", compat)})
		}
	}

	if m, err := setup.ReadDSN(cfg.DsnName); err != nil {
		out = append(out, Result{"DSN " + cfg.DsnName, false, err.Error()})
	} else {
		out = append(out, Result{"DSN " + cfg.DsnName, true, fmt.Sprintf("Server=%s Database=%s", m["Server"], m["Database"])})
	}

	if miss := setup.CheckCrystal(); len(miss) > 0 {
		out = append(out, Result{"CRYSTAL runtime", false, "faltan en SysWOW64: " + strings.Join(miss, ", ")})
	} else {
		out = append(out, Result{"CRYSTAL runtime", true, "crpe32 + craxdrt + crviewer presentes"})
	}

	for _, miss := range setup.CheckAppFiles(cfg.AppDir) {
		out = append(out, Result{"App " + miss, false, "falta"})
	}
	if len(out) == 0 || allOK(out) {
		out = append(out, Result{"App ficheros", true, "exe + ~57 rpt + Principal.jpg"})
	}
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
