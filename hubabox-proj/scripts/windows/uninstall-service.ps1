#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Remove HubaBox Windows service and its firewall rule.

.PARAMETER FirewallRuleName
  Must match the name used at install (default: HubaBox HTTP).
#>
param(
  [string]$FirewallRuleName = "HubaBox HTTP"
)

$ErrorActionPreference = "Stop"
$svcName = "HubaBox"

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

Assert-Admin

$svc = Get-Service -Name $svcName -ErrorAction SilentlyContinue
if ($svc) {
  if ($svc.Status -ne "Stopped") {
    Stop-Service -Name $svcName -Force
  }
  & sc.exe delete $svcName
  if ($LASTEXITCODE -ne 0) {
    Write-Warning "sc delete returned $LASTEXITCODE"
  } else {
    Wait-ServiceRemoved -Name $svcName
  }
} else {
  Write-Host "Service $svcName not found."
}

Remove-NetFirewallRule -DisplayName $FirewallRuleName -ErrorAction SilentlyContinue | Out-Null
Write-Host "Removed firewall rule '$FirewallRuleName' if present."
