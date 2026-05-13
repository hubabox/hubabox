HubaBox — Linux bundle (amd64)
==============================

Prefer the GitHub release .deb on Debian/Ubuntu
-------------------------------------------------
  Download hubabox_<version>_amd64.deb from the same release page, then:
    sudo apt install ./hubabox_0.1.0_amd64.deb
  (replace 0.1.0 with the release version; no leading "v".)
  Remove: sudo apt remove hubabox
  (Data in /var/lib/hubabox is kept unless you delete it manually.)

Contents (this tarball)
--------
  hubabox              — binary (install to /usr/local/bin)
  hubabox.service      — systemd unit (listens :8787, data /var/lib/hubabox)
  install-systemd.sh   — copies files, enables and starts the service (needs sudo)

Quick install (Debian / Ubuntu style)
---------------------------------------
  tar xzf hubabox-linux-amd64-bundle.tar.gz
  cd <extracted folder>
  sudo ./install-systemd.sh

Then open http://127.0.0.1:8787/ and complete /setup in the browser.

Firewall (if ufw is on)
------------------------
  sudo ufw allow 8787/tcp comment 'HubaBox'
  sudo ufw reload

Logs
----
  journalctl -u hubabox -f

Stop / uninstall
----------------
  sudo systemctl stop hubabox
  sudo systemctl disable hubabox
  sudo rm /etc/systemd/system/hubabox.service
  sudo systemctl daemon-reload
  sudo rm -f /usr/local/bin/hubabox
  (optional) sudo rm -rf /var/lib/hubabox
