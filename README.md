# hubaBox

**Hub in a box** — one machine on your LAN acts as a small file hub with a browser UI: admin uploads, guests can open a **read-only library** (invite link + access code). Works offline once the hub is running; no cloud SaaS runtime.

This repository mixes **planning docs** at the repo root with the **Go implementation** under `hubabox-proj/`.

## Repository layout

| Path | Contents |
|------|----------|
| `implementation-plan.md` | Phased build order and product notes |
| `sureBox.md` | Early product/thought memo (placeholder names like CommunityBox appear there historically) |
| **`hubabox-proj/`** | **Go module** — clone this path as `$PROJECT_ROOT` for builds |

## Requirements

- Go **1.22+** (`cd hubabox-proj`)

## Quick start (development)

```bash
cd hubabox-proj
go run ./cmd/hubabox
```

Then open **`http://127.0.0.1:8787`** (default port). Complete first-time setup, then use **`/files`**. Optionally enable **public library** and share the invite link or **`/library`**.

Environment variables (optional):

- `HUBABOX_LISTEN` — bind address (default `:8787`)
- `HUBABOX_DATA` — data directory (SQLite + `files/`)
- `HUBABOX_MDNS` — `1`/`0` to toggle mDNS announcement
- `HUBABOX_MDNS_NAME` — mDNS instance name

## Testing

```bash
cd hubabox-proj
go test ./...
```

## Release binaries

From **`hubabox-proj/`**:

```bash
make dist              # linux + windows amd64 → dist/
# or only Windows:
make dist-windows-amd64
```

## Windows: service + firewall

Copy **`hubabox.exe`** beside **`hubabox-proj/scripts/windows/install-service.ps1`** on the target PC, then run PowerShell **as Administrator**:

```powershell
Set-ExecutionPolicy -Scope Process Bypass -Force
.\install-service.ps1
```

Defaults: service **`HubaBox`**, data under **`%ProgramData%\HubaBox`**, inbound TCP **`8787`** on Private/Domain firewall profiles.  

Remove with **`scripts/windows/uninstall-service.ps1`**.

Cross-compile from Linux/macOS: `GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o hubabox.exe ./cmd/hubabox` (after `cd hubabox-proj`).

## License

Not specified in this README — add a `LICENSE` file when you pick one.
