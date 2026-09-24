//go:build windows

package precheck

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// printerApta sólo consulta el estado. No inicia servicios ni selecciona colas.
func printerApta() (bool, string) {
	ps, err := exec.LookPath("powershell.exe")
	if err != nil {
		return false, "PowerShell no disponible para consultar Spooler e impresoras"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	script := `$ErrorActionPreference='Stop'; $s=Get-Service -Name Spooler; if($s.Status -ne 'Running'){ Write-Output 'SPOOLER=STOPPED'; exit 2 }; $p=@(Get-CimInstance Win32_Printer | Where-Object { $_.Default -eq $true }); if($p.Count -eq 0){ Write-Output 'PRINTER=NONE'; exit 3 }; Write-Output 'PRINTER=DEFAULT'`
	out, err := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return false, "timeout consultando Print Spooler e impresora predeterminada"
	}
	text := string(out)
	if err != nil || !strings.Contains(text, "PRINTER=DEFAULT") {
		if strings.Contains(text, "SPOOLER=STOPPED") {
			return false, "Print Spooler detenido; Aegis no lo inicia automáticamente"
		}
		return false, "no hay impresora predeterminada; Aegis no cambia la selección"
	}
	return true, "Print Spooler en ejecución y hay una cola predeterminada"
}
