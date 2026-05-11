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

Then open **`http://127.0.0.1:8787`** (default port). Complete first-time setup, then use **`/files`**. Optionally enable **public library** and share the invite link or **`/library`**. Guests enter the access code (full hex token or **last 6 characters**), pick a **display name**, then can browse downloads, **text chat**, and **voice notes** (recorded clips attached to a message — not live audio). On **`/files`**, multi-select and drag-and-drop both send **one** `multipart/form-data` request with all chosen files (field name **`files`**). **Browser uploads** are capped at **100 MiB** per file; **USB / import-folder** copies allow up to **8 GiB** per file (streamed from disk, not loaded whole into RAM). Voice notes are stored under **`library_chat_audio/`** inside the hub data directory (small per-file cap; see code).

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

The running hub’s **`/files`** admin page lists **auto-detected private IPv4 addresses** with ready **login / files / library** links, your **`hostname`** for `.local` hints, and the mDNS instance name. **Copy invite link** uses **`HUBABOX_PUBLIC_ORIGIN`** when set; otherwise a non-**localhost** **Host** if you opened `/files` by LAN IP; otherwise the first detected LAN IP; otherwise **`http://<hostname>.local:<port>`** (e.g. `pop-os.local`) so guests are not given **`localhost`** when avoidable. Opening admin at **`http://localhost:…`** does **not** make the invite use localhost: the server ignores the **`Host: localhost`** name for invite URLs so the same LAN IP list as on the card can drive the link.

Turn mDNS off with **`HUBABOX_MDNS=0`** or **`-mdns=false`** if you do not want LAN broadcast (you will rely on IP or DNS only).

### Public library: access, chat, and voice notes

- **Who counts as a guest:** anyone who knows the **library access code** (full 64-character hex from the admin UI, or the **last 6 hex characters** of that token for manual entry) gets a **library cookie** (~30 days). That is the trust boundary for read-only files, chat, and voice playback URLs.
- **Display name:** on first entry (unlock form or, after an invite link, the **“pick display name”** step), guests choose a **1–32 character** name (letters, digits, spaces, and a small set of punctuation). It is stored in a separate **HttpOnly** cookie and shown next to chat messages.
- **Chat:** recent messages are stored in SQLite and listed on **`/library`**; the list **auto-refreshes every few seconds** (HTMX) without reloading the whole page. There is **no live voice chat** — only **voice notes**: short **WebM / Ogg / WAV** clips. **In-browser recording** only works in a **secure context** (**`https://`** or **`http://localhost` / `http://127.0.0.1`**). On typical **`http://<LAN-IP>`** hubs, Chrome and Firefox **block the microphone** by policy — guests can still use **Attach audio file** to upload a clip. Posts and name changes are **rate-limited per IP** like other library endpoints.
- **Data:** chat text lives in the database; audio blobs live under **`{HUBABOX_DATA}/library_chat_audio/`** with random hex filenames (not mixed into the main **`files/`** share tree).

### LAN security (v1 sketch)

- **Roles:** one **admin** (password + session cookie); **library guests** (shared code + cookie). No tenant isolation beyond that.
- **Transport:** default is **HTTP on the LAN** (no TLS); the LAN is the trust boundary. Bind to **`127.0.0.1`** if the UI must not be reachable from other machines.
- **Brute force:** **`POST /login`**, **`POST /setup`**, **`GET /library/join`**, **`POST /library/unlock`**, **`POST /library/set-name`**, and **`POST /library/chat/post`** are **rate-limited per client IP** (sliding window); excess attempts return **429** with **`Retry-After: 60`**.
- **Cookies:** admin, library token, and **library display name** cookies are **HttpOnly** and **SameSite=Lax** (see `internal/server/middleware.go`).

## Testing

```bash
cd hubabox-proj
go test ./...
```

## Release binaries

From **`hubabox-proj/`**:

```bash
make dist              # linux + windows amd64 → dist/
# Windows exe only:
make dist-windows-amd64
# Windows one-file handoff (zip: hubabox.exe + PowerShell scripts + README-WINDOWS.txt); needs `zip` on the build machine:
make dist-windows-bundle
# or both Windows artifacts:
make dist-windows
```

The bundle **`dist/hubabox-windows-amd64-bundle.zip`** is what you typically send to a friend: extract, then follow **`README-WINDOWS.txt`** (same steps as below).

## Windows: service + firewall

**Who needs the git repo?** Only people building or changing hubaBox. **Friends and pilot PCs do not clone anything** — easiest: send **`dist/hubabox-windows-amd64-bundle.zip`** (from `make dist-windows-bundle` on a dev machine); they extract once and get **`hubabox.exe`**, the three **`*.ps1`** scripts, **`README-WINDOWS.txt`**, and **`hubabox-config.example.txt`**. Alternatively, zip that same set yourself from **`hubabox-proj/scripts/windows/`** plus a renamed `hubabox-windows-amd64.exe` → **`hubabox.exe`**. Everything must live in the **same folder** on the Windows PC before running the commands below.

If you are developing from a clone, the same scripts live under **`hubabox-proj/scripts/windows/`**; paths in examples use that layout for convenience.

Copy **`hubabox.exe`** next to **`install-service.ps1`** on the target PC, then run PowerShell **as Administrator**:

```powershell
Set-ExecutionPolicy -Scope Process Bypass -Force
.\install-service.ps1
```

Defaults: service **`HubaBox`**, data under **`%ProgramData%\HubaBox`**, inbound TCP **`8787`** on Private/Domain firewall profiles.  

Useful install options:

```powershell
# Custom port + custom data dir
.\install-service.ps1 -ListenPort 8788 -DataDir "D:\HubaBoxData"

# Bind only loopback (hub not reachable from LAN; use with care)
.\install-service.ps1 -Listen "127.0.0.1:8787"

# mDNS off; custom mDNS name; library invite base; USB import path (same flags as the hub binary)
.\install-service.ps1 -MdnsOff -MdnsName "FriendsHub" -PublicOrigin "http://192.168.0.7:8787" -ImportDir "E:\USBShare"

# Optional KEY=value file (see hubabox-config.example.txt). Explicit parameters override the file.
.\install-service.ps1 -HubConfigFile .\my-hub.conf

# If the PC is often on guest/public Wi-Fi and LAN clients still need access
.\install-service.ps1 -IncludePublicProfile
```

Admin password and first-run setup are still done in the **browser** (`/setup`, then `/login`) after the service is running — the installer only wires the Windows service and firewall.

The installer script is idempotent: if **`HubaBox`** already exists, it is stopped, removed, and recreated cleanly; startup is verified before exit. It also configures service recovery to auto-restart after crashes.

After install (or when debugging), run **`scripts/windows/verify-install.ps1`** from PowerShell. It checks that the **`HubaBox`** service is **Running**, something is **listening** on the hub port, **`GET /health`** returns **`ok`**, and (if you run **as Administrator**) that the **`HubaBox HTTP`** firewall rule exists and is enabled. Example:

```powershell
cd path\to\the\folder\with\hubabox.exe\and\scripts
.\verify-install.ps1
.\verify-install.ps1 -ListenPort 8788
```

Remove with **`scripts/windows/uninstall-service.ps1`**.

Cross-compile from Linux/macOS: `GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o hubabox.exe ./cmd/hubabox` (after `cd hubabox-proj`).

## License

Not specified in this README — add a `LICENSE` file when you pick one.
