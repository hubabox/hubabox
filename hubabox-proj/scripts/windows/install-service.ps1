#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Install HubaBox as a Windows service and allow inbound HTTP on the LAN.

.DESCRIPTION
  Place hubabox.exe next to this script (or pass -ExePath). Creates data directory,
  firewall rule, and service "HubaBox". Optional -HubConfigFile reads KEY=value lines
  (see hubabox-config.example.txt). Explicit script parameters override the config file.

.PARAMETER ExePath
  Full path to hubabox.exe. Default: hubabox.exe beside this script.

.PARAMETER InstallDir
  Stable program directory. The executable is copied here before the service is
  registered. Default: $env:ProgramFiles\HubaBox.

.PARAMETER ListenPort
  TCP port when not using -Listen (default 8787). Becomes -listen :PORT unless -Listen or LISTEN= in config.

.PARAMETER Listen
  Full -listen value for hubabox (e.g. :8788, 127.0.0.1:8787). Overrides -ListenPort and config LISTEN when set.

.PARAMETER DataDir
  Database and files root. Default: $env:ProgramData\HubaBox

.PARAMETER MdnsOff
  Pass -mdns=false to the service binary.

.PARAMETER MdnsOn
  Pass -mdns=true explicitly (default hubabox behavior is mDNS on when unset).

.PARAMETER MdnsName
  mDNS instance name (-mdns-name).

.PARAMETER PublicOrigin
  Base URL for library invite links (-public-origin), e.g. http://192.168.0.7:8787

.PARAMETER ImportDir
  Optional import/watch folder (-import).

.PARAMETER AllowRemoteSetup
  Permit first-time administrator setup from another LAN device. Off by
  default; local setup at http://127.0.0.1:PORT is safer.

.PARAMETER TlsCertPath / TlsKeyPath
  Optional PEM certificate and matching private key. When both are supplied,
  they are copied into the hub data directory with service-only access and
  hubaBox serves HTTPS (needed for browser microphone recording on the LAN).

.PARAMETER HubConfigFile
  Path to KEY=value file (LISTEN, DATADIR, MDNS, MDNS_NAME, PUBLIC_ORIGIN,
  IMPORT, ALLOW_REMOTE_SETUP, TLS_CERT, TLS_KEY). Relative paths resolve from
  the script directory.

.PARAMETER FirewallRuleName
  Windows Defender Firewall rule display name (default: HubaBox HTTP).

.PARAMETER IncludePublicProfile
  If set, firewall rule also applies on "Public" networks (e.g. some guest Wi‑Fi).
  Default: Private + Domain only (safer on laptops).
#>
[CmdletBinding()]
param(
  [string]$ExePath = "",
  [string]$InstallDir = $(Join-Path $env:ProgramFiles "HubaBox"),
  [int]$ListenPort = 8787,
  [string]$Listen = "",
  [string]$DataDir = $(Join-Path $env:ProgramData "HubaBox"),
  [switch]$MdnsOff,
  [switch]$MdnsOn,
  [string]$MdnsName = "",
  [string]$PublicOrigin = "",
  [string]$ImportDir = "",
  [switch]$AllowRemoteSetup,
  [string]$TlsCertPath = "",
  [string]$TlsKeyPath = "",
  [string]$HubConfigFile = "",
  [string]$FirewallRuleName = "HubaBox HTTP",
  [switch]$IncludePublicProfile
)

$ErrorActionPreference = "Stop"
$script:TranscriptStarted = $false
$script:InstallLogPath = ""
$script:ResolvedDataDir = ""
$script:ResolvedInstallDir = ""
$script:ResolvedListenPort = 8787

function Start-InstallTranscript {
  $logDir = Join-Path $env:ProgramData "HubaBox"
  try {
    New-Item -ItemType Directory -Force -Path $logDir | Out-Null
    $script:InstallLogPath = Join-Path $logDir "install.log"
    Start-Transcript -Path $script:InstallLogPath -Append -Force | Out-Null
    $script:TranscriptStarted = $true
  } catch {
    Write-Warning "Could not start installer transcript: $_"
  }
}

