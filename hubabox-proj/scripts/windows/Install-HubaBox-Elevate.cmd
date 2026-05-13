@echo off
setlocal
REM UAC prompt, then runs install-service.ps1 as admin (same folder as this file).

cd /d "%~dp0"

powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath ($env:SystemRoot + '\\System32\\WindowsPowerShell\\v1.0\\powershell.exe') -Verb RunAs -WorkingDirectory (Get-Location).Path -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File',(Join-Path (Get-Location).Path 'install-service.ps1')"

exit /b 0
