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
- `HUBABOX_ALLOW_REMOTE_SETUP` — set to `1` only when you deliberately need to create the first admin account from another LAN machine. By default, first-time setup is available only on the hub itself (`127.0.0.1`), preventing a nearby client from claiming an unconfigured hub.
- `HUBABOX_TRUST_PROXY` — set to `1` only behind a trusted reverse proxy. Otherwise forwarded-IP headers are ignored, so clients cannot spoof the IP used by rate limits.
- `HUBABOX_TLS_CERT` and `HUBABOX_TLS_KEY` — PEM certificate and matching private-key paths. Set **both** to serve HTTPS, which enables in-browser microphone recording from LAN addresses. Equivalent flags: `-tls-cert` and `-tls-key`.

Equivalent **flags** (for scripts or systemd): **`-listen`**, **`-data`**, **`-mdns`**, **`-mdns-name`**, **`-import`**, **`-public-origin`**, **`-allow-remote-setup`**, **`-trust-proxy`**.

### Voice notes on a LAN

Text chat and uploaded audio clips work over the default HTTP LAN setup. Browser microphone recording is intentionally restricted by Chrome, Firefox, and Safari to a **secure context**: use `https://` with a certificate trusted by guest devices (or access the hub on `localhost`). To enable HTTPS, start hubaBox with both certificate options:

```bash
hubabox -tls-cert /path/to/hub-cert.pem -tls-key /path/to/hub-key.pem
```

Use a certificate that guests trust—typically from your organization’s local CA or a trusted reverse proxy. A browser certificate warning must be accepted before it will expose microphone permission. When HTTPS is enabled, hubaBox’s generated LAN and library invite links use `https://` automatically.

