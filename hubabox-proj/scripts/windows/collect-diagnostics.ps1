<#
.SYNOPSIS
  Collect a privacy-conscious HubaBox Windows diagnostics zip.

.DESCRIPTION
  Captures OS/architecture, service configuration and status, firewall and
  listener state, the local /health result, executable hash/signature, HubaBox
  logs, and relevant recent Windows service/code-integrity events. It does not
  include the SQLite database, uploaded files, TLS private keys, or passwords.
#>
[CmdletBinding()]
param(
  [string]$ServiceName = "HubaBox",
  [int]$ListenPort = 8787,
  [string]$DataDir = $(Join-Path $env:ProgramData "HubaBox"),
  [string]$InstallDir = $(Join-Path $env:ProgramFiles "HubaBox"),
  [string]$OutputDirectory = ""
)

$ErrorActionPreference = "Continue"
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
if ([string]::IsNullOrWhiteSpace($OutputDirectory)) {
  $desktop = [Environment]::GetFolderPath("Desktop")
  $OutputDirectory = if ($desktop -and (Test-Path -LiteralPath $desktop)) { $desktop } else { $env:TEMP }
}
New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$workDir = Join-Path $env:TEMP "HubaBox-diagnostics-$stamp"
$zipPath = Join-Path $OutputDirectory "HubaBox-diagnostics-$stamp.zip"
New-Item -ItemType Directory -Force -Path $workDir | Out-Null
$reportPath = Join-Path $workDir "report.txt"

function Add-ReportLine {
  param([string]$Text = "")
  $Text | Out-File -LiteralPath $reportPath -Encoding utf8 -Append
}

function Capture-ReportSection {
  param(
    [Parameter(Mandatory = $true)][string]$Title,
    [Parameter(Mandatory = $true)][scriptblock]$Command
  )
  Add-ReportLine ""
  Add-ReportLine "===== $Title ====="
  try {
    $output = & $Command 2>&1 | Out-String -Width 240
    if ([string]::IsNullOrWhiteSpace($output)) { $output = "(no output)" }
    Add-ReportLine $output.TrimEnd()
  } catch {
    Add-ReportLine "ERROR: $(($_ | Out-String).Trim())"
  }
}

Add-ReportLine "HubaBox Windows diagnostics"
Add-ReportLine "Collected: $(Get-Date -Format o)"
Add-ReportLine "Service: $ServiceName"
Add-ReportLine "Expected port: $ListenPort"
Add-ReportLine "Expected data directory: $DataDir"
Add-ReportLine "Expected install directory: $InstallDir"
Add-ReportLine "Excluded intentionally: hubabox.db, uploaded files, TLS keys, passwords, and browser data."

Capture-ReportSection "identity and PowerShell" {
  $principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
  [pscustomobject]@{
    User = [Security.Principal.WindowsIdentity]::GetCurrent().Name
    IsAdministrator = $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    PowerShell = $PSVersionTable.PSVersion.ToString()
    ProcessArchitecture = $env:PROCESSOR_ARCHITECTURE
    NativeArchitecture = $env:PROCESSOR_ARCHITEW6432
    Is64BitOS = [Environment]::Is64BitOperatingSystem
    Is64BitProcess = [Environment]::Is64BitProcess
  } | Format-List
}

Capture-ReportSection "operating system" {
  Get-CimInstance -ClassName Win32_OperatingSystem |
    Select-Object Caption, Version, BuildNumber, OSArchitecture, LastBootUpTime |
    Format-List
}

$serviceInfo = Get-CimInstance -ClassName Win32_Service -Filter "Name='$ServiceName'" -ErrorAction SilentlyContinue
Capture-ReportSection "service" {
  Get-Service -Name $ServiceName -ErrorAction SilentlyContinue | Format-List *
  $serviceInfo | Select-Object Name, DisplayName, State, Status, StartMode, StartName, ExitCode, ServiceSpecificExitCode, PathName | Format-List
  & sc.exe qc $ServiceName
  & sc.exe queryex $ServiceName
  & sc.exe failure $ServiceName
}

$serviceExe = ""
if ($serviceInfo -and $serviceInfo.PathName) {
  $imagePath = $serviceInfo.PathName.Trim()
  if ($imagePath -match '^"([^"]+)"') { $serviceExe = $Matches[1] }
  elseif ($imagePath -match '^(\S+)') { $serviceExe = $Matches[1] }
}
if (-not $serviceExe) { $serviceExe = Join-Path $InstallDir "hubabox.exe" }

