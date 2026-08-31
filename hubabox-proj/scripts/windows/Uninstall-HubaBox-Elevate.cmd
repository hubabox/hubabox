@echo off
setlocal
REM UAC prompt, then runs uninstall-service.ps1 as admin.

cd /d "%~dp0"

if not exist "%~dp0uninstall-service.ps1" (
  echo ERROR: uninstall-service.ps1 is missing.
  echo Extract the entire release zip before running this uninstaller.
  echo.
  pause
  exit /b 2
)

powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "$script = Join-Path (Get-Location).Path 'uninstall-service.ps1'; $q = [char]34; $argLine = '-NoProfile -ExecutionPolicy Bypass -File ' + $q + $script + $q; try { $p = Start-Process -FilePath ($env:SystemRoot + '\\System32\\WindowsPowerShell\\v1.0\\powershell.exe') -Verb RunAs -WorkingDirectory (Get-Location).Path -ArgumentList $argLine -Wait -PassThru -ErrorAction Stop; exit $p.ExitCode } catch { Write-Host ('Could not start the elevated uninstaller: ' + $_.Exception.Message) -ForegroundColor Red; exit 1 }"
set "ERR=%ERRORLEVEL%"

echo.
if %ERR% neq 0 (
  echo HubaBox uninstall failed with exit code %ERR%.
) else (
  echo HubaBox uninstall completed successfully.
)
pause
exit /b %ERR%
