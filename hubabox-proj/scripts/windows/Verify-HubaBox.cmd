@echo off
setlocal
REM Smoke-check install (no admin required for basic checks; firewall line skipped if not admin).

cd /d "%~dp0"
title HubaBox — verify

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0verify-install.ps1"
echo.
pause
exit /b %ERRORLEVEL%