Capture-ReportSection "installed executable" {
  Write-Output "Resolved executable: $serviceExe"
  if (Test-Path -LiteralPath $serviceExe -PathType Leaf) {
    Get-Item -LiteralPath $serviceExe | Select-Object FullName, Length, CreationTimeUtc, LastWriteTimeUtc, VersionInfo | Format-List
    Get-FileHash -LiteralPath $serviceExe -Algorithm SHA256 | Format-List
    Get-AuthenticodeSignature -LiteralPath $serviceExe | Select-Object Status, StatusMessage, Path, SignerCertificate | Format-List
  } else {
    Write-Output "Executable not found."
  }
}

Capture-ReportSection "network profiles" {
  Get-NetConnectionProfile -ErrorAction SilentlyContinue |
    Select-Object Name, InterfaceAlias, NetworkCategory, IPv4Connectivity, IPv6Connectivity |
    Format-Table -AutoSize
}

Capture-ReportSection "firewall" {
  $rules = Get-NetFirewallRule -DisplayName "HubaBox*" -ErrorAction SilentlyContinue
  $rules | Select-Object DisplayName, Enabled, Direction, Action, Profile, PolicyStoreSourceType | Format-Table -AutoSize
  foreach ($rule in $rules) {
    $rule | Get-NetFirewallPortFilter -ErrorAction SilentlyContinue |
      Select-Object Protocol, LocalPort, RemotePort | Format-Table -AutoSize
  }
}

Capture-ReportSection "TCP listener" {
  Get-NetTCPConnection -LocalPort $ListenPort -ErrorAction SilentlyContinue |
    Select-Object State, LocalAddress, LocalPort, RemoteAddress, RemotePort, OwningProcess |
    Format-Table -AutoSize
}

Capture-ReportSection "health check" {
  $healthUrl = "http://127.0.0.1:$ListenPort/health"
  Write-Output "GET $healthUrl"
  $response = Invoke-WebRequest -Uri $healthUrl -UseBasicParsing -TimeoutSec 8 -ErrorAction Stop
  [pscustomobject]@{ StatusCode = $response.StatusCode; Body = ($response.Content | Out-String).Trim() } | Format-List
}

Capture-ReportSection "recent Service Control Manager events" {
  Get-WinEvent -FilterHashtable @{ LogName = "System"; ProviderName = "Service Control Manager"; StartTime = (Get-Date).AddDays(-7) } -ErrorAction SilentlyContinue |
    Where-Object { $_.Message -match "HubaBox" } |
    Select-Object -First 100 TimeCreated, Id, LevelDisplayName, Message |
    Format-List
}

Capture-ReportSection "recent Code Integrity events" {
  Get-WinEvent -LogName "Microsoft-Windows-CodeIntegrity/Operational" -MaxEvents 300 -ErrorAction SilentlyContinue |
    Where-Object { $_.Message -match "hubabox" } |
    Select-Object -First 100 TimeCreated, Id, LevelDisplayName, Message |
    Format-List
}

Capture-ReportSection "recent Microsoft Defender events" {
  Get-WinEvent -LogName "Microsoft-Windows-Windows Defender/Operational" -MaxEvents 300 -ErrorAction SilentlyContinue |
    Where-Object { $_.Message -match "hubabox" } |
    Select-Object -First 100 TimeCreated, Id, LevelDisplayName, Message |
    Format-List
}

$logCandidates = @(
  (Join-Path $env:ProgramData "HubaBox\install.log"),
  (Join-Path $env:ProgramData "HubaBox\uninstall.log"),
  (Join-Path $DataDir "hubabox.log"),
  (Join-Path $env:TEMP "hubabox.log"),
  (Join-Path $env:APPDATA "hubabox\hubabox.log")
) | Select-Object -Unique
foreach ($logPath in $logCandidates) {
  if (Test-Path -LiteralPath $logPath -PathType Leaf) {
    $safeName = ($logPath -replace '[:\\/ ]', '_').Trim('_')
    Copy-Item -LiteralPath $logPath -Destination (Join-Path $workDir $safeName) -Force -ErrorAction SilentlyContinue
  }
}

try {
  Compress-Archive -Path (Join-Path $workDir "*") -DestinationPath $zipPath -CompressionLevel Optimal -Force -ErrorAction Stop
  Remove-Item -LiteralPath $workDir -Recurse -Force -ErrorAction SilentlyContinue
  Write-Host "Diagnostics written to: $zipPath" -ForegroundColor Green
  Write-Output $zipPath
} catch {
  Write-Warning "Could not create diagnostics zip: $_"
  Write-Host "Uncompressed diagnostics remain at: $workDir" -ForegroundColor Yellow
  Write-Output $workDir
}
