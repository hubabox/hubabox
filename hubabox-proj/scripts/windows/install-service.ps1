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

.PARAMETER HubConfigFile
  Path to KEY=value file (LISTEN, DATADIR, MDNS, MDNS_NAME, PUBLIC_ORIGIN, IMPORT). Relative paths resolve from the script directory.

.PARAMETER FirewallRuleName
  Windows Defender Firewall rule display name (default: HubaBox HTTP).

.PARAMETER IncludePublicProfile
  If set, firewall rule also applies on "Public" networks (e.g. some guest Wi‑Fi).
  Default: Private + Domain only (safer on laptops).
#>
[CmdletBinding()]
param(
  [string]$ExePath = "",
  [int]$ListenPort = 8787,
  [string]$Listen = "",
  [string]$DataDir = $(Join-Path $env:ProgramData "HubaBox"),
  [switch]$MdnsOff,
  [switch]$MdnsOn,
  [string]$MdnsName = "",
  [string]$PublicOrigin = "",
  [string]$ImportDir = "",
  [string]$HubConfigFile = "",
  [string]$FirewallRuleName = "HubaBox HTTP",
  [switch]$IncludePublicProfile
)

$ErrorActionPreference = "Stop"

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

if ($vMdnsOff -and $vMdnsOn) {
  throw "Config resolves to both mDNS off and on; fix MDNS= in file or remove conflicting switches."
}

if (-not $ExePath) {
  $ExePath = Join-Path $PSScriptRoot "hubabox.exe"
}
if (-not (Test-Path -LiteralPath $ExePath)) {
  throw "hubabox.exe not found at: $ExePath"
}
$ExePath = (Resolve-Path -LiteralPath $ExePath).Path

$svcName = "HubaBox"
$existing = Get-Service -Name $svcName -ErrorAction SilentlyContinue
if ($existing) {
  Write-Host "Service $svcName already exists; replacing it..."
  if ($existing.Status -ne "Stopped") {
    Write-Host "Stopping existing service..."
    Stop-Service -Name $svcName -Force
    Wait-ServiceStatus -Name $svcName -DesiredStatus "Stopped"
  }
  & sc.exe delete $svcName | Out-Null
  Wait-ServiceRemoved -Name $svcName
}

New-Item -ItemType Directory -Force -Path $vDataDir | Out-Null

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

# If you double-clicked hubabox.exe earlier, it may still be listening on this port and block the service.
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

Write-Host "Starting service..."
try {
  Start-Service -Name $svcName -ErrorAction Stop
} catch {
  throw @"
Start-Service failed: $_

Common causes: hubabox.exe blocked by antivirus, bad path in the service, or the binary exits on startup.
Try running manually (elevated CMD or PowerShell) to see the real error:
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
Test the same command line the service uses:
  & `"$ExePath`" -listen $listenArg -data `"$vDataDir`"
Then check Event Viewer → Windows Logs → Application (source: hubabox or Application Error).
"@
}

Write-Host "Done. Open http://<this-pc-ip>:$fwPort/ on your LAN (or /library for guests)."
