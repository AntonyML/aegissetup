//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// La sonda no fija LANGUAGE ni DATEFORMAT: observa la configuración que recibió
// la conexión real del EXE. ISDATE funciona con compatibilidad 100 (SQL 2014).
const msdasql32Script = `$ErrorActionPreference='Stop'
[Console]::OutputEncoding=[System.Text.Encoding]::UTF8
$conn=$null
$rs=$null
$result=@{connected=$false; language=''; dateFormat=''; planDate=''; ambiguousDate=''; reverseDate=''; compactDate=''; isoDate=''; errorCode=0}
try {
    $conn=New-Object -ComObject ADODB.Connection
    $conn.ConnectionTimeout=5
    $conn.CommandTimeout=5
    $conn.Open([Environment]::GetEnvironmentVariable('AEGIS_CHECK_CONN'))
    $result.connected=$true
    $query=@'
SELECT @@LANGUAGE AS language, date_format AS dateFormat,
  CASE WHEN ISDATE('25/9/2026')=1 THEN CONVERT(char(8),CONVERT(datetime,'25/9/2026'),112) ELSE '' END AS planDate,
  CASE WHEN ISDATE('5/9/2026')=1 THEN CONVERT(char(8),CONVERT(datetime,'5/9/2026'),112) ELSE '' END AS ambiguousDate,
  CASE WHEN ISDATE('09/05/2026')=1 THEN CONVERT(char(8),CONVERT(datetime,'09/05/2026'),112) ELSE '' END AS reverseDate,
  CASE WHEN ISDATE('20260925')=1 THEN CONVERT(char(8),CONVERT(datetime,'20260925'),112) ELSE '' END AS compactDate,
  CASE WHEN ISDATE('2026-09-25T00:00:00')=1 THEN CONVERT(char(8),CONVERT(datetime,'2026-09-25T00:00:00'),112) ELSE '' END AS isoDate
FROM sys.dm_exec_sessions WHERE session_id=@@SPID
'@
    $rs=$conn.Execute($query)
    if($rs.EOF) { throw 'No se recibió la configuración de la sesión' }
    foreach($name in @('language','dateFormat','planDate','ambiguousDate','reverseDate','compactDate','isoDate')) {
        $result[$name]=[string]$rs.Fields.Item($name).Value
    }
} catch {
    # Nunca devolvemos Exception.Message: podría contener la cadena o la clave.
    $result.errorCode=[int]$_.Exception.HResult
    if($null -ne $conn) {
        for($i=0; $i -lt $conn.Errors.Count; $i++) {
            $native=[int]$conn.Errors.Item($i).NativeError
            if($native -ne 0) { $result.errorCode=$native; break }
        }
    }
} finally {
    if($null -ne $rs) { try { $rs.Close() } catch {} }
    if($null -ne $conn) { try { $conn.Close() } catch {} }
}
$result | ConvertTo-Json -Compress`

// testMSDASQL32 abre ADODB y valida fechas sobre esa misma sesión de 32 bits.
func testMSDASQL32(ctx context.Context, connStr string) msdasqlProbe {
	ps32 := `C:\Windows\SysWOW64\WindowsPowerShell\v1.0\powershell.exe`
	if _, err := exec.LookPath(ps32); err != nil {
		return msdasqlProbe{
			ConnectionInfo: "PowerShell 32-bit no disponible; conexión no verificada",
			DatesInfo:      "fechas no verificadas: PowerShell 32-bit no disponible",
		}
	}

	tCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(tCtx, ps32, "-NoProfile", "-NonInteractive", "-Command", msdasql32Script)
	cmd.Env = checkCommandEnv(map[string]string{"AEGIS_CHECK_CONN": connStr})
	out, err := cmd.CombinedOutput()
	if err != nil {
		if tCtx.Err() == context.DeadlineExceeded {
			return msdasqlProbe{
				ConnectionInfo: "timeout esperando prueba MSDASQL 32-bit",
				DatesInfo:      "fechas no verificadas: la prueba superó el límite de 10 s",
			}
		}
		return msdasqlProbe{
			ConnectionInfo: "no se pudo ejecutar la prueba MSDASQL 32-bit",
			DatesInfo:      "fechas no verificadas: no se pudo ejecutar la prueba",
		}
	}
	res := strings.TrimPrefix(strings.TrimSpace(string(out)), "\ufeff")
	return parseMSDASQL32Output([]byte(res))
}
