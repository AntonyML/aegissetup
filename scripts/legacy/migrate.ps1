# © Antony Monge López — Costa Rica — Céd. 604700548
# LEGACY: reemplazado por `aegis setup-db`. Se conserva como referencia.
# migrate.ps1 - Restaura el .bak 2014 mas nuevo a Docker 2019
# Uso: cd C:\DEV\SIDC\docker-dev ; .\migrate.ps1 [-BakName "SIDC_2026.bak"]
param([string]$BakName = "")

$ErrorActionPreference = "Stop"
. ./LoadEnv.ps1 2>$null
if (-not $env:SA_PASSWORD) { $env:SA_PASSWORD = "Sidc*2026*Dev" }
if (-not $env:SQL_APP_PASSWORD) { $env:SQL_APP_PASSWORD = "Sidc*2026*App" }
$SA = $env:SA_PASSWORD
$APP_PWD = $env:SQL_APP_PASSWORD
$C = "sidc_sql2019"
$SQLCMD = "/opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P `"$SA`" -C"

Write-Host "== 1/5 Buscando .bak en ..\..\assets\backups\sqlserver2014\ =="
$baks = Get-ChildItem -LiteralPath "..\..\assets\backups\sqlserver2014" -Filter "*.bak" -File | Sort-Object LastWriteTime -Descending
if (-not $baks) { throw "No hay .bak en ..\..\assets\backups\sqlserver2014\. Copia el de prod ahi." }
$bak = if ($BakName) { $baks | Where-Object { $_.Name -eq $BakName } | Select-Object -First 1 } else { $baks[0] }
if (-not $bak) { throw "No se encontro $BakName. Hay: $($baks.Name -join ', ')" }
Write-Host "BAK: $($bak.Name) $([math]::Round($bak.Length/1MB,1)) MB"
$bakLinux = "/var/opt/mssql/backup/$($bak.Name)"

Write-Host "== 2/5 Esperando SQL listo =="
for ($i=0; $i -lt 30; $i++) {
  $r = docker exec $C $SQLCMD -Q "SELECT 1" 2>&1
  if ($LASTEXITCODE -eq 0) { break }
  Start-Sleep 2
}

Write-Host "== 3/5 Leyendo nombres logicos del .bak =="
$files = docker exec $C /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA" -C -W -s "," -Q "SET NOCOUNT ON; RESTORE FILELISTONLY FROM DISK='$bakLinux'" | Select-String ","
# FILELISTONLY devuelve: LogicalName, PhysicalName, Type (D=datos, L=log), ...
$dataRow = $files | Select-String ",D," | Select-Object -First 1
$logRow  = $files | Select-String ",L," | Select-Object -First 1
if (-not $dataRow -or -not $logRow) { throw "No pude leer FILELISTONLY. Salida: $($files -join ';')" }
$dataLogic = ($dataRow -split ",")[0].Trim()
$logLogic  = ($logRow -split ",")[0].Trim()
Write-Host "DATA=$dataLogic LOG=$logLogic"

Write-Host "== 4/5 Restaurando como SIDC (REPLACE, sin tocar tu .bak) =="
$restore = "RESTORE DATABASE SIDC FROM DISK='$bakLinux' WITH REPLACE, MOVE '$dataLogic' TO '/var/opt/mssql/data/SIDC.mdf', MOVE '$logLogic' TO '/var/opt/mssql/data/SIDC_log.ldf';"
docker exec $C /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA" -C -Q $restore
if ($LASTEXITCODE -ne 0) { throw "Fallo RESTORE. Revisa MOVE con FILELISTONLY." }

Write-Host "== 5/5 Compat 2014 + login dev + check =="
$fix = @"
ALTER DATABASE SIDC SET COMPATIBILITY_LEVEL = 120;
ALTER DATABASE SIDC SET SINGLE_USER WITH ROLLBACK IMMEDIATE;
DBCC CHECKDB('SIDC') WITH NO_INFOMSGS;
ALTER DATABASE SIDC SET MULTI_USER;
IF NOT EXISTS (SELECT * FROM sys.server_principals WHERE name='sidc_dev') CREATE LOGIN sidc_dev WITH PASSWORD='$APP_PWD', DEFAULT_DATABASE=SIDC;
USE SIDC; IF NOT EXISTS (SELECT * FROM sys.database_principals WHERE name='sidc_dev') BEGIN CREATE USER sidc_dev FOR LOGIN sidc_dev; ALTER ROLE db_owner ADD MEMBER sidc_dev; END
SELECT name, compatibility_level, collation_name FROM sys.databases WHERE name='SIDC';
"@
docker exec $C /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA" -C -Q $fix

Write-Host ""
Write-Host "OK. Prueba DSN: Server=localhost,14333 Database=SIDC UID=sidc_dev"
Write-Host "Si quieres literal Server=CONTABILIDAD agrega 127.0.0.1 CONTABILIDAD al hosts y usa CONTABILIDAD,14333"
