-- © Antony Monge López — Costa Rica — Céd. 604700548
-- 01-sidc.sql : se ejecuta UNA vez ya con el contenedor arriba.
-- Crea base SIDC vacia (si no vas a restaurar .bak) + login de app para dev.
-- Prod: Server=CONTABILIDAD Database=SIDC Trusted_Connection=Yes Language=Espanol
-- Dev Docker (Linux, sin AD): mismo DSN pero con SQL Auth. Por eso creamos sidc_dev.

IF DB_ID('SIDC') IS NULL
BEGIN
    CREATE DATABASE SIDC;
END
GO

-- Compatibilidad SQL 2014 (120) para imitar CONTABILIDAD (2014 Express 12.0.2000.8).
ALTER DATABASE SIDC SET COMPATIBILITY_LEVEL = 120;
GO

-- Login de app solo dev
IF NOT EXISTS (SELECT * FROM sys.server_principals WHERE name = 'sidc_dev')
BEGIN
    CREATE LOGIN sidc_dev WITH PASSWORD = 'Sidc*2026*App', DEFAULT_DATABASE = SIDC;
END
GO

USE SIDC;
GO
IF NOT EXISTS (SELECT * FROM sys.database_principals WHERE name = 'sidc_dev')
BEGIN
    CREATE USER sidc_dev FOR LOGIN sidc_dev;
    ALTER ROLE db_owner ADD MEMBER sidc_dev;
END
GO

-- Verifica: debe decir SIDC
SELECT DB_NAME() AS DbActual, @@SERVERNAME AS ServerReal, SERVERPROPERTY('ProductVersion') AS Version;
GO
