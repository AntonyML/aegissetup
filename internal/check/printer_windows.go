//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// checkPrinterEnvironment solo consulta el servicio y las colas. Nunca cambia
// StartType, inicia Spooler ni selecciona una impresora predeterminada.
func checkPrinterEnvironment(ctx context.Context) []Result {
	ps, err := exec.LookPath("powershell.exe")
	if err != nil {
		return []Result{{"Spooler", false, "PowerShell no disponible para consultar Print Spooler"}, {"Impresora predeterminada", false, "PowerShell no disponible para consultar impresoras"}}
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	script := `$ErrorActionPreference='Stop'; $s=Get-Service -Name Spooler; if($s.Status -ne 'Running'){ Write-Output 'SPOOLER=STOPPED'; exit 2 }; $p=@(Get-CimInstance Win32_Printer | Where-Object { $_.Default -eq $true }); if($p.Count -eq 0){ Write-Output 'SPOOLER=RUNNING'; Write-Output 'PRINTER=NONE'; exit 3 }; Write-Output 'SPOOLER=RUNNING'; Write-Output 'PRINTER=DEFAULT'`
	out, err := exec.CommandContext(tctx, ps, "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if tctx.Err() == context.DeadlineExceeded {
		return []Result{{"Spooler", false, "timeout consultando Print Spooler"}, {"Impresora predeterminada", false, "timeout consultando impresoras"}}
	}
	text := string(out)
	running := strings.Contains(text, "SPOOLER=RUNNING")
	printer := strings.Contains(text, "PRINTER=DEFAULT")
	if err != nil && !running {
		return []Result{{"Spooler", false, "Print Spooler detenido o no se pudo consultar"}, {"Impresora predeterminada", false, "no se puede evaluar mientras Spooler está detenido"}}
	}
	spooler := Result{"Spooler", running, "Print Spooler en ejecución"}
	if !running {
		spooler.Info = "Print Spooler detenido; Aegis no lo inicia automáticamente"
	}
	printerResult := Result{"Impresora predeterminada", printer, "hay una cola predeterminada disponible"}
	if !printer {
		printerResult.Info = "no hay impresora predeterminada; Aegis no cambia la selección"
	}
	if err != nil && !printer {
		printerResult.Info = fmt.Sprintf("no se pudo consultar una impresora predeterminada: %s", strings.TrimSpace(text))
	}
	return []Result{spooler, printerResult}
}