For a small home/pilot LAN, [`mkcert`](https://github.com/FiloSottile/mkcert) is usually the least painful option: it creates a local CA and a certificate with the required hostname/IP SAN entries.

```bash
# Install mkcert by your OS package method, then create and trust its local CA.
mkcert -install

# Use the exact hostname and LAN IP guests will open.
mkcert -key-file hubabox-key.pem -cert-file hubabox-cert.pem hubabox.local 192.168.1.20

hubabox -tls-cert hubabox-cert.pem -tls-key hubabox-key.pem
```

Install the mkcert local CA on each guest phone or computer that will use recording. A self-signed OpenSSL certificate can start HTTPS, but it still has to be manually trusted on every guest device and must include the hostname/IP in its Subject Alternative Names; for this use case, mkcert or an organization-managed CA is generally simpler.

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
- **First run:** initial setup is **local-only by default**, even when the hub listens on the LAN. Complete it at `http://127.0.0.1:8787` on the hub machine; use `-allow-remote-setup` only for a controlled installation.
- **Browser protection:** all state-changing browser requests use CSRF tokens; response headers block framing, MIME sniffing, and cross-origin referrers. The invite token is still a bearer secret—share its link only with intended guests.
- **Brute force:** **`POST /login`**, **`POST /setup`**, **`GET /library/join`**, **`POST /library/unlock`**, **`POST /library/set-name`**, and **`POST /library/chat/post`** are **rate-limited per client IP** (sliding window); excess attempts return **429** with **`Retry-After: 60`**.
- **Cookies:** admin, library token, and **library display name** cookies are **HttpOnly** and **SameSite=Lax** (see `internal/server/middleware.go`).

## Testing

```bash
cd hubabox-proj
go test ./...
go vet ./...
```

### CI (GitHub Actions)

On **push** and **pull request** to **`main`** / **`master`**, **`.github/workflows/ci.yml`** runs **`gofmt`**, **`go vet`**, **`go test`**, and a **native `go build`** on **Ubuntu** and **Windows**. Ubuntu also **cross-compiles** `GOOS=windows GOARCH=amd64` to catch Windows-only compile issues from Linux dev machines. **CI does not upload release zips** (keeps PR runs fast).

If **`gofmt`** fails on the **Windows** job with a long list of files, it is usually **CRLF vs LF**: the repo root **`.gitattributes`** forces **`*.go`** / **`go.mod`** / **`go.sum`** to **LF** so `gofmt` matches Linux. After pulling that change, run once if needed: **`git add --renormalize .`** then commit. Locally always run **`gofmt -w .`** from **`hubabox-proj/`** before pushing.

### Release zip (Windows bundle on GitHub)

When you push a **version tag** matching **`v*`** (e.g. **`git tag v0.1.0 && git push origin v0.1.0`**), **`.github/workflows/release.yml`** attaches:

- **`hubabox-windows-amd64-bundle.zip`** — same idea as the Windows pilot zip (`make dist-windows-bundle`).
- **`hubabox-linux-amd64-bundle.tar.gz`** — Linux “folder handoff”: binary + systemd unit + install script + README (`make dist-linux-bundle`).
- **`hubabox_<version>_amd64.deb`** — install on Debian/Ubuntu with **`sudo apt install ./hubabox_*_amd64.deb`** (`make dist-linux-deb`; version comes from the tag, e.g. **`v0.1.0`** → **`hubabox_0.1.0_amd64.deb`**).
- **`hubabox-linux-amd64`** — bare binary only (useful for scripting or non-Debian distros).

Older releases may list **only** the bare Linux binary if the tag pointed at a commit **before** these `Makefile` / workflow updates. **GitHub runs whatever workflow existed on that tag** — use **Actions → Release → Run workflow** on a tag whose commit includes the current workflow, or tag a newer commit (e.g. **`v0.1.1`**) to get all assets.

**Why “nothing happened” or “release is empty”**

1. **GitHub only runs the workflows that exist on the tagged commit.** If you created **`v0.1.0`** on an old commit (before **`.github/workflows/release.yml`** landed on **`main`**), that tag never ran this job. Pushing the **same tag again** does not re-fire the event — Git reports **everything up to date**.
2. **Fix:** Point the tag at a commit that contains the workflow (usually current **`main`**), then push the tag again:
   ```bash
   git checkout main && git pull
   git tag -d v0.1.0                           # delete locally
   git push origin :refs/tags/v0.1.0           # delete on GitHub (optional if you need to reuse the name)
   git tag v0.1.0
   git push origin v0.1.0
   ```
   Or use a **new** tag (e.g. **`v0.1.1`**) on **`main`**.
3. **Check** the **Actions** tab → workflow **“Release”** for errors (failed **`make`**, missing **`zip`**, or permissions).
4. **Manual retry (after this repo’s workflow update):** **Actions → Release → Run workflow** and enter the existing tag (e.g. **`v0.1.0`**). That checks out that tag and re-uploads assets (only works if that commit has **`Makefile`** targets **`dist-linux-amd64`**, **`dist-linux-bundle`**, **`dist-linux-deb`**, and **`dist-windows-bundle`**).

## Release binaries

From **`hubabox-proj/`**:

```bash
make dist              # linux + windows amd64 → dist/
# Linux tarball (binary + systemd unit + install script + README-LINUX.txt):
make dist-linux-bundle
# Linux .deb for Debian/Ubuntu (set VERSION to match your tag without the leading v):
make dist-linux-deb VERSION=0.1.0
# or all Linux artifacts (binary + tarball + .deb):
make dist-linux VERSION=0.1.0
# Windows exe only:
make dist-windows-amd64
# Windows one-file handoff (zip: hubabox.exe + .cmd launchers + .ps1 + README); needs `zip` on the build machine:
make dist-windows-bundle
# or both Windows artifacts:
make dist-windows
```

The bundle **`dist/hubabox-windows-amd64-bundle.zip`** is what you typically send to a Windows friend: extract, then follow **`README-WINDOWS.txt`** (same steps as below). For Linux, prefer the **`.deb`** on GitHub releases for Ubuntu/Debian, or **`dist/hubabox-linux-amd64-bundle.tar.gz`** for a scriptable tarball — see **Linux: systemd** below.

### Windows pilot smoke checklist

Use this on a **clean or real pilot PC** after extracting the Windows zip (or from a dev clone with the same scripts next to **`hubabox.exe`**). Goal: repeatability before you ask others to install.

1. **Close** any interactive **`hubabox.exe`** you opened for testing (it holds the HTTP port; the installer will warn if the port is busy).
2. **Install:** double-click **`Install-HubaBox-Elevate.cmd`** and approve UAC (or elevated PowerShell per **`README-WINDOWS.txt`**).
3. **Verify:** in PowerShell, **`.\verify-install.ps1`** — expect **Running** service, listener on the hub port, **`GET /health` → `ok`**, and (when run as Administrator) firewall rule present.
4. **Browser:** open **`http://127.0.0.1:8787/`** (or your chosen port) → **`/setup`**, set admin password, confirm **`/files`** loads.
5. **LAN / phone:** same Wi‑Fi, **`http://<this-PC-LAN-IP>:8787/health`** → **`ok`** (adjust port if you customized it).
6. **Reboot (optional but recommended):** sign out or reboot, wait for login, re-run **`verify-install.ps1`** and spot-check **`/health`** again.

## Linux: systemd

**Who needs the git repo?** Same as Windows: pilots can use **GitHub release assets** only.

### Ubuntu / Debian (recommended): `.deb`

Download **`hubabox_<version>_amd64.deb`** from the release, then:

```bash
sudo apt install ./hubabox_0.1.0_amd64.deb
```

(`apt` resolves local `./` packages; **`dpkg -i`** works too, then **`sudo apt -f install`** if dependencies complain.) The package installs **`/usr/local/bin/hubabox`**, **`/lib/systemd/system/hubabox.service`**, creates **`/var/lib/hubabox`**, enables and starts **`hubabox`**. Open **`http://127.0.0.1:8787/`** and complete **`/setup`**.

Remove: **`sudo apt remove hubabox`** (stops the service; your data in **`/var/lib/hubabox`** is left on disk unless you delete it yourself).

### Tarball (any Linux with systemd + sudo)

**`hubabox-linux-amd64-bundle.tar.gz`** contains **`hubabox`**, **`hubabox.service`** (defaults: listen **`:8787`**, data **`/var/lib/hubabox`**), **`install-systemd.sh`**, and **`README-LINUX.txt`**.

```bash
tar xzf hubabox-linux-amd64-bundle.tar.gz
cd extracted-folder
sudo ./install-systemd.sh   # installs to /usr/local/bin, enables + starts unit
```

Then open **`http://127.0.0.1:8787/`** and complete **`/setup`** in the browser. Logs: **`journalctl -u hubabox -f`**. If **`ufw`** is enabled, allow **`8787/tcp`** (see **`README-LINUX.txt`** in the bundle).

Developers: the same files live under **`hubabox-proj/scripts/linux/`** if you prefer to copy from a clone.

## Windows: service + firewall

**Who needs the git repo?** Only people building or changing hubaBox. **Friends and pilot PCs do not clone anything** — easiest: send **`dist/hubabox-windows-amd64-bundle.zip`**; they extract and double-click **`Install-HubaBox-Elevate.cmd`** (UAC prompt) — **not** the `.ps1` files (Windows often opens those in Notepad). The zip includes **`hubabox.exe`**, **`*.cmd`**, **`*.ps1`**, **`README-WINDOWS.txt`**, and **`hubabox-config.example.txt`**. **`hubabox.exe` alone does not install the service**; use the `.cmd` installer or elevated PowerShell per **`README-WINDOWS.txt`**.

If you are developing from a clone, the same scripts live under **`hubabox-proj/scripts/windows/`**; paths in examples use that layout for convenience.

**Developers:** from an elevated PowerShell in that folder, **`.\install-service.ps1`** is fine. **Friends:** use **`Install-HubaBox-Elevate.cmd`** or **`README-WINDOWS.txt`**.

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

# HTTPS for LAN voice recording (certificate/key are copied into ProgramData with service-only access)
.\install-service.ps1 -TlsCertPath "C:\certs\hubabox-cert.pem" -TlsKeyPath "C:\certs\hubabox-key.pem"

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
# HTTPS install: certificate must be trusted on this PC
.\verify-install.ps1 -UseHttps
```

Remove with **`scripts/windows/uninstall-service.ps1`**.

**When something fails:** on Windows the hub always writes a log file — **`hubabox.log`** inside the data directory (**`%ProgramData%\HubaBox`** for the service, **`%AppData%\hubabox`** for a double-clicked exe; fallback **`%TEMP%`** if the data dir cannot be created yet). Startup errors (busy port, data dir, database) land there even though a service has no console. A service that exits on startup now also reports a **service-specific exit code** to Windows (visible in `services.msc` / the install script output) instead of dying silently.

Cross-compile from Linux/macOS: `GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o hubabox.exe ./cmd/hubabox` (after `cd hubabox-proj`).

## License

Not specified in this README — add a `LICENSE` file when you pick one.
