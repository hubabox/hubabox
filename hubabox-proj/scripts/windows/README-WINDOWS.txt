HubaBox — Windows pilot bundle
================================

This folder contains everything needed on a Windows PC (no Git clone).

1. Copy this entire folder somewhere on the machine (e.g. Desktop\HubaBox).
2. Open PowerShell as Administrator, then:
     cd "C:\path\to\this\folder"
     Set-ExecutionPolicy -Scope Process Bypass -Force
     .\install-service.ps1
3. Optional: .\verify-install.ps1
4. Open a browser on this PC: http://127.0.0.1:8787/  (or use your LAN IP from another device).

Remove the service later: .\uninstall-service.ps1 (also as Administrator).

Optional settings: copy hubabox-config.example.txt to e.g. my-hub.conf, edit KEY=value lines, then:
    .\install-service.ps1 -HubConfigFile .\my-hub.conf

For all switches (listen address, data dir, mDNS, public invite URL, import folder, firewall profiles),
see the comment block at the top of install-service.ps1 or the project README.
