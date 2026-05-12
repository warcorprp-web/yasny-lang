# Скрипт для создания Windows инсталлятора с помощью Inno Setup

[Setup]
AppName=Простой
AppVersion=0.40
AppPublisher=Prostoy Lang Team
AppPublisherURL=https://github.com/user/prostoy-lang
DefaultDirName={autopf}\Prostoy
DefaultGroupName=Простой
OutputDir=installers
OutputBaseFilename=prostoy-setup-0.40-windows
Compression=lzma2
SolidCompression=yes
ArchitecturesInstallIn64BitMode=x64
WizardStyle=modern
SetupIconFile=assets\icon.ico
UninstallDisplayIcon={app}\prostoy.exe

[Languages]
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"

[Tasks]
Name: "desktopicon"; Description: "Создать ярлык на рабочем столе"; GroupDescription: "Дополнительно:"
Name: "addtopath"; Description: "Добавить в PATH (рекомендуется)"; GroupDescription: "Дополнительно:"

[Files]
Source: "dist\prostoy-windows-amd64.exe"; DestDir: "{app}"; DestName: "prostoy.exe"; Flags: ignoreversion
Source: "РУКОВОДСТВО.md"; DestDir: "{app}\docs"; Flags: ignoreversion
Source: "README.md"; DestDir: "{app}\docs"; Flags: ignoreversion
Source: "examples\*"; DestDir: "{app}\examples"; Flags: ignoreversion recursesubdirs

[Icons]
Name: "{group}\Простой REPL"; Filename: "{app}\prostoy.exe"
Name: "{group}\Документация"; Filename: "{app}\docs\РУКОВОДСТВО.md"
Name: "{group}\Удалить Простой"; Filename: "{uninstallexe}"
Name: "{autodesktop}\Простой"; Filename: "{app}\prostoy.exe"; Tasks: desktopicon

[Registry]
Root: HKLM; Subkey: "SYSTEM\CurrentControlSet\Control\Session Manager\Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; Tasks: addtopath; Check: NeedsAddPath('{app}')

[Code]
function NeedsAddPath(Param: string): boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue(HKEY_LOCAL_MACHINE,
    'SYSTEM\CurrentControlSet\Control\Session Manager\Environment',
    'Path', OrigPath)
  then begin
    Result := True;
    exit;
  end;
  Result := Pos(';' + Param + ';', ';' + OrigPath + ';') = 0;
end;

[Run]
Filename: "{app}\prostoy.exe"; Description: "Запустить Простой REPL"; Flags: nowait postinstall skipifsilent
