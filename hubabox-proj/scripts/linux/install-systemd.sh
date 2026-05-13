#!/bin/sh
# Install hubabox binary + systemd unit. Run from the extracted bundle directory.
set -e
ROOT=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
PREFIX=${PREFIX:-/usr/local}

if [ ! -f "$ROOT/hubabox" ] || [ ! -f "$ROOT/hubabox.service" ]; then
  echo "Run this script from the extracted bundle (need ./hubabox and ./hubabox.service)." >&2
  exit 1
fi

echo "Installing to $PREFIX/bin/hubabox and /etc/systemd/system/hubabox.service ..."
sudo install -m 0755 "$ROOT/hubabox" "$PREFIX/bin/hubabox"
sudo mkdir -p /var/lib/hubabox
sudo install -m 0644 "$ROOT/hubabox.service" /etc/systemd/system/hubabox.service
sudo systemctl daemon-reload
sudo systemctl enable hubabox.service
sudo systemctl restart hubabox.service
echo "Done. systemctl status hubabox   |   journalctl -u hubabox -f"
