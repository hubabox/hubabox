<#
.SYNOPSIS
  Smoke-check a HubaBox Windows install (service, port, /health, optional firewall).

.DESCRIPTION
  Does not modify the system. Run from any PowerShell; firewall rule inspection
  requires elevation (otherwise that step is skipped with a note).

.PARAMETER ServiceName
  Windows service name (default: HubaBox).

.PARAMETER ListenPort
  TCP port the hub listens on (default: 8787).

.PARAMETER FirewallRuleName
  Display name of the install-time firewall rule (default: HubaBox HTTP).

.PARAMETER BaseUrl
  Full base URL for HTTP checks. Default: http://127.0.0.1:<ListenPort>
#>
param(
  [string]$ServiceName = "HubaBox",
  [int]$ListenPort = 8787,
  [string]$FirewallRuleName = "HubaBox HTTP",
  [string]$BaseUrl = ""
)

$ErrorActionPreference = "Stop"

function Test-IsAdministrator {
  $p = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
  return $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

if (-not $BaseUrl) {
  $BaseUrl = "http://127.0.0.1:$ListenPort"
}
$BaseUrl = $BaseUrl.TrimEnd("/")
$healthUrl = "$BaseUrl/health"

$failures = 0
$warn = @()

Write-Host "HubaBox verify-install (service=$ServiceName port=$ListenPort)" -ForegroundColor Cyan
Write-Host ""

# 1) Service
$svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if (-not $svc) {
  Write-Host "[FAIL] Service '$ServiceName' not found." -ForegroundColor Red
  $failures++
} elseif ($svc.Status -ne "Running") {
  Write-Host "[FAIL] Service '$ServiceName' status is $($svc.Status) (expected Running)." -ForegroundColor Red
  $failures++
} else {
  Write-Host "[ OK ] Service '$ServiceName' is Running." -ForegroundColor Green
}

# 2) TCP port (listening)
try {
  $listeners = Get-NetTCPConnection -LocalPort $ListenPort -State Listen -ErrorAction SilentlyContinue
  if (-not $listeners) {
    Write-Host "[FAIL] Nothing listening on TCP port $ListenPort." -ForegroundColor Red
    $failures++
  } else {
    Write-Host "[ OK ] TCP port $ListenPort is in Listen state." -ForegroundColor Green
  }
} catch {
  Write-Host "[WARN] Could not query TCP listeners: $_" -ForegroundColor Yellow
  $warn += "TCP check skipped or inconclusive."
}

# 3) HTTP /health
try {
  $resp = Invoke-WebRequest -Uri $healthUrl -UseBasicParsing -TimeoutSec 8 -ErrorAction Stop
  $body = ($resp.Content | Out-String).Trim()
  if ($resp.StatusCode -ne 200) {
    Write-Host "[FAIL] GET $healthUrl returned HTTP $($resp.StatusCode)." -ForegroundColor Red
    $failures++
  } elseif ($body -ne "ok") {
    $snippet = if ($null -eq $body -or $body -eq "") { "(empty)" } elseif ($body.Length -gt 120) { $body.Substring(0, 120) + "..." } else { $body }
    Write-Host "[FAIL] GET $healthUrl body is not 'ok' (got: $snippet)." -ForegroundColor Red
    $failures++
  } else {
    Write-Host "[ OK ] GET $healthUrl -> 200 ok" -ForegroundColor Green
  }
} catch {
  Write-Host "[FAIL] GET $healthUrl failed: $_" -ForegroundColor Red
  $failures++
}

# 4) Firewall (optional, admin)
if (Test-IsAdministrator) {
  try {
    $rules = Get-NetFirewallRule -DisplayName $FirewallRuleName -ErrorAction SilentlyContinue
    if (-not $rules) {
      Write-Host "[WARN] No firewall rule named '$FirewallRuleName' (install may have used another name)." -ForegroundColor Yellow
      $warn += "Firewall rule not found."
    } else {
      $enabled = $rules | Where-Object { $_.Enabled -eq $true }
      if (-not $enabled) {
        Write-Host "[WARN] Firewall rule '$FirewallRuleName' exists but is disabled." -ForegroundColor Yellow
        $warn += "Firewall rule disabled."
      } else {
        Write-Host "[ OK ] Firewall rule '$FirewallRuleName' present and enabled." -ForegroundColor Green
      }
    }
  } catch {
    Write-Host "[WARN] Could not read firewall rules: $_" -ForegroundColor Yellow
    $warn += "Firewall check failed."
  }
} else {
  Write-Host "[SKIP] Firewall rule check (run as Administrator to verify '$FirewallRuleName')." -ForegroundColor DarkGray
}

Write-Host ""
if ($warn.Count -gt 0) {
  foreach ($w in $warn) {
    Write-Host "Note: $w" -ForegroundColor Yellow
  }
  Write-Host ""
}

if ($failures -gt 0) {
  Write-Host "Result: FAILED ($failures hard check(s))." -ForegroundColor Red
  exit 1
}

Write-Host "Result: PASSED (LAN clients still need correct network profile / firewall profile)." -ForegroundColor Green
exit 0