function Stop-InstallTranscript {
  if (-not $script:TranscriptStarted) { return }
  try { Stop-Transcript | Out-Null } catch {}
  $script:TranscriptStarted = $false
}

function Assert-SupportedWindows {
  if (-not [Environment]::Is64BitOperatingSystem) {
    throw "This release requires 64-bit Windows (amd64/x64)."
  }
  try {
    $os = Get-CimInstance -ClassName Win32_OperatingSystem -ErrorAction Stop
    $version = [version]$os.Version
    Write-Host "Windows: $($os.Caption) $($os.Version); 64-bit=$([Environment]::Is64BitOperatingSystem)"
    if ($version -lt [version]"10.0") {
      throw "This HubaBox build requires Windows 10 or Windows Server 2016 and newer; detected $($os.Caption) $($os.Version)."
    }
  } catch {
    if ($_.Exception.Message -like "This HubaBox build requires*") { throw }
    Write-Warning "Could not verify the Windows version: $_"
  }
}

function Wait-HubHealth {
  param(
    [Parameter(Mandatory = $true)][string]$ServiceName,
    [Parameter(Mandatory = $true)][string]$HealthUrl,
    [int]$TimeoutSeconds = 45
  )
  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  $lastError = "no response"
  while ((Get-Date) -lt $deadline) {
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if (-not $svc) { throw "Service '$ServiceName' disappeared during startup." }
    if ($svc.Status -eq "Stopped") { break }
    try {
      $resp = Invoke-WebRequest -Uri $HealthUrl -UseBasicParsing -TimeoutSec 4 -ErrorAction Stop
      $body = ($resp.Content | Out-String).Trim()
      if ($resp.StatusCode -eq 200 -and $body -eq "ok") { return }
      $lastError = "HTTP $($resp.StatusCode), body '$body'"
    } catch {
      $lastError = $_.Exception.Message
    }
    Start-Sleep -Milliseconds 500
  }
  throw "HubaBox did not pass GET $HealthUrl within $TimeoutSeconds seconds (last result: $lastError)."
}

trap {
  $message = ($_ | Out-String).Trim()
  Write-Host ""
  Write-Host "HubaBox installation FAILED: $message" -ForegroundColor Red
  if ($script:ResolvedDataDir) {
    $serviceLog = Join-Path $script:ResolvedDataDir "hubabox.log"
    if (Test-Path -LiteralPath $serviceLog -PathType Leaf) {
      Write-Host ""
      Write-Host "Last service log lines ($serviceLog):" -ForegroundColor Yellow
      Get-Content -LiteralPath $serviceLog -Tail 80 -ErrorAction SilentlyContinue | ForEach-Object { Write-Host $_ }
    }
  }
  if ($script:InstallLogPath) {
    Write-Host "Installer transcript: $($script:InstallLogPath)" -ForegroundColor Yellow
  }
  Stop-InstallTranscript
  $collector = Join-Path $PSScriptRoot "collect-diagnostics.ps1"
  if (Test-Path -LiteralPath $collector -PathType Leaf) {
    try {
      $diagDataDir = if ($script:ResolvedDataDir) { $script:ResolvedDataDir } else { Join-Path $env:ProgramData "HubaBox" }
      $diagInstallDir = if ($script:ResolvedInstallDir) { $script:ResolvedInstallDir } else { Join-Path $env:ProgramFiles "HubaBox" }
      Write-Host "Collecting a diagnostics zip..." -ForegroundColor Yellow
      & $collector -ServiceName "HubaBox" -ListenPort $script:ResolvedListenPort -DataDir $diagDataDir -InstallDir $diagInstallDir | Out-Host
    } catch {
      Write-Warning "Automatic diagnostics collection failed: $_"
    }
  }
  exit 1
}

Start-InstallTranscript
Write-Host "HubaBox Windows service installer started at $(Get-Date -Format o)"

function Assert-Admin {
  $principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
  if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "Run this script in an elevated PowerShell (Run as Administrator)."
  }
}

