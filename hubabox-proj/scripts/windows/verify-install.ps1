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

.PARAMETER UseHttps
  Use HTTPS for the default health URL. The certificate must be trusted by this PC.

.PARAMETER BaseUrl
  Full base URL for health checks. Overrides -UseHttps.
#>
param(
  [string]$ServiceName = "HubaBox",
  [int]$ListenPort = 8787,
  [string]$FirewallRuleName = "HubaBox HTTP",
  [switch]$UseHttps,
  [string]$BaseUrl = ""
)

$ErrorActionPreference = "Stop"

function Test-IsAdministrator {
  $p = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
  return $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

if (-not $BaseUrl) {
  $scheme = if ($UseHttps) { "https" } else { "http" }
  $BaseUrl = "${scheme}://127.0.0.1:$ListenPort"
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

# 2) TCP port (listening on a LAN-capable address)
try {
  $listeners = @(Get-NetTCPConnection -LocalPort $ListenPort -State Listen -ErrorAction SilentlyContinue)
  if (-not $listeners) {
    Write-Host "[FAIL] Nothing listening on TCP port $ListenPort." -ForegroundColor Red
    $failures++
  } else {
    Write-Host "[ OK ] TCP port $ListenPort is in Listen state." -ForegroundColor Green
    $lanListeners = @($listeners | Where-Object { $_.LocalAddress -notin @("127.0.0.1", "::1") })
    if ($lanListeners.Count -eq 0) {
      Write-Host "[FAIL] Port $ListenPort is bound to loopback only; other LAN devices cannot connect." -ForegroundColor Red
      $failures++
    } else {
      $boundAddresses = @($lanListeners | Select-Object -ExpandProperty LocalAddress -Unique)
      Write-Host "[ OK ] LAN-capable listener address: $($boundAddresses -join ', ')" -ForegroundColor Green
    }
  }
} catch {
  Write-Host "[WARN] Could not query TCP listeners: $_" -ForegroundColor Yellow
  $warn += "TCP check skipped or inconclusive."
}

# 3) /health
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
      Write-Host "[FAIL] No firewall rule named '$FirewallRuleName'." -ForegroundColor Red
      $failures++
    } else {
      $enabled = @($rules | Where-Object { $_.Enabled -eq $true -and $_.Direction -eq "Inbound" -and $_.Action -eq "Allow" })
      if (-not $enabled) {
        Write-Host "[FAIL] Firewall rule '$FirewallRuleName' is not an enabled inbound Allow rule." -ForegroundColor Red
        $failures++
      } else {
        $profileCoverage = @($enabled | Where-Object {
          $profileText = $_.Profile.ToString()
          $profileText -eq "Any" -or $profileText -match "Public"
        })
        if ($profileCoverage.Count -eq 0) {
          Write-Host "[FAIL] Firewall rule '$FirewallRuleName' does not cover Public networks." -ForegroundColor Red
          $failures++
        } else {
          Write-Host "[ OK ] Firewall rule covers every active Windows network profile." -ForegroundColor Green
        }

        $addressFilters = @($enabled | Get-NetFirewallAddressFilter -ErrorAction Stop)
        $localSubnetOnly = @($addressFilters | Where-Object {
          $remote = @($_.RemoteAddress)
          $nonLocal = @($remote | Where-Object { $_ -notmatch '^LocalSubnet(4|6)?$' })
          $remote.Count -gt 0 -and $nonLocal.Count -eq 0
        })
        if ($localSubnetOnly.Count -eq 0) {
          Write-Host "[FAIL] Firewall rule is not restricted to RemoteAddress=LocalSubnet." -ForegroundColor Red
          $failures++
        } else {
          Write-Host "[ OK ] Firewall access is restricted to the local subnet." -ForegroundColor Green
        }

        $portFilters = @($enabled | Get-NetFirewallPortFilter -ErrorAction Stop)
        $matchingPort = @($portFilters | Where-Object {
          ($_.Protocol.ToString() -eq "TCP" -or $_.Protocol -eq 6) -and $_.LocalPort.ToString() -eq $ListenPort.ToString()
        })
        if ($matchingPort.Count -eq 0) {
          Write-Host "[FAIL] Firewall rule does not allow TCP port $ListenPort." -ForegroundColor Red
          $failures++
        } else {
          Write-Host "[ OK ] Firewall rule allows TCP port $ListenPort." -ForegroundColor Green
        }
      }
    }
  } catch {
    Write-Host "[FAIL] Could not fully validate firewall rules: $_" -ForegroundColor Red
    $failures++
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

Write-Host "Result: PASSED (service is listening for LAN clients with LocalSubnet-only firewall access)." -ForegroundColor Green
exit 0
