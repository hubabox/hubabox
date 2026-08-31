@echo off
setlocal
REM Smoke-check install (no admin required for basic checks; firewall line skipped if not admin).

cd /d "%~dp0"
title HubaBox - verify

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0verify-install.ps1"
set "ERR=%ERRORLEVEL%"

if %ERR% neq 0 (
  echo.
  echo Verification failed. Collecting diagnostics...
  if exist "%~dp0collect-diagnostics.ps1" (
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0collect-diagnostics.ps1"
  ) else (
    echo collect-diagnostics.ps1 is missing; extract the entire release zip.
  )
)
echo.
pause
exit /b %ERR%
