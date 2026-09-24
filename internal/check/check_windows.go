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

// testMSDASQL32 ejecuta una prueba de conexión real contra el driver ODBC de 32 bits
// a través de ADODB.Connection en SysWOW64 PowerShell.
func testMSDASQL32(ctx context.Context, connStr string) (bool, string) {
	ps32 := `C:\Windows\SysWOW64\WindowsPowerShell\v1.0\powershell.exe`
	if _, err := exec.LookPath(ps32); err != nil {
		return true, "PowerShell 32-bit no disponible; prueba MSDASQL omitida"
	}

	tCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	psScript := fmt.Sprintf(`
try {
    $conn = New-Object -ComObject ADODB.Connection
    $conn.ConnectionTimeout = 5
    $conn.Open([Environment]::GetEnvironmentVariable('AEGIS_CHECK_CONN'))
    $conn.Close()
    Write-Output "OK"
} catch {
    Write-Output ("FAIL: " + $_.Exception.Message)
}
`)

	cmd := exec.CommandContext(tCtx, ps32, "-NoProfile", "-NonInteractive", "-Command", psScript)
	cmd.Env = checkCommandEnv(map[string]string{"AEGIS_CHECK_CONN": connStr})
	out, err := cmd.CombinedOutput()
	if err != nil {
		if tCtx.Err() == context.DeadlineExceeded {
			return false, "timeout esperando conexión MSDASQL 32-bit"
		}
		return false, fmt.Sprintf("error ejecutando prueba 32-bit: %v", err)
	}

	res := strings.TrimSpace(string(out))
	if strings.HasPrefix(res, "OK") {
		return true, "conexión 32-bit MSDASQL verificada"
	}

	failMsg := strings.TrimPrefix(res, "FAIL: ")
	if strings.Contains(failMsg, "Login failed for user ''") {
		return false, "login falló para usuario '' (el ejecutable no tiene credenciales embebidas)"
	}
	if strings.Contains(failMsg, "Login failed") {
		return false, "login falló (credenciales inválidas o no coinciden con SQL Server)"
	}
	if strings.Contains(failMsg, "untrusted domain") || strings.Contains(failMsg, "18452") {
		return false, "autenticación rechazada: el equipo no pertenece al dominio del servidor SQL"
	}

	return false, failMsg
}
