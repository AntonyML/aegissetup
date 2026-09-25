; Script de Inno Setup para Aegis Setup (SIDC)
; © Antony Monge López — Costa Rica — Céd. 604700548

#define MyAppName "Aegis Setup"
#ifndef MyAppVersion
  #define MyAppVersion "1.0.0"
#endif
#define MyAppPublisher "Antony Monge López"
#define MyAppURL "https://github.com/AntonyML/aegissetup"
#define MyAppExeName "aegis.exe"
#define SidcExeSuffix "_AegisSetup.exe"
#define SidcDockerExeTemplate "_Docker_AegisSetup_v<version>.exe"

[Setup]
AppId={{5E9A8C12-8921-4BA2-9D3E-A0B82F6D173C}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\AegisSetup
DefaultGroupName={#MyAppName}
AllowNoIcons=yes
LicenseFile=LICENSE
OutputDir=dist
OutputBaseFilename=AegisSetup-Setup-v{#MyAppVersion}
SetupIconFile=assets\public\logo.ico
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayIcon={app}\logo.ico

[Languages]
Name: "spanish"; MessagesFile: "compiler:Languages\Spanish.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
; Binario principal de Aegis Setup
Source: "bin\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
; Host x86 para Crystal Reports COM/ActiveX; Aegis.exe permanece x64
Source: "bin\Aegis.ReportHost.exe"; DestDir: "{app}"; Flags: ignoreversion
; Icono oficial de la aplicación
Source: "assets\public\logo.ico"; DestDir: "{app}"; Flags: ignoreversion
; Documentación y licencia
Source: "README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
; Accesos directos para iniciar el asistente interactivo (TUI) con su icono oficial
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Parameters: "menu"; IconFilename: "{app}\logo.ico"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Parameters: "menu"; IconFilename: "{app}\logo.ico"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Parameters: "menu"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent
