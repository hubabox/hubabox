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

$svc = Get-Service -Name $svcName -ErrorAction SilentlyContinue
if ($svc) {
  if ($svc.Status -ne "Stopped") {
    Stop-Service -Name $svcName -Force
  }
  & sc.exe delete $svcName
  if ($LASTEXITCODE -ne 0) {
    Write-Warning "sc delete returned $LASTEXITCODE"
  }
} else {
  Write-Host "Service $svcName not found."
}

Remove-NetFirewallRule -DisplayName $FirewallRuleName -ErrorAction SilentlyContinue | Out-Null
Write-Host "Removed firewall rule '$FirewallRuleName' if present."
