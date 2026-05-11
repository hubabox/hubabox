#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Install HubaBox as a Windows service and allow inbound HTTP on the LAN.

.DESCRIPTION
  Place hubabox.exe next to this script (or pass -ExePath). Creates data directory,
  firewall rule, and service "HubaBox".

.PARAMETER ExePath
  Full path to hubabox.exe. Default: hubabox.exe beside this script.

.PARAMETER ListenPort
  TCP port for HTTP (default 8787).

.PARAMETER DataDir
  Database and files root. Default: $env:ProgramData\HubaBox

.PARAMETER FirewallRuleName
  Windows Defender Firewall rule display name (default: HubaBox HTTP).

.PARAMETER IncludePublicProfile
  If set, firewall rule also applies on "Public" networks (e.g. some guest Wi‑Fi).
  Default: Private + Domain only (safer on laptops).
#>
param(
  [string]$ExePath = "",
  [int]$ListenPort = 8787,
  [string]$DataDir = $(Join-Path $env:ProgramData "HubaBox"),
  [string]$FirewallRuleName = "HubaBox HTTP",
  [switch]$IncludePublicProfile
)

$ErrorActionPreference = "Stop"

if (-not $ExePath) {
  $ExePath = Join-Path $PSScriptRoot "hubabox.exe"
}
$ExePath = (Resolve-Path -LiteralPath $ExePath).Path
if (-not (Test-Path -LiteralPath $ExePath)) {
  throw "hubabox.exe not found at: $ExePath"
}

$svcName = "HubaBox"
$existing = Get-Service -Name $svcName -ErrorAction SilentlyContinue
if ($existing) {
  if ($existing.Status -ne "Stopped") {
    Stop-Service -Name $svcName -Force
  }
  & sc.exe delete $svcName | Out-Null
  Start-Sleep -Seconds 2
}

New-Item -ItemType Directory -Force -Path $DataDir | Out-Null

$listenArg = ":$ListenPort"
# Explicit -data so the service account does not rely on SYSTEM profile paths.
$binaryPathName = "`"$ExePath`" -listen $listenArg -data `"$DataDir`""

Write-Host "Creating service $svcName ..."
try {
  New-Service -Name $svcName -BinaryPathName $binaryPathName -DisplayName "HubaBox" `
    -StartupType Automatic | Out-Null
} catch {
  throw "New-Service failed: $_ (try running from elevated PowerShell)"
}

$profiles = if ($IncludePublicProfile) { "Private", "Domain", "Public" } else { "Private", "Domain" }
Write-Host "Adding firewall rule $FirewallRuleName (TCP $ListenPort, profiles: $($profiles -join ', ')) ..."
Remove-NetFirewallRule -DisplayName $FirewallRuleName -ErrorAction SilentlyContinue | Out-Null
New-NetFirewallRule -DisplayName $FirewallRuleName -Direction Inbound `
  -Action Allow -Protocol TCP -LocalPort $ListenPort -Profile $profiles | Out-Null

Write-Host "Starting service..."
Start-Service -Name $svcName

Write-Host "Done. Open http://<this-pc-ip>:$ListenPort/ on your LAN (or /library for guests)."
