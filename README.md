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

Then open **`http://127.0.0.1:8787`** (default port). Complete first-time setup, then use **`/files`**. Optionally enable **public library** and share the invite link or **`/library`**. On **`/files`**, multi-select and drag-and-drop both send **one** `multipart/form-data` request with all chosen files (field name **`files`**). **Browser uploads** are capped at **100 MiB** per file; **USB / import-folder** copies allow up to **8 GiB** per file (streamed from disk, not loaded whole into RAM).

**USB / import folder:** on **`/files`**, set the watch path in the admin UI (stored in SQLite). The page lists **top-level** names in that folder: tick the ones you want and use **Import selected** (best for large USB sticks). Optional **Automatically copy every new file** watches the folder and copies everything without prompting (off by default). **Import entire folder now** still runs a full copy. The watch path must not be the same as or nested inside `files/`. For automation or packaged installs, **`HUBABOX_IMPORT`** or **`hubabox -import …`** can set a path that **overrides** the UI value while that process is running.

Environment variables (optional):

- `HUBABOX_LISTEN` — bind address (default `:8787`)
- `HUBABOX_DATA` — data directory (SQLite + `files/`)
- `HUBABOX_MDNS` — `1`/`0` to toggle mDNS announcement
- `HUBABOX_MDNS_NAME` — mDNS **service instance** name (default **`HubaBox`**). This is what appears in Bonjour / `_http._tcp` discovery — **not** the same as your PC’s OS hostname. See **mDNS and hostnames** below.
- `HUBABOX_IMPORT` — optional ops override: absolute path to a folder to watch (e.g. USB mount). Same as flag **`-import`**; when set, it wins over the path saved in the UI.
- `HUBABOX_PUBLIC_ORIGIN` — optional base URL for **library invite links**, e.g. `http://192.168.0.7:8787` (no path). Use when you always open admin at `localhost` but guests must use a LAN URL. Same as flag **`-public-origin`**.

Equivalent **flags** (for scripts or systemd): **`-listen`**, **`-data`**, **`-mdns`**, **`-mdns-name`**, **`-import`**, **`-public-origin`**.

## Operations

### Graceful shutdown

**SIGINT** / **SIGTERM** (Ctrl+C in a terminal, or **Stop** on the Windows **HubaBox** service) cancels the process context and runs **`http.Server.Shutdown`** with a **25s** drain so in-flight requests can finish before the listener closes.

### Database backup

The database file is **`hubabox.db`** inside your **data directory** (`-data` / `HUBABOX_DATA`).

From **`hubabox-proj/`**, use **`scripts/backup-sqlite.sh`**: it runs **`sqlite3 … .backup`** when the `sqlite3` CLI is available (consistent snapshot while hubaBox is still running), otherwise **`cp`** (if you use the copy path, stopping the hub first is safest for a crash-consistent file).

```bash
chmod +x scripts/backup-sqlite.sh   # once
./scripts/backup-sqlite.sh /path/to/your/hubabox-data
# optional second argument: output directory (default: DATA_DIR/backups)
```

On **Windows** without bash: stop the service, copy **`hubabox.db`** from **`%ProgramData%\HubaBox`**, or install the SQLite CLI and run  
`sqlite3 "%ProgramData%\HubaBox\hubabox.db" ".backup 'C:\backups\hubabox-snapshot.db'"` while the service is running.

### mDNS and hostnames (library links on the LAN)

hubaBox does **not** rename your computer. Two different names show up in docs and on **`/files`**:

| Name | What it is | In the browser URL |
|------|------------|---------------------|
| **mDNS instance** (default `HubaBox`, overridable with **`HUBABOX_MDNS_NAME`**) | The human-readable label in **Bonjour / `_http._tcp` discovery** lists | **Usually not.** `http://HubaBox.local/…` often **does not resolve** — the instance name is not the same as the DNS host the stack uses for the service record. |
| **OS hostname** (e.g. `pop-os` from **`hostname`**) | The machine name the **zeroconf** library ties to the service (per upstream `Register` behavior) | **Yes — prefer this.** Try **`http://pop-os.local:8787/library`** (replace `pop-os` with your **`hostname`**). Plain **`http://pop-os:8787/…`** may work on some LANs without `.local`. |

**Practical order:** (1) **`http://<hostname>.local:<port>/library`** (2) **`http://<hostname>:<port>/library`** (3) **`http://<LAN-IP>:<port>/library`**. Changing **`HUBABOX_MDNS_NAME`** changes how the service **appears in discovery UIs**; it does **not** by itself make **`http://ThatName.local`** work in the browser.

The running hub’s **`/files`** admin page lists **auto-detected private IPv4 addresses** with ready **login / files / library** links, your **`hostname`** for `.local` hints, and the mDNS instance name. **Copy invite link** uses **`HUBABOX_PUBLIC_ORIGIN`** when set; otherwise a non-**localhost** **Host** if you opened `/files` by LAN IP; otherwise the first detected LAN IP; otherwise **`http://<hostname>.local:<port>`** (e.g. `pop-os.local`) so guests are not given **`localhost`** when avoidable.

Turn mDNS off with **`HUBABOX_MDNS=0`** or **`-mdns=false`** if you do not want LAN broadcast (you will rely on IP or DNS only).

### LAN security (v1 sketch)

- **Roles:** one **admin** (password + session cookie); **library guests** (shared code + cookie). No tenant isolation beyond that.
- **Transport:** default is **HTTP on the LAN** (no TLS); the LAN is the trust boundary. Bind to **`127.0.0.1`** if the UI must not be reachable from other machines.
- **Brute force:** **`POST /login`**, **`POST /setup`**, **`GET /library/join`**, and **`POST /library/unlock`** are **rate-limited per client IP** (sliding window); excess attempts return **429** with **`Retry-After: 60`**.
- **Cookies:** admin and library cookies are **HttpOnly** and **SameSite=Lax** (see `internal/server/middleware.go`).

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
