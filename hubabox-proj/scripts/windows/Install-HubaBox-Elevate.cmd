@echo off
setlocal
REM UAC prompt, then runs install-service.ps1 as admin (same folder as this file).
REM Keep the quoted script path inside one ArgumentList string: Start-Process
REM otherwise loses quoting when the extracted directory contains spaces.

cd /d "%~dp0"

if not exist "%~dp0install-service.ps1" (
  echo ERROR: install-service.ps1 is missing.
  echo Extract the entire release zip before running this installer.
  echo.
  pause
  exit /b 2
)
if not exist "%~dp0hubabox.exe" (
  echo ERROR: hubabox.exe is missing.
  echo Extract the entire release zip before running this installer.
  echo.
  pause
  exit /b 2
)

echo Requesting Administrator permission...
powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "$script = Join-Path (Get-Location).Path 'install-service.ps1'; $q = [char]34; $argLine = '-NoProfile -ExecutionPolicy Bypass -File ' + $q + $script + $q; try { $p = Start-Process -FilePath ($env:SystemRoot + '\\System32\\WindowsPowerShell\\v1.0\\powershell.exe') -Verb RunAs -WorkingDirectory (Get-Location).Path -ArgumentList $argLine -Wait -PassThru -ErrorAction Stop; exit $p.ExitCode } catch { Write-Host ('Could not start the elevated installer: ' + $_.Exception.Message) -ForegroundColor Red; exit 1 }"
set "ERR=%ERRORLEVEL%"

echo.
if %ERR% neq 0 (
  echo HubaBox installation failed with exit code %ERR%.
  echo See C:\ProgramData\HubaBox\install.log for details.
) else (
  echo HubaBox installation completed successfully.
)
pause
exit /b %ERR%
