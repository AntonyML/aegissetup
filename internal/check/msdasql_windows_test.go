//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"aegis-setup/internal/setup"
)

func TestMSDASQL32ScriptSePuedeAnalizarSinConectar(t *testing.T) {
	ps32 := `C:\Windows\SysWOW64\WindowsPowerShell\v1.0\powershell.exe`
	if _, err := exec.LookPath(ps32); err != nil {
		t.Skip("PowerShell 32-bit no disponible")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Crear el ScriptBlock analiza la sintaxis; no invoca ADODB ni el servidor.
	cmd := exec.CommandContext(ctx, ps32, "-NoProfile", "-NonInteractive", "-Command",
		`$ErrorActionPreference='Stop'; $null=[scriptblock]::Create([Environment]::GetEnvironmentVariable('AEGIS_TEST_SCRIPT'))`)
	cmd.Env = checkCommandEnv(map[string]string{"AEGIS_TEST_SCRIPT": msdasql32Script})
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("PowerShell no pudo analizar la sonda: %v: %s", err, output)
	}
}

// TestMSDASQL32FechaDMYIntegracionEXE solo se habilita con una ruta explícita.
// Compara la cadena del EXE con un candidato en memoria; no escribe DSN ni EXE.
func TestMSDASQL32FechaDMYIntegracionEXE(t *testing.T) {
	exePath := os.Getenv("AEGIS_VERIFY_EXE")
	if exePath == "" {
		t.Skip("integración opt-in: configurá AEGIS_VERIFY_EXE con el EXE de la copia")
	}
	connStr, err := setup.ReadExeConnString(exePath)
	if err != nil {
		t.Fatal("no se pudo extraer la cadena del EXE para la integración")
	}
	if strings.Contains(strings.ToLower(connStr), "language=") {
		t.Fatal("la comparación requiere un EXE sin Language explícito")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	baseline := testMSDASQL32(ctx, connStr)
	t.Logf("baseline: conexión=%t; fechas=%t; %s", baseline.ConnectionOK, baseline.DatesOK, baseline.DatesInfo)
	if !baseline.ConnectionOK {
		t.Fatalf("el baseline no abrió: %s", baseline.ConnectionInfo)
	}
	if baseline.DatesOK {
		t.Log("la instalación actual ya interpreta DMY; se verifica también el candidato")
	}
	// MSDASQL pasa los parámetros no definidos por ADO al driver ODBC. Se cambia
	// únicamente el idioma de esta nueva conexión, sin SET ni cambios persistentes.
	candidate := testMSDASQL32(ctx, strings.TrimRight(connStr, ";")+";Language=Spanish;")
	t.Logf("candidato en memoria: conexión=%t; fechas=%t; %s", candidate.ConnectionOK, candidate.DatesOK, candidate.DatesInfo)
	if !candidate.ConnectionOK {
		t.Fatalf("el candidato no abrió: %s", candidate.ConnectionInfo)
	}
	if !candidate.DatesOK {
		t.Fatal("el candidato Language=Spanish no corrigió las conversiones de fecha")
	}
}
