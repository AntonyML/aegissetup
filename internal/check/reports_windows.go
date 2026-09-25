//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aegis-setup/internal/config"
	"aegis-setup/internal/setup"
)

// openReportSample abre exactamente Rpt_Caja_Chica y fuerza la misma ruta de
// login que necesita Crystal al visualizar. El timeout es intencional: Crystal
// puede quedar esperando Spooler o impresora.
func openReportSample(ctx context.Context, repDir string, cfg config.Config, appPass, appConnStr string) (bool, string) {
	path, err := reportPath(repDir)
	if err != nil {
		return false, reportUnderCheck + ": " + err.Error()
	}
	ps := filepath.Join(os.Getenv("WINDIR"), "SysWOW64", "WindowsPowerShell", "v1.0", "powershell.exe")
	if _, err := os.Stat(ps); err != nil {
		ps, err = exec.LookPath("powershell.exe")
	}
	if err != nil {
		return false, reportUnderCheck + ": PowerShell no disponible"
	}
	user := strings.TrimSpace(cfg.SQLUser)
	if user == "" {
		user = "sidc"
	}
	password := strings.TrimSpace(appPass)
	if appConnStr != "" {
		if embeddedUser, embeddedPassword := setup.ExeConnectionCredentials(appConnStr); embeddedUser != "" {
			user = embeddedUser
			password = embeddedPassword
		}
	}
	if !cfg.UseWinAuth && password == "" {
		password = setup.DSNPassword(cfg.DsnName)
	}
	dsn := strings.TrimSpace(cfg.DsnName)
	if dsn == "" {
		dsn = "SIDC_SQL"
	}
	database := strings.TrimSpace(cfg.Database)
	if database == "" {
		database = "SIDC"
	}
	script := `$ErrorActionPreference='Stop'
$app=$null
$r=$null
$exitCode=1
try {
    $app=New-Object -ComObject 'CrystalRuntime.Application'
    $r=$app.OpenReport($env:AEGIS_CHECK_REPORT)
    if($null -eq $r) { throw 'OpenReport devolvió vacío' }
    $uid=$env:AEGIS_CHECK_USER
    $pwd=$env:AEGIS_CHECK_PASSWORD
    if($env:AEGIS_CHECK_WIN_AUTH -eq '1') { $uid=''; $pwd='' }
    $r.Database.LogOnServer('p2sodbc.dll', $env:AEGIS_CHECK_DSN, $env:AEGIS_CHECK_DATABASE, $uid, $pwd)
    $r.ReadRecords()
    $n=$r.Database.Tables.Count
    Write-Output ('OK:' + $n)
    $exitCode=0
} catch {
    $message=$_.Exception.Message
    if($message -match '20599' -or $message -match 'Server has not yet been opened') {
        Write-Output 'FAIL:20599'
    } else {
        Write-Output ('FAIL:' + $message)
    }
} finally {
    if($null -ne $r) { try { $r.Close() } catch {} }
}
exit $exitCode`
	tctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(tctx, ps, "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = checkCommandEnv(map[string]string{
		"AEGIS_CHECK_REPORT":   path,
		"AEGIS_CHECK_DSN":      dsn,
		"AEGIS_CHECK_DATABASE": database,
		"AEGIS_CHECK_USER":     user,
		"AEGIS_CHECK_PASSWORD": password,
		"AEGIS_CHECK_WIN_AUTH": boolEnv(cfg.UseWinAuth),
	})
	out, err := cmd.CombinedOutput()
	if err != nil {
		if tctx.Err() == context.DeadlineExceeded {
			return false, reportUnderCheck + ": timeout de 15 s (posible espera de Spooler/impresora)"
		}
		if strings.Contains(string(out), "FAIL:20599") {
			return false, reportUnderCheck + ": error Crystal 20599 (el servidor del reporte no abrió)"
		}
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			detail = err.Error()
		}
		return false, reportUnderCheck + ": " + detail
	}
	res := strings.TrimSpace(string(out))
	if !strings.HasPrefix(res, "OK:") {
		return false, reportUnderCheck + ": no abrió o no pudo leer registros dentro del límite"
	}
	return true, reportUnderCheck + " abierta por Crystal 8 y registros leídos dentro de 15 s"
}

func boolEnv(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func checkCommandEnv(values map[string]string) []string {
	env := os.Environ()
	for key, value := range values {
		prefix := key + "="
		filtered := make([]string, 0, len(env))
		for _, item := range env {
			if !strings.HasPrefix(item, prefix) {
				filtered = append(filtered, item)
			}
		}
		env = append(filtered, prefix+value)
	}
	return env
}
