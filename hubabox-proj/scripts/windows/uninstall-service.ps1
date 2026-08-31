#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Remove HubaBox Windows service and its firewall rule.

.PARAMETER FirewallRuleName
  Must match the name used at install (default: HubaBox HTTP).

.PARAMETER InstallDir
  Program directory used by the installer (default: $env:ProgramFiles\HubaBox).

.PARAMETER KeepProgramFiles
  Keep the installed hubabox.exe after removing the service.
#>
param(
  [string]$FirewallRuleName = "HubaBox HTTP",
  [string]$InstallDir = $(Join-Path $env:ProgramFiles "HubaBox"),
  [switch]$KeepProgramFiles
)

$ErrorActionPreference = "Stop"
$svcName = "HubaBox"
$script:TranscriptStarted = $false
$logDir = Join-Path $env:ProgramData "HubaBox"
try {
  New-Item -ItemType Directory -Force -Path $logDir | Out-Null
  $uninstallLog = Join-Path $logDir "uninstall.log"
  Start-Transcript -Path $uninstallLog -Append -Force | Out-Null
  $script:TranscriptStarted = $true
} catch {
  Write-Warning "Could not start uninstall transcript: $_"
}

trap {
  Write-Host "HubaBox uninstall FAILED: $(($_ | Out-String).Trim())" -ForegroundColor Red
  if ($script:TranscriptStarted) { try { Stop-Transcript | Out-Null } catch {} }
  exit 1
}

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
    $current = Get-Service -Name $Name -ErrorAction SilentlyContinue
    if ($current -and $current.Status.ToString() -eq $DesiredStatus) { return }
    Start-Sleep -Milliseconds 400
  }
  throw "Timed out waiting for service '$Name' status '$DesiredStatus'."
}

Assert-Admin

$svc = Get-Service -Name $svcName -ErrorAction SilentlyContinue
if ($svc) {
  if ($svc.Status -ne "Stopped") {
    Stop-Service -Name $svcName -Force
    Wait-ServiceStatus -Name $svcName -DesiredStatus "Stopped"
  }
  & sc.exe delete $svcName
  if ($LASTEXITCODE -ne 0) {
    throw "sc.exe delete returned $LASTEXITCODE"
  }
  Wait-ServiceRemoved -Name $svcName
} else {
  Write-Host "Service $svcName not found."
}

$ruleNames = @($FirewallRuleName, "HubaBox HTTP") | Select-Object -Unique
foreach ($ruleName in $ruleNames) {
  Remove-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue | Out-Null
}
Write-Host "Removed HubaBox firewall rule(s) if present."

if (-not $KeepProgramFiles) {
  if ([string]::IsNullOrWhiteSpace($InstallDir)) { throw "InstallDir must not be empty." }
  $resolvedInstallDir = [System.IO.Path]::GetFullPath($InstallDir.Trim())
  $installRoot = [System.IO.Path]::GetPathRoot($resolvedInstallDir)
  if ($resolvedInstallDir.TrimEnd('\') -eq $installRoot.TrimEnd('\')) {
    throw "Refusing to remove program files from drive root '$installRoot'."
  }
  $installedExe = Join-Path $resolvedInstallDir "hubabox.exe"
  Remove-Item -LiteralPath $installedExe -Force -ErrorAction SilentlyContinue
  if (Test-Path -LiteralPath $resolvedInstallDir -PathType Container) {
    $remaining = @(Get-ChildItem -LiteralPath $resolvedInstallDir -Force -ErrorAction SilentlyContinue)
    if ($remaining.Count -eq 0) {
      Remove-Item -LiteralPath $resolvedInstallDir -Force
    } else {
      Write-Warning "Program directory contains other files and was kept: $resolvedInstallDir"
    }
  }
  Write-Host "Removed installed executable if present: $installedExe"
}

Write-Host "Hub data was preserved in $logDir."
if ($script:TranscriptStarted) { Stop-Transcript | Out-Null }
