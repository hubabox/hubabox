HubaBox — Windows pilot bundle
================================

This folder contains everything needed on a Windows PC (no Git clone).

IMPORTANT — do not double-click the .ps1 files
-----------------------------------------------
Windows often opens .ps1 scripts in Notepad instead of running them.
Use the .cmd files below (double-click), or open PowerShell as Administrator and run .\install-service.ps1

Install (pick one)
------------------
  A) Double-click:  Install-HubaBox-Elevate.cmd
     (You get a UAC prompt; then the service installs.)

  B) Right-click:   Install-HubaBox.cmd  →  "Run as administrator"

  C) PowerShell (Administrator):
       cd "C:\path\to\this\folder"
       Set-ExecutionPolicy -Scope Process Bypass -Force
       .\install-service.ps1

hubabox.exe is the program only — it does NOT install the Windows service.
Use the steps above for a proper install (service + firewall rule).

If you already double-clicked hubabox.exe to try it, close that window before installing —
otherwise port 8787 (or your chosen port) stays busy and the service cannot start.

Verify (optional)
-----------------
  Double-click:  Verify-HubaBox.cmd
  Or:            .\verify-install.ps1   in PowerShell

Uninstall
---------
  Double-click:  Uninstall-HubaBox-Elevate.cmd
  Or PowerShell as Administrator:  .\uninstall-service.ps1

After install, open a browser: http://127.0.0.1:8787/  (or your LAN IP from another device).

HTTPS / voice recording (optional)
----------------------------------
Browser microphone recording from another device needs HTTPS. Give the installer a trusted PEM certificate and its key; it copies them into C:\ProgramData\HubaBox\tls with access limited to SYSTEM and Administrators:

    .\install-service.ps1 -TlsCertPath "C:\certs\hubabox-cert.pem" -TlsKeyPath "C:\certs\hubabox-key.pem"

Then open https://<this-pc-ip>:8787/ and verify with:
    .\verify-install.ps1 -UseHttps

The certificate must include the hostname/IP guests use and be trusted by their devices. See the project README for mkcert guidance.

Troubleshooting
---------------
If the service will not install or start, check the log file — every startup
error (port busy, data folder problem, database error) is written there:
    C:\ProgramData\HubaBox\hubabox.log   (fallback: %TEMP%\hubabox.log)
You can also run the exe manually in a terminal to see the error live:
    .\hubabox.exe -listen :8787 -data "C:\ProgramData\HubaBox"
(Double-clicking hubabox.exe also writes to hubabox.log next to its data folder,
%AppData%\hubabox — so a flash-close console window still leaves a trace.)

Optional settings: copy hubabox-config.example.txt to e.g. my-hub.conf, edit KEY=value lines, then from elevated PowerShell:
    .\install-service.ps1 -HubConfigFile .\my-hub.conf

For all switches, see the comment block at the top of install-service.ps1 or the project README.
