@echo off
setlocal
REM Do not double-click .ps1 files - Windows often opens them in Notepad.
REM This file runs the real installer in PowerShell.

cd /d "%~dp0"
title HubaBox - install service

echo.
echo Installing HubaBox as a Windows service...
echo If you see "Run as Administrator", close this window, then RIGHT-CLICK this file
echo and choose "Run as administrator", OR use Install-HubaBox-Elevate.cmd for a UAC prompt.
echo.

if not exist "%~dp0install-service.ps1" (
  echo ERROR: install-service.ps1 is missing. Extract the entire release zip first.
  pause
  exit /b 2
)
if not exist "%~dp0hubabox.exe" (
  echo ERROR: hubabox.exe is missing. Extract the entire release zip first.
  pause
  exit /b 2
)

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0install-service.ps1"
set ERR=%ERRORLEVEL%

echo.
if %ERR% neq 0 (
  echo Install failed (exit %ERR%). Read the messages above.
  echo Installer transcript: C:\ProgramData\HubaBox\install.log
) else (
  echo Done.
)
pause
exit /b %ERR%
