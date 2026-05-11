# Run from hubabox-proj/ (Go project root).
.PHONY: test build dist dist-windows dist-windows-amd64 dist-windows-bundle clean

test:
	go test ./...

build:
	go build -trimpath -ldflags="-s -w" -o bin/hubabox ./cmd/hubabox

dist: dist-linux-amd64 dist-windows-amd64

dist-linux-amd64:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/hubabox-linux-amd64 ./cmd/hubabox

dist-windows-amd64:
	mkdir -p dist
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/hubabox-windows-amd64.exe ./cmd/hubabox

# Single zip for pilot PCs: hubabox.exe + install / uninstall / verify scripts + README-WINDOWS.txt
dist-windows-bundle: dist-windows-amd64
	@command -v zip >/dev/null 2>&1 || { echo "zip is required (e.g. sudo apt install zip)"; exit 1; }
	rm -rf dist/windows-bundle-staging
	mkdir -p dist/windows-bundle-staging
	cp dist/hubabox-windows-amd64.exe dist/windows-bundle-staging/hubabox.exe
	cp scripts/windows/*.ps1 scripts/windows/README-WINDOWS.txt scripts/windows/hubabox-config.example.txt dist/windows-bundle-staging/
	cd dist/windows-bundle-staging && zip -r -q ../hubabox-windows-amd64-bundle.zip .
	rm -rf dist/windows-bundle-staging
	@echo "Wrote dist/hubabox-windows-amd64-bundle.zip"

dist-windows: dist-windows-amd64 dist-windows-bundle

clean:
	rm -rf bin dist
