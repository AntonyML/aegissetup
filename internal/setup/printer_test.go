// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestEnsurePrintToPDF(t *testing.T) {
	if runtime.GOOS != "windows" {
		var msgs []string
		err := EnsurePrintToPDF(func(s string) { msgs = append(msgs, s) })
		if err != nil {
			t.Fatalf("EnsurePrintToPDF en no-windows no debe fallar: %v", err)
		}
		if len(msgs) == 0 || !strings.Contains(msgs[0], "no aplica") {
			t.Errorf("se esperaba mensaje de 'no aplica fuera de Windows', obtuvo: %v", msgs)
		}
		return
	}

	// Caso 1: La cola ya existe
	prevRun := runCmdContext
	defer func() { runCmdContext = prevRun }()

	runCmdContext = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		// Simula que la consulta a Get-CimInstance Win32_Printer devuelve EXISTS
		cmdLine := strings.Join(args, " ")
		if strings.Contains(cmdLine, "Get-CimInstance Win32_Printer") && strings.Contains(cmdLine, PrintToPDFPrinterName) {
			return []byte("EXISTS\r\n"), nil
		}
		return []byte(""), nil
	}

	var msgs []string
	err := EnsurePrintToPDF(func(s string) { msgs = append(msgs, s) })
	if err != nil {
		t.Fatalf("EnsurePrintToPDF falló cuando la cola ya existe: %v", err)
	}

	found := false
	for _, m := range msgs {
		if strings.Contains(m, "ya se encuentra disponible") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("se esperaba mensaje 'ya se encuentra disponible', obtuvo: %v", msgs)
	}
}

func TestEnsurePrintToPDFEnablesFeatureAndAddsQueue(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Prueba específica de simulación Windows")
	}

	prevRun := runCmdContext
	defer func() { runCmdContext = prevRun }()

	queueExists := false
	featureEnabled := false
	dismCalled := false
	addPrinterCalled := false

	runCmdContext = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmdLine := strings.Join(args, " ")

		// Consulta si la cola existe
		if strings.Contains(cmdLine, "Get-CimInstance Win32_Printer") && strings.Contains(cmdLine, PrintToPDFPrinterName) && !strings.Contains(cmdLine, "Invoke-CimMethod") {
			if queueExists {
				return []byte("EXISTS\r\n"), nil
			}
			return []byte(""), nil
		}

		// Consulta estado en DISM
		if strings.Contains(cmdLine, "/Get-FeatureInfo") {
			if featureEnabled {
				return []byte("Feature Name : Printing-PrintToPDFServices-Features\r\nState : Enabled\r\n"), nil
			}
			return []byte("Feature Name : Printing-PrintToPDFServices-Features\r\nState : Disabled\r\n"), nil
		}

		// Habilita característica en DISM
		if strings.Contains(cmdLine, "/Enable-Feature") {
			dismCalled = true
			featureEnabled = true
			return []byte("The operation completed successfully.\r\n"), nil
		}

		// Agrega la impresora vía Add-Printer
		if strings.Contains(cmdLine, "Add-Printer") {
			addPrinterCalled = true
			queueExists = true
			return []byte(""), nil
		}

		// Default printer
		if strings.Contains(cmdLine, "SetDefaultPrinter") {
			return []byte("SET_DEFAULT\r\n"), nil
		}

		return []byte(""), nil
	}

	var msgs []string
	err := EnsurePrintToPDF(func(s string) { msgs = append(msgs, s) })
	if err != nil {
		t.Fatalf("EnsurePrintToPDF falló durante la habilitación simulada: %v", err)
	}

	if !dismCalled {
		t.Error("se esperaba llamada a DISM para habilitar la característica")
	}
	if !addPrinterCalled {
		t.Error("se esperaba llamada a Add-Printer para registrar la cola")
	}
}

func TestEnsurePrintToPDFDismFails(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Prueba específica de simulación Windows")
	}

	prevRun := runCmdContext
	defer func() { runCmdContext = prevRun }()

	runCmdContext = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmdLine := strings.Join(args, " ")
		if strings.Contains(cmdLine, "Get-CimInstance Win32_Printer") {
			return []byte(""), nil
		}
		if strings.Contains(cmdLine, "/Get-FeatureInfo") {
			return []byte("State : Disabled\r\n"), nil
		}
		if strings.Contains(cmdLine, "/Enable-Feature") {
			return []byte("Error: 740 - The requested operation requires elevation.\r\n"), errors.New("exit status 740")
		}
		return []byte(""), nil
	}

	err := EnsurePrintToPDF(func(string) {})
	if err == nil {
		t.Fatal("se esperaba error cuando DISM falla por permisos")
	}
	if !strings.Contains(err.Error(), "Administrador") {
		t.Errorf("el mensaje de error debe orientar a correr como Administrador: %v", err)
	}
}
