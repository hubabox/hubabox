@echo off
setlocal
REM Collect logs and Windows service/network details without including hub data.

cd /d "%~dp0"
title HubaBox - collect diagnostics

if not exist "%~dp0collect-diagnostics.ps1" (
  echo ERROR: collect-diagnostics.ps1 is missing.
  echo Extract the entire release zip before running this tool.
  echo.
  pause
  exit /b 2
)

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0collect-diagnostics.ps1"
set "ERR=%ERRORLEVEL%"
echo.
pause
exit /b %ERR%
