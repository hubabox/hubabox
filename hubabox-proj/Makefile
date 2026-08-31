# Run from hubabox-proj/ (Go project root).
# VERSION for .deb (semver without leading "v", e.g. 0.1.0). Release CI sets this from the git tag.
VERSION ?= 0.0.0
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
LDFLAGS := -s -w -X main.buildVersion=$(VERSION) -X main.buildCommit=$(COMMIT)

.PHONY: test build dist dist-linux dist-linux-amd64 dist-linux-bundle dist-linux-deb dist-windows dist-windows-amd64 dist-windows-bundle clean

test:
	go test ./...

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o bin/hubabox ./cmd/hubabox

dist: dist-linux-amd64 dist-windows-amd64

dist-linux-amd64:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/hubabox-linux-amd64 ./cmd/hubabox

# Tarball: binary + systemd unit + install script + README (same story as Windows zip).
dist-linux-bundle: dist-linux-amd64
	rm -rf dist/linux-bundle-staging
	mkdir -p dist/linux-bundle-staging
	cp dist/hubabox-linux-amd64 dist/linux-bundle-staging/hubabox
	cp scripts/linux/hubabox.service scripts/linux/README-LINUX.txt scripts/linux/install-systemd.sh dist/linux-bundle-staging/
	chmod +x dist/linux-bundle-staging/install-systemd.sh dist/linux-bundle-staging/hubabox
	cd dist/linux-bundle-staging && tar czvf ../hubabox-linux-amd64-bundle.tar.gz .
	rm -rf dist/linux-bundle-staging
	@echo "Wrote dist/hubabox-linux-amd64-bundle.tar.gz"

# Debian/Ubuntu package: binary + systemd unit under /lib/systemd/system (install: sudo apt install ./hubabox_*_amd64.deb).
dist-linux-deb: dist-linux-amd64
	@command -v dpkg-deb >/dev/null 2>&1 || { echo "dpkg-deb required (e.g. sudo apt install dpkg-dev)"; exit 1; }
	rm -rf dist/deb-staging
	staging=dist/deb-staging/hubabox_$(VERSION)_amd64; \
	install -d "$$staging/DEBIAN" "$$staging/usr/local/bin" "$$staging/lib/systemd/system"; \
	install -m755 dist/hubabox-linux-amd64 "$$staging/usr/local/bin/hubabox"; \
	install -m644 scripts/linux/hubabox.service "$$staging/lib/systemd/system/hubabox.service"; \
	install -m755 scripts/linux/deb/DEBIAN/postinst "$$staging/DEBIAN/postinst"; \
	install -m755 scripts/linux/deb/DEBIAN/prerm "$$staging/DEBIAN/prerm"; \
	install -m755 scripts/linux/deb/DEBIAN/postrm "$$staging/DEBIAN/postrm"; \
	kb=$$(du -sk "$$staging/usr" "$$staging/lib" 2>/dev/null | awk '{t+=$$1} END {printf "%d", t+0}'); \
	sed -e "s/@VERSION@/$(VERSION)/" -e "s/@INSTALLED_KB@/$$kb/" scripts/linux/deb/DEBIAN/control.in > "$$staging/DEBIAN/control"; \
	( cd dist/deb-staging && dpkg-deb --root-owner-group --build "hubabox_$(VERSION)_amd64" ); \
	mv "dist/deb-staging/hubabox_$(VERSION)_amd64.deb" "dist/hubabox_$(VERSION)_amd64.deb"; \
	rm -rf dist/deb-staging; \
	echo "Wrote dist/hubabox_$(VERSION)_amd64.deb"

dist-linux: dist-linux-amd64 dist-linux-bundle dist-linux-deb

dist-windows-amd64:
	mkdir -p dist
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/hubabox-windows-amd64.exe ./cmd/hubabox

# Single zip for pilot PCs: hubabox.exe + install / uninstall / verify scripts + README-WINDOWS.txt
dist-windows-bundle: dist-windows-amd64
	@command -v zip >/dev/null 2>&1 || { echo "zip is required (e.g. sudo apt install zip)"; exit 1; }
	rm -rf dist/windows-bundle-staging
	mkdir -p dist/windows-bundle-staging
	cp dist/hubabox-windows-amd64.exe dist/windows-bundle-staging/hubabox.exe
	cp scripts/windows/*.ps1 scripts/windows/*.cmd scripts/windows/README-WINDOWS.txt scripts/windows/hubabox-config.example.txt dist/windows-bundle-staging/
	cd dist/windows-bundle-staging && zip -r -q ../hubabox-windows-amd64-bundle.zip .
	rm -rf dist/windows-bundle-staging
	@echo "Wrote dist/hubabox-windows-amd64-bundle.zip"

dist-windows: dist-windows-amd64 dist-windows-bundle

clean:
	rm -rf bin dist
