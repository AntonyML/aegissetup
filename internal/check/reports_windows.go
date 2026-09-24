//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// openReportSample abre una plantilla con la automatización de Crystal 8. El
// timeout es intencional: Crystal puede quedar esperando Spooler o impresora.
func openReportSample(ctx context.Context, repDir string) (bool, string) {
	path, err := sampleReport(repDir)
	if err != nil {
		return false, "muestra .rpt: " + err.Error()
	}
	ps := filepath.Join(os.Getenv("WINDIR"), "SysWOW64", "WindowsPowerShell", "v1.0", "powershell.exe")
	if _, err := os.Stat(ps); err != nil {
		ps, err = exec.LookPath("powershell.exe")
	}
	if err != nil {
		return false, "muestra .rpt: PowerShell no disponible"
	}
	pathPS := strings.ReplaceAll(path, "'", "''")
	script := fmt.Sprintf(`$ErrorActionPreference='Stop'; $p='%s'; $app=$null; $r=$null; foreach($id in @('CRAXDRT.Application','CrystalRuntime.Application')) { try { $app=New-Object -ComObject $id; break } catch {} }; if($null -eq $app) { throw 'Crystal automation no disponible' }; $r=$app.OpenReport($p); if($null -eq $r) { throw 'OpenReport devolvió vacío' }; $n=$r.Database.Tables.Count; try { $r.Close() } catch {}; Write-Output ('OK:' + $n)`, pathPS)
	tctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(tctx, ps, "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		if tctx.Err() == context.DeadlineExceeded {
			return false, "muestra .rpt: timeout de 15 s (posible espera de Spooler/impresora)"
		}
		return false, "muestra .rpt no abrió dentro del límite"
	}
	res := strings.TrimSpace(string(out))
	if !strings.HasPrefix(res, "OK:") {
		return false, "muestra .rpt no abrió dentro del límite"
	}
	return true, "muestra .rpt abierta por Crystal 8 dentro de 15 s"
}
