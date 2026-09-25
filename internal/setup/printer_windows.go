//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	// PrintToPDFFeatureName es el identificador de la característica opcional en Windows.
	PrintToPDFFeatureName = "Printing-PrintToPDFServices-Features"
	// PrintToPDFPrinterName es el nombre de la cola de impresión virtual.
	PrintToPDFPrinterName = "Microsoft Print to PDF"
	// PrintToPDFDriverName es el controlador registrado por Windows.
	PrintToPDFDriverName = "Microsoft Print to PDF"
	// PrintToPDFPortName es el puerto estándar que despliega el diálogo Guardar como PDF.
	PrintToPDFPortName = "PORTPROMPT:"
)

var (
	runCmdContext = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		return cmd.CombinedOutput()
	}
)

// IsPrintToPDFInstalled verifica si la cola "Microsoft Print to PDF" ya existe.
func IsPrintToPDFInstalled() bool {
	ps, err := exec.LookPath("powershell.exe")
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	script := fmt.Sprintf(`$p = @(Get-CimInstance Win32_Printer | Where-Object { $_.Name -eq '%s' }); if ($p.Count -gt 0) { Write-Output 'EXISTS' }`, PrintToPDFPrinterName)
	out, err := runCmdContext(ctx, ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "EXISTS")
}

// isPrintToPDFFeatureEnabled consulta a DISM si la característica opcional está habilitada.
func isPrintToPDFFeatureEnabled() (bool, error) {
	dism, err := exec.LookPath("dism.exe")
	if err != nil {
		return false, fmt.Errorf("dism.exe no encontrado en el sistema: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := runCmdContext(ctx, dism, "/Online", "/Get-FeatureInfo", "/FeatureName:"+PrintToPDFFeatureName)
	if err != nil {
		return false, fmt.Errorf("error consultando estado de característica con DISM: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	text := string(out)
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "state :") {
			return strings.Contains(strings.ToLower(trimmed), "enabled"), nil
		}
	}
	return false, nil
}

// enablePrintToPDFFeature activa la característica en Windows con DISM.
func enablePrintToPDFFeature(out func(string)) error {
	dism, err := exec.LookPath("dism.exe")
	if err != nil {
		return fmt.Errorf("dism.exe no encontrado en el sistema: %w", err)
	}
	if out != nil {
		out("Habilitando característica de Windows: " + PrintToPDFFeatureName + "...")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := runCmdContext(ctx, dism, "/Online", "/Enable-Feature", "/FeatureName:"+PrintToPDFFeatureName, "/NoRestart")
	if err != nil {
		// DISM puede retornar 3010 (reinicio requerido), que se considera éxito de instalación.
		exitErr, ok := err.(*exec.ExitError)
		if ok && exitErr.ExitCode() == 3010 {
			if out != nil {
				out("Característica " + PrintToPDFFeatureName + " habilitada (reinicio sugerido por Windows).")
			}
			return nil
		}
		return fmt.Errorf("no se pudo habilitar %s con DISM: %w (%s). Asegurate de correr como Administrador", PrintToPDFFeatureName, err, strings.TrimSpace(string(res)))
	}
	if out != nil {
		out("Característica " + PrintToPDFFeatureName + " habilitada con éxito.")
	}
	return nil
}

// addPrintToPDFQueue agrega la cola de impresora si el controlador ya está instalado pero falta la cola.
func addPrintToPDFQueue(out func(string)) error {
	ps, err := exec.LookPath("powershell.exe")
	if err != nil {
		return fmt.Errorf("powershell.exe no encontrado: %w", err)
	}
	if out != nil {
		out("Creando cola de impresión '" + PrintToPDFPrinterName + "'...")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	script := fmt.Sprintf(`$ErrorActionPreference='Stop'; Add-Printer -Name '%s' -DriverName '%s' -PortName '%s'`,
		PrintToPDFPrinterName, PrintToPDFDriverName, PrintToPDFPortName)
	res, err := runCmdContext(ctx, ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	if err != nil {
		return fmt.Errorf("no se pudo registrar la cola de impresión: %w (%s)", err, strings.TrimSpace(string(res)))
	}
	if out != nil {
		out("Cola de impresión '" + PrintToPDFPrinterName + "' creada con éxito.")
	}
	return nil
}

// ensureDefaultPrinter verifica si existe una impresora predeterminada.
// Si no hay ninguna en el sistema, asigna Microsoft Print to PDF como predeterminada
// para evitar que Crystal Reports 8 se cuelgue al abrir reportes.
func ensureDefaultPrinter(out func(string)) {
	ps, err := exec.LookPath("powershell.exe")
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	script := fmt.Sprintf(`
		$p = @(Get-CimInstance Win32_Printer | Where-Object { $_.Default -eq $true });
		if ($p.Count -eq 0) {
			$target = @(Get-CimInstance Win32_Printer | Where-Object { $_.Name -eq '%s' });
			if ($target.Count -gt 0) {
				$target[0] | Invoke-CimMethod -MethodName SetDefaultPrinter | Out-Null;
				Write-Output 'SET_DEFAULT';
			}
		}
	`, PrintToPDFPrinterName)

	res, err := runCmdContext(ctx, ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	if err == nil && strings.Contains(string(res), "SET_DEFAULT") {
		if out != nil {
			out("No había ninguna impresora predeterminada en el sistema; se asignó '" + PrintToPDFPrinterName + "' como predeterminada.")
		}
	}
}

// EnsurePrintToPDF se asegura de que la característica opcional esté activa, la cola exista
// y que el sistema cuente con una impresora predeterminada funcional.
func EnsurePrintToPDF(out func(string)) error {
	if out == nil {
		out = func(string) {}
	}

	// 1. Si la cola ya existe, solo comprobamos si hace falta como predeterminada
	if IsPrintToPDFInstalled() {
		out("Impresora '" + PrintToPDFPrinterName + "' ya se encuentra disponible.")
		ensureDefaultPrinter(out)
		return nil
	}

	// 2. Verificar y habilitar característica opcional si está deshabilitada
	enabled, err := isPrintToPDFFeatureEnabled()
	if err != nil {
		out("AVISO DISM: " + err.Error())
	}
	if !enabled {
		if err := enablePrintToPDFFeature(out); err != nil {
			return err
		}
	}

	// 3. Si tras habilitar la característica aún no está la cola, la agregamos explícitamente
	if !IsPrintToPDFInstalled() {
		if err := addPrintToPDFQueue(out); err != nil {
			return err
		}
	}

	// 4. Asegurar impresora predeterminada para evitar cuelgues en Crystal Reports
	ensureDefaultPrinter(out)
	out("Microsoft Print to PDF configurada y lista para su uso.")
	return nil
}