function Wait-ServiceRemoved {
  param(
    [Parameter(Mandatory = $true)][string]$Name,
    [int]$TimeoutSeconds = 20
  )
  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    $svc = Get-Service -Name $Name -ErrorAction SilentlyContinue
    if (-not $svc) { return }
    Start-Sleep -Milliseconds 400
  }
  throw "Timed out waiting for service '$Name' removal."
}

function Wait-ServiceStatus {
  param(
    [Parameter(Mandatory = $true)][string]$Name,
    [Parameter(Mandatory = $true)][string]$DesiredStatus,
    [int]$TimeoutSeconds = 20
  )
  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    $svc = Get-Service -Name $Name -ErrorAction SilentlyContinue
    if ($svc -and $svc.Status.ToString() -eq $DesiredStatus) { return }
    Start-Sleep -Milliseconds 400
  }
  throw "Timed out waiting for service '$Name' status '$DesiredStatus'."
}

function Remove-PreviousInstallation {
  param(
    [Parameter(Mandatory = $true)][string]$ServiceName,
    [Parameter(Mandatory = $true)][string]$RequestedFirewallRuleName
  )
  $existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
  if ($existing) {
    $oldImagePath = ""
    try {
      $oldService = Get-CimInstance -ClassName Win32_Service -Filter "Name='$ServiceName'" -ErrorAction Stop
      $oldImagePath = $oldService.PathName
    } catch {}
    Write-Host "Previous HubaBox service found; uninstalling it before upgrade..."
    if ($oldImagePath) { Write-Host "  Previous ImagePath: $oldImagePath" }
    if ($existing.Status -ne "Stopped") {
      Write-Host "Stopping previous service..."
      Stop-Service -Name $ServiceName -Force
      Wait-ServiceStatus -Name $ServiceName -DesiredStatus "Stopped"
    }
    & sc.exe delete $ServiceName | Out-Null
    if ($LASTEXITCODE -ne 0) {
      throw "Could not delete previous service '$ServiceName' (sc.exe exit $LASTEXITCODE)."
    }
    Wait-ServiceRemoved -Name $ServiceName
    Write-Host "Previous service removed. Existing hub data will be preserved."
  } else {
    Write-Host "No previous HubaBox service was found."
  }

  $oldRuleNames = @($RequestedFirewallRuleName, "HubaBox HTTP") | Select-Object -Unique
  foreach ($ruleName in $oldRuleNames) {
    Remove-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue | Out-Null
  }
  Write-Host "Previous HubaBox firewall rule removed if present."
}

function Read-HubConfigFile {
  param([Parameter(Mandatory = $true)][string]$Path)
  $map = @{}
  Get-Content -LiteralPath $Path | ForEach-Object {
    $line = $_.Trim()
    if ($line -match '^\s*#' -or $line -eq "") { return }
    $eq = $line.IndexOf("=")
    if ($eq -lt 1) { throw "Invalid config line (expected KEY=value): $line" }
    $k = $line.Substring(0, $eq).Trim().ToUpperInvariant()
    $v = $line.Substring($eq + 1).Trim()
    if ($k) { $map[$k] = $v }
  }
  return $map
}

function Normalize-ListenArg {
  param([string]$Raw)
  $s = $Raw.Trim()
  if ($s -match '^\d+$') { return ":$s" }
  return $s
}

function TcpPortFromListenArg {
  param([string]$ListenArg)
  if ($ListenArg -match ':(\d+)\s*$') { return [int]$Matches[1] }
  if ($ListenArg -match '^(\d+)$') { return [int]$Matches[1] }
  throw "Could not infer TCP port from -listen value '$ListenArg' for firewall rule."
}

function Quote-ScArg {
  param([string]$Value)
  if ($null -eq $Value) { return '""' }
  $v = $Value.Trim()
  if ($v -match '[\s"]') {
    return '"' + ($v -replace '"', '""') + '"'
  }
  return $v
}

Assert-Admin
Assert-SupportedWindows

if ($MdnsOff -and $MdnsOn) {
  throw "Use only one of -MdnsOff or -MdnsOn."
}

