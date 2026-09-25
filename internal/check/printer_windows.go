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
		return []Result{
			{"Spooler", false, "PowerShell no disponible para consultar Print Spooler"},
			{"Impresora predeterminada", false, "PowerShell no disponible para consultar impresoras"},
			{"Microsoft Print to PDF", false, "PowerShell no disponible para consultar impresoras"},
		}
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	script := `$ErrorActionPreference='Stop'; $s=Get-Service -Name Spooler; if($s.Status -ne 'Running'){ Write-Output 'SPOOLER=STOPPED'; exit 2 }; $printers=@(Get-CimInstance Win32_Printer); if(@($printers | Where-Object { $_.Default -eq $true }).Count -eq 0){ Write-Output 'PRINTER=NONE' } else { Write-Output 'PRINTER=DEFAULT' }; if(@($printers | Where-Object { $_.Name -eq 'Microsoft Print to PDF' }).Count -eq 0){ Write-Output 'PDF=NONE' } else { Write-Output 'PDF=INSTALLED' }; Write-Output 'SPOOLER=RUNNING'`
	out, err := exec.CommandContext(tctx, ps, "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if tctx.Err() == context.DeadlineExceeded {
		return []Result{
			{"Spooler", false, "timeout consultando Print Spooler"},
			{"Impresora predeterminada", false, "timeout consultando impresoras"},
			{"Microsoft Print to PDF", false, "timeout consultando cola PDF"},
		}
	}
	text := string(out)
	running := strings.Contains(text, "SPOOLER=RUNNING")
	printer := strings.Contains(text, "PRINTER=DEFAULT")
	pdf := strings.Contains(text, "PDF=INSTALLED")
	if err != nil && !running {
		return []Result{
			{"Spooler", false, "Print Spooler detenido o no se pudo consultar"},
			{"Impresora predeterminada", false, "no se puede evaluar mientras Spooler está detenido"},
			{"Microsoft Print to PDF", false, "no se puede evaluar mientras Spooler está detenido"},
		}
	}
	spooler := Result{"Spooler", running, "Print Spooler en ejecución"}
	if !running {
		spooler.Info = "Print Spooler detenido; Aegis no lo inicia automáticamente"
	}
	printerResult := Result{"Impresora predeterminada", printer, "hay una cola predeterminada disponible"}
	if !printer {
		printerResult.Info = "no hay impresora predeterminada; Crystal Reports puede colgarse"
	}
	if err != nil && !printer {
		printerResult.Info = fmt.Sprintf("no se pudo consultar una impresora predeterminada: %s", strings.TrimSpace(text))
	}
	pdfResult := Result{"Microsoft Print to PDF", pdf, "cola de impresión virtual PDF disponible"}
	if !pdf {
		pdfResult.Info = "cola no encontrada; Aegis la puede configurar automáticamente"
	}
	return []Result{spooler, printerResult, pdfResult}
}
