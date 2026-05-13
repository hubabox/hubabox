@echo off
setlocal
REM UAC prompt, then runs uninstall-service.ps1 as admin.

cd /d "%~dp0"

powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath ($env:SystemRoot + '\\System32\\WindowsPowerShell\\v1.0\\powershell.exe') -Verb RunAs -WorkingDirectory (Get-Location).Path -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File',(Join-Path (Get-Location).Path 'uninstall-service.ps1')"

exit /b 0