# --- Merge optional hub config file (explicit parameters win) ---
$cfg = @{}
if (-not [string]::IsNullOrWhiteSpace($HubConfigFile)) {
  $cfgPath = $HubConfigFile.Trim()
  if (-not [System.IO.Path]::IsPathRooted($cfgPath)) {
    $cfgPath = Join-Path $PSScriptRoot $cfgPath
  }
  if (-not (Test-Path -LiteralPath $cfgPath)) {
    throw "Hub config file not found: $cfgPath"
  }
  $cfg = Read-HubConfigFile -Path $cfgPath
}

$vDataDir = $DataDir
if (-not $PSBoundParameters.ContainsKey("DataDir") -and $cfg.ContainsKey("DATADIR")) {
  $vDataDir = $cfg["DATADIR"].Trim()
}
if ([string]::IsNullOrWhiteSpace($vDataDir)) {
  throw "DataDir / DATADIR must not be empty."
}
$script:ResolvedDataDir = [System.IO.Path]::GetFullPath($vDataDir)
$vDataDir = $script:ResolvedDataDir

if ([string]::IsNullOrWhiteSpace($InstallDir)) {
  throw "InstallDir must not be empty."
}
$vInstallDir = [System.IO.Path]::GetFullPath($InstallDir.Trim())
$script:ResolvedInstallDir = $vInstallDir
$installRoot = [System.IO.Path]::GetPathRoot($vInstallDir)
if ($vInstallDir.TrimEnd('\') -eq $installRoot.TrimEnd('\')) {
  throw "InstallDir must be a dedicated subdirectory, not drive root '$installRoot'."
}

# LISTEN from file only if caller did not pin -Listen or -ListenPort on the command line.
$fileListen = ""
if ($cfg.ContainsKey("LISTEN") -and -not $PSBoundParameters.ContainsKey("Listen") -and -not $PSBoundParameters.ContainsKey("ListenPort")) {
  $fileListen = Normalize-ListenArg $cfg["LISTEN"]
}

$vMdnsOff = [bool]$MdnsOff
$vMdnsOn = [bool]$MdnsOn
if (-not $PSBoundParameters.ContainsKey("MdnsOff") -and -not $PSBoundParameters.ContainsKey("MdnsOn") -and $cfg.ContainsKey("MDNS")) {
  $m = $cfg["MDNS"].Trim().ToLowerInvariant()
  if ($m -in @("0", "false", "off", "no")) { $vMdnsOff = $true }
  elseif ($m -in @("1", "true", "on", "yes")) { $vMdnsOn = $true }
}

$vMdnsName = $MdnsName.Trim()
if (-not $PSBoundParameters.ContainsKey("MdnsName") -and $cfg.ContainsKey("MDNS_NAME")) {
  $vMdnsName = $cfg["MDNS_NAME"].Trim()
}

$vPublicOrigin = $PublicOrigin.Trim()
if (-not $PSBoundParameters.ContainsKey("PublicOrigin") -and $cfg.ContainsKey("PUBLIC_ORIGIN")) {
  $vPublicOrigin = $cfg["PUBLIC_ORIGIN"].Trim()
}

$vImportDir = $ImportDir.Trim()
if (-not $PSBoundParameters.ContainsKey("ImportDir") -and $cfg.ContainsKey("IMPORT")) {
  $vImportDir = $cfg["IMPORT"].Trim()
}

$vAllowRemoteSetup = [bool]$AllowRemoteSetup
if (-not $PSBoundParameters.ContainsKey("AllowRemoteSetup") -and $cfg.ContainsKey("ALLOW_REMOTE_SETUP")) {
  $remoteSetupRaw = $cfg["ALLOW_REMOTE_SETUP"].Trim().ToLowerInvariant()
  if ($remoteSetupRaw -in @("1", "true", "on", "yes")) { $vAllowRemoteSetup = $true }
  elseif ($remoteSetupRaw -in @("0", "false", "off", "no")) { $vAllowRemoteSetup = $false }
  else { throw "ALLOW_REMOTE_SETUP must be true/false, on/off, yes/no, or 1/0." }
}

$vTlsCertPath = $TlsCertPath.Trim()
if (-not $PSBoundParameters.ContainsKey("TlsCertPath") -and $cfg.ContainsKey("TLS_CERT")) {
  $vTlsCertPath = $cfg["TLS_CERT"].Trim()
}
$vTlsKeyPath = $TlsKeyPath.Trim()
if (-not $PSBoundParameters.ContainsKey("TlsKeyPath") -and $cfg.ContainsKey("TLS_KEY")) {
  $vTlsKeyPath = $cfg["TLS_KEY"].Trim()
}
if ([string]::IsNullOrWhiteSpace($vTlsCertPath) -xor [string]::IsNullOrWhiteSpace($vTlsKeyPath)) {
  throw "HTTPS needs both -TlsCertPath and -TlsKeyPath (or TLS_CERT and TLS_KEY in the config file)."
}

# Final -listen value and firewall port
$listenArg = ""
if ($PSBoundParameters.ContainsKey("Listen") -and -not [string]::IsNullOrWhiteSpace($Listen)) {
  $listenArg = Normalize-ListenArg $Listen
}
elseif (-not [string]::IsNullOrWhiteSpace($fileListen)) {
  $listenArg = $fileListen
}
else {
  if ($ListenPort -lt 1 -or $ListenPort -gt 65535) {
    throw "ListenPort must be between 1 and 65535."
  }
  $listenArg = ":$ListenPort"
}

if ([string]::IsNullOrWhiteSpace($listenArg)) {
  throw "Resolved -listen value is empty."
}
$fwPort = TcpPortFromListenArg $listenArg
$script:ResolvedListenPort = $fwPort

if ($vMdnsOff -and $vMdnsOn) {
  throw "Config resolves to both mDNS off and on; fix MDNS= in file or remove conflicting switches."
}

if (-not $ExePath) {
  $ExePath = Join-Path $PSScriptRoot "hubabox.exe"
}
if (-not (Test-Path -LiteralPath $ExePath)) {
  throw "hubabox.exe not found at: $ExePath"
}
$sourceExePath = (Resolve-Path -LiteralPath $ExePath).Path
$signature = Get-AuthenticodeSignature -LiteralPath $sourceExePath -ErrorAction SilentlyContinue
if ($signature -and $signature.Status -eq "Valid") {
  Write-Host "Executable signature: Valid ($($signature.SignerCertificate.Subject))"
} else {
  $signatureStatus = if ($signature) { $signature.Status.ToString() } else { "Unavailable" }
  Write-Warning "Executable signature status is $signatureStatus. Windows Smart App Control or an organization policy may block this unsigned pilot build."
}

$svcName = "HubaBox"
Remove-PreviousInstallation -ServiceName $svcName -RequestedFirewallRuleName $FirewallRuleName

# Check the port after the old service is gone, but before creating the new
# service or firewall rule. A failed preflight therefore does not leave a
# partial installation behind.
$listeners = @()
try {
  $listeners = @(Get-NetTCPConnection -LocalPort $fwPort -State Listen -ErrorAction SilentlyContinue)
} catch {
  Write-Warning "Could not check if port $fwPort is free (continuing): $_"
}
if ($listeners.Count -gt 0) {
  $seen = @{}
  $lines = @()
  foreach ($l in $listeners) {
    $opid = $l.OwningProcess
    if ($seen.ContainsKey($opid)) { continue }
    $seen[$opid] = $true
    $proc = Get-Process -Id $opid -ErrorAction SilentlyContinue
    if ($proc) {
      $pathInfo = ""
      try {
        if ($proc.Path) { $pathInfo = " — $($proc.Path)" }
      } catch {}
      $lines += "  PID $opid — $($proc.ProcessName)$pathInfo"
    } else {
      $lines += "  PID $opid (process details unavailable)"
    }
  }
  throw "TCP port $fwPort is already in use. Close whatever is listening — often a hubabox.exe / console window opened by double-clicking the exe before install.`n$($lines -join "`n")`n`nThen re-run this script."
}

New-Item -ItemType Directory -Force -Path $vDataDir | Out-Null
New-Item -ItemType Directory -Force -Path $vInstallDir | Out-Null

$installedExePath = Join-Path $vInstallDir "hubabox.exe"
$sameExePath = [string]::Equals($sourceExePath, $installedExePath, [System.StringComparison]::OrdinalIgnoreCase)
if (-not $sameExePath) {
  $stagedExePath = Join-Path $vInstallDir "hubabox.exe.new"
  Remove-Item -LiteralPath $stagedExePath -Force -ErrorAction SilentlyContinue
  Copy-Item -LiteralPath $sourceExePath -Destination $stagedExePath -Force
  Unblock-File -LiteralPath $stagedExePath -ErrorAction SilentlyContinue
  $sourceHash = (Get-FileHash -LiteralPath $sourceExePath -Algorithm SHA256).Hash
  $stagedHash = (Get-FileHash -LiteralPath $stagedExePath -Algorithm SHA256).Hash
  if ($sourceHash -ne $stagedHash) {
    Remove-Item -LiteralPath $stagedExePath -Force -ErrorAction SilentlyContinue
    throw "Installed executable checksum mismatch while copying to $vInstallDir."
  }
  Move-Item -LiteralPath $stagedExePath -Destination $installedExePath -Force
} else {
  Unblock-File -LiteralPath $installedExePath -ErrorAction SilentlyContinue
}
$ExePath = (Resolve-Path -LiteralPath $installedExePath).Path
Write-Host "Installed executable: $ExePath"
Write-Host "Executable SHA256: $((Get-FileHash -LiteralPath $ExePath -Algorithm SHA256).Hash)"

$serviceTlsCert = ""
$serviceTlsKey = ""
if (-not [string]::IsNullOrWhiteSpace($vTlsCertPath)) {
  if (-not (Test-Path -LiteralPath $vTlsCertPath -PathType Leaf)) { throw "TLS certificate not found: $vTlsCertPath" }
  if (-not (Test-Path -LiteralPath $vTlsKeyPath -PathType Leaf)) { throw "TLS private key not found: $vTlsKeyPath" }
  $tlsDir = Join-Path $vDataDir "tls"
  New-Item -ItemType Directory -Force -Path $tlsDir | Out-Null
  & icacls.exe $tlsDir /inheritance:r /grant:r "SYSTEM:(OI)(CI)F" "Administrators:(OI)(CI)F" | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "Could not restrict access to TLS files in $tlsDir." }
  $serviceTlsCert = Join-Path $tlsDir "hubabox-cert.pem"
  $serviceTlsKey = Join-Path $tlsDir "hubabox-key.pem"
  Copy-Item -LiteralPath $vTlsCertPath -Destination $serviceTlsCert -Force
  Copy-Item -LiteralPath $vTlsKeyPath -Destination $serviceTlsKey -Force
}

$parts = @()
$parts += '"' + $ExePath.Replace('"', '""') + '"'
$parts += "-listen"
$parts += (Quote-ScArg $listenArg)
$parts += "-data"
$parts += (Quote-ScArg $vDataDir)

if ($vMdnsOff) {
  $parts += "-mdns=false"
}
elseif ($vMdnsOn) {
  $parts += "-mdns=true"
}

if (-not [string]::IsNullOrWhiteSpace($vMdnsName)) {
  $parts += "-mdns-name"
  $parts += (Quote-ScArg $vMdnsName)
}

if (-not [string]::IsNullOrWhiteSpace($vPublicOrigin)) {
  if ($vPublicOrigin.Length -gt 512) { throw "PUBLIC_ORIGIN / -PublicOrigin is too long." }
  $parts += "-public-origin"
  $parts += (Quote-ScArg $vPublicOrigin)
}

if (-not [string]::IsNullOrWhiteSpace($vImportDir)) {
  $parts += "-import"
  $parts += (Quote-ScArg $vImportDir)
}

if ($vAllowRemoteSetup) {
  $parts += "-allow-remote-setup"
  Write-Warning "Remote first-time setup is enabled. Complete setup promptly on a trusted LAN."
}

if ($serviceTlsCert) {
  $parts += "-tls-cert"
  $parts += (Quote-ScArg $serviceTlsCert)
  $parts += "-tls-key"
  $parts += (Quote-ScArg $serviceTlsKey)
}

$binaryPathName = ($parts -join " ")

Write-Host "Creating service $svcName ..."
Write-Host "  ImagePath (summary): $ExePath -listen $listenArg -data <DataDir> ..."
try {
  New-Service -Name $svcName -BinaryPathName $binaryPathName -DisplayName "HubaBox" `
    -StartupType Automatic | Out-Null
} catch {
  throw "New-Service failed: $_ (try running from elevated PowerShell)"
}

# If the service crashes, try restarting it up to 3 times before giving up.
# Ignore sc.exe errors (syntax/locale differences); non-zero exit does not fail install.
& sc.exe failure $svcName reset= 86400 actions= restart/5000/restart/15000/restart/30000 2>$null | Out-Null
& sc.exe failureflag $svcName 1 2>$null | Out-Null

$profiles = if ($IncludePublicProfile) { @("Private", "Domain", "Public") } else { @("Private", "Domain") }
Write-Host "Adding firewall rule $FirewallRuleName (TCP $fwPort, profiles: $($profiles -join ', ')) ..."
Remove-NetFirewallRule -DisplayName $FirewallRuleName -ErrorAction SilentlyContinue | Out-Null
try {
  New-NetFirewallRule -DisplayName $FirewallRuleName -Direction Inbound `
    -Action Allow -Protocol TCP -LocalPort $fwPort -Profile $profiles | Out-Null
} catch {
  Write-Warning "Firewall rule with profiles [$($profiles -join ', ')] failed: $_"
  Write-Host "Retrying with Private profile only..."
  New-NetFirewallRule -DisplayName $FirewallRuleName -Direction Inbound `
    -Action Allow -Protocol TCP -LocalPort $fwPort -Profile Private | Out-Null
}

Write-Host "Starting service..."
try {
  Start-Service -Name $svcName -ErrorAction Stop
} catch {
  throw @"
Start-Service failed: $_

Common causes: hubabox.exe blocked by antivirus, bad path in the service, or the binary exits on startup.
Check the service log file for the exact error:
  "$vDataDir\hubabox.log"   (fallback: "$env:TEMP\hubabox.log")
Or run manually (elevated CMD or PowerShell) to see the error live:
  & `"$ExePath`" -listen $listenArg -data `"$vDataDir`"
Also open services.msc → HubaBox → see if Windows shows a service-specific error code.
"@
}

try {
  Wait-ServiceStatus -Name $svcName -DesiredStatus "Running" -TimeoutSeconds 45
} catch {
  $st = "unknown"
  $svcNow = Get-Service -Name $svcName -ErrorAction SilentlyContinue
  if ($svcNow) { $st = $svcNow.Status.ToString() }
  throw @"
$_

The HubaBox service did not stay Running (current status: $st). It may be crashing on startup.
Check the service log file for the exact error:
  "$vDataDir\hubabox.log"   (fallback: "$env:TEMP\hubabox.log")
Or test the same command line the service uses:
  & `"$ExePath`" -listen $listenArg -data `"$vDataDir`"
services.msc → HubaBox will show the service-specific exit code the hub reported.
"@
}

$scheme = if ($serviceTlsCert) { "https" } else { "http" }
Write-Host "Checking $scheme health endpoint..."
Wait-HubHealth -ServiceName $svcName -HealthUrl "${scheme}://127.0.0.1:$fwPort/health" -TimeoutSeconds 45
Write-Host "Health check passed: ${scheme}://127.0.0.1:$fwPort/health -> 200 ok" -ForegroundColor Green

$publicProfiles = @(Get-NetConnectionProfile -ErrorAction SilentlyContinue | Where-Object { $_.NetworkCategory -eq "Public" })
if (-not $IncludePublicProfile -and $publicProfiles.Count -gt 0) {
  Write-Warning "This PC has a Public network profile, but the firewall rule allows Private/Domain only. Local access will work; LAN access may require changing the trusted network to Private or reinstalling with -IncludePublicProfile."
}

Write-Host "Done. Open ${scheme}://<this-pc-ip>:$fwPort/ on your LAN (or /library for guests)."
Write-Host "Installer transcript: $($script:InstallLogPath)"
Write-Host "Service log: $(Join-Path $vDataDir 'hubabox.log')"
Stop-InstallTranscript
