# hubaBox

**Hub in a box** — one machine on your LAN acts as a small file hub with a browser UI: admin uploads, guests can open a **read-only library** (invite link + access code). Works offline once the hub is running; no cloud SaaS runtime.

This repository mixes **planning docs** at the repo root with the **Go implementation** under `hubabox-proj/`.

## Repository layout

| Path | Contents |
|------|----------|
| `implementation-plan.md` | Phased build order and product notes |
| `sureBox.md` | Early product/thought memo for **hubaBox** (vision and context) |
| **`hubabox-proj/`** | **Go module** — clone this path as `$PROJECT_ROOT` for builds |

## Requirements

- Go **1.22+** (`cd hubabox-proj`)

## Quick start (development)

```bash
cd hubabox-proj
go run ./cmd/hubabox
```

Then open **`http://127.0.0.1:8787`** (default port). Complete first-time setup, then use **`/files`**. Optionally enable **public library** and share the invite link or **`/library`**. On **`/files`**, multi-select and drag-and-drop both send **one** `multipart/form-data` request with all chosen files (field name **`files`**); per-file size limits still apply on the server.

**USB / import folder:** on **`/files`**, set the watch path in the admin UI (stored in SQLite). The page lists **top-level** names in that folder: tick the ones you want and use **Import selected** (best for large USB sticks). Optional **Automatically copy every new file** watches the folder and copies everything without prompting (off by default). **Import entire folder now** still runs a full copy. The watch path must not be the same as or nested inside `files/`. For automation or packaged installs, **`HUBABOX_IMPORT`** or **`hubabox -import …`** can set a path that **overrides** the UI value while that process is running.

Environment variables (optional):

- `HUBABOX_LISTEN` — bind address (default `:8787`)
- `HUBABOX_DATA` — data directory (SQLite + `files/`)
- `HUBABOX_MDNS` — `1`/`0` to toggle mDNS announcement
- `HUBABOX_MDNS_NAME` — mDNS instance name
- `HUBABOX_IMPORT` — optional ops override: absolute path to a folder to watch (e.g. USB mount). Same as flag **`-import`**; when set, it wins over the path saved in the UI.

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
