# Run from hubabox-proj/ (Go project root).
.PHONY: test build dist dist-windows clean

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

clean:
	rm -rf bin dist
