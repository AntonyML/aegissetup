# HISTÓRICO — no seguir. Reemplazado por docker\GUIA-DOCKER.txt y `aegis setup-db`.
# Se conserva como referencia. OJO: menciona SQL 2012 y compat 110, pero prod es
# SQL 2014 Express y el nivel correcto es compat 120 (ver LEEME.txt de esta carpeta).
# SIDC - SQL dev en Docker
# Replica prod: DSN SIDC_SQL / Database SIDC / Language Espanol
# Prod real: Server=CONTABILIDAD Database=SIDC Trusted_Connection=Yes Driver=SQL Server
# OJO: no existe imagen Docker oficial de SQL 2012 (solo Windows). Se usa 2019
# con COMPATIBILITY_LEVEL 110 para imitar 2012. Validado para .bak 2012 -> 2019.

# 1) Requisitos: Docker Desktop + plugin compose. En esta PC aun no esta instalado.

# 2) Configura:
copy .env.example .env
# edita SA_PASSWORD / SQL_APP_PASSWORD si quieres

# 3) Levanta:
docker compose up -d
docker ps
docker logs sidc_sql2019 --tail 50
# espera "SQL Server is now ready for client connections"

# 4a) Opcion A - base vacia (solo estructura para probar conexion):
docker exec -i sidc_sql2019 /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "Sidc*2026*Dev" -C -i /var/opt/mssql/init/01-sidc.sql

# 4b) Opcion B - restaurar .bak real 2012 (recomendado, automatico):
#  - Copia SIDC_prod.bak a ..\..\assets\backups\sqlserver2014\
#  - Corre: .\migrate.ps1
#  El script detecta el .bak mas nuevo, hace FILELISTONLY solo,
#  RESTORE ... WITH MOVE ... REPLACE como SIDC, CHECKDB,
#  COMPATIBILITY_LEVEL 110 (modo 2012) y crea sidc_dev.
#  Manual equivalente (si migrate.ps1 falla):
#  docker exec -it sidc_sql2019 /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "Sidc*2026*Dev" -C -Q "RESTORE FILELISTONLY FROM DISK='/var/opt/mssql/backup/SIDC_prod.bak'"

# 5) Verifica collation (debe coincidir con prod o dara errores en JOINs con tempdb):
# En prod: SELECT DATABASEPROPERTYEX('SIDC','Collation');
# Si prod no es SQL_Latin1_General_CP1_CI_AS, cambia SQL_COLLATION en .env ANTES del primer up,
# o recrea volumen: docker compose down -v

# 6) DSN 32-bit en Windows apuntando a Docker (VB6 solo ve 32-bit):
# C:\Windows\SysWOW64\odbcad32.exe > System DSN > Add
#   Driver: ODBC Driver 17 for SQL Server (mejor que 18 contra 2019 sin Encrypt)
#   Name: SIDC_SQL
#   Server: localhost,14333   (coma, no dos puntos)
#   Auth: SQL Server authentication -> Login: sidc_dev / Sidc*2026*App
#   Default database: SIDC
#   Language: Espanol
#   Test -> SUCCESS
# PowerShell equivalente:
# Add-OdbcDsn -Name "SIDC_SQL" -DriverName "ODBC Driver 17 for SQL Server" -DsnType System -Platform "32-bit" -SetPropertyValue @("Server=localhost,14333","Database=SIDC","Language=Espanol")

# 7) Truco para mantener LITERALMENTE Server=CONTABILIDAD:
# Agrega a C:\Windows\System32\drivers\etc\hosts:
#   127.0.0.1  CONTABILIDAD
# Y en el DSN usa Server: CONTABILIDAD,14333
# Asi el .exe y los .rpt creen que hablan con prod.

# 8) Access: copia DB_SISTEMA.mdb (pwd fdrfrd) a C:\DEV\SIDC\DB_SISTEMA.mdb

# 9) Baja / limpia:
# docker compose down      (mantiene datos)
# docker compose down -v   (borra todo, util si cambias collation)
