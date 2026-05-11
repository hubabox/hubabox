# hubaBox — implementation plan

This document turns the product intent (**hub in a box**: one PC as a LAN-first digital hub, browser as client) into a **sequenced build order**. Later steps assume earlier ones are stable.

**How this relates to `sureBox.md`**

`sureBox.md` is the **vision memo**: problem space, audiences, long-horizon features (education stacks, caching, SMB, printers, and similar). It uses the product name **hubaBox** and describes a **mature** system—not a commitment for the next milestone.

**This file is canonical for delivery:** phases, gates, what is implemented in `hubabox-proj/`, and what ships next. When sequencing or “what exists today” differs from the vision memo, **update this plan** (not `sureBox.md`) unless you are deliberately changing product direction, in which case update **both** starting with Phase 0 here.

**Principles (carry through all phases)**

- One **narrow MVP wedge** first; resist feature sprawl until the hub is boringly reliable.
- **Server + browser UI** only; no Electron / heavy SPA for the product shell.
- **Go** backend, **SQLite** first. **UI today:** embedded `html/template` + CSS under `internal/server/web/`. **Optional polish:** HTMX + Tailwind (and Alpine only if needed), as recommended in `sureBox.md`—not required to pass Phase 1.
- Ship **Windows** and **Linux** from one codebase when practical; **Windows installer + service** is the main adoption path for many pilots (cybercafés, schools, and offices often on Windows—aligned with `sureBox.md` deployment notes).
- **Reliability and small footprint** over feature count; **zero-friction LAN access** is non-negotiable for v1.

## Repository layout

- **Workspace root** (the folder that contains this file and `sureBox.md`): product notes only. It is **not** the Go module root; do not run `go build` here.
- **`hubabox-proj/`**: **project root** for hubaBox — `go.mod`, `cmd/`, `internal/`, and future CI scripts should treat this directory as `$PROJECT_ROOT` (`cd hubabox-proj && go build ./...`). Embedded HTML/CSS lives under **`internal/server/web/`** (required for `go:embed` paths relative to the `server` package).

---

## Intended feature inventory (reference)

Single checklist of **what hubaBox is meant to do**, from today’s code through long-horizon vision (`sureBox.md`). **Status** is a snapshot for planning—when something ships or slips, update this section with the phase table above.

**Legend:** **Done** = in `hubabox-proj/` today · **Planned** = tied to a phase below, not yet acceptable for the gate · **Vision** = product intent from `sureBox.md`, not scheduled (may never ship as stated).

### Core runtime, config, and UX shell

- **Single Go binary** serving HTTP; embedded templates + static CSS/JS (`go:embed`). (**Done**)
- **Configurable listen address** (flags / `HUBABOX_LISTEN`). (**Done**)
- **`/health` HTTP probe** for operators and load balancers. (**Done**)
- **Structured logging** (request IDs, recoverer). (**Done**)
- **Configurable data directory** (`HUBABOX_DATA` / flags); SQLite + `files/` tree under it. (**Done**)
- **Optional HTMX + Tailwind (+ Alpine)** for lighter, faster UI iteration. (**Vision / optional polish** — Phase 1.4)

### Identity, sessions, and security

- **First-run admin password** (setup flow); **no default password**. (**Done**)
- **Admin session cookie** (single admin; sign-in / sign-out). (**Done**)
- **Public library guest cookie** (~30d) after unlock. (**Done**)
- **LAN-only threat model** documented (who is admin vs guest, bind defaults, TLS stance). (**Partial** — README “LAN security”; fuller Phase 0.4 doc optional)
- **Rate limits** on **`POST /login`**, **`POST /setup`**, **`GET /library/join`**, **`POST /library/unlock`** (per client IP, sliding window; **429** + **`Retry-After`**). (**Done** — Phase 1.8)
- **Graceful HTTP shutdown** on SIGINT / SIGTERM / Windows service stop (`http.Server.Shutdown` with drain). (**Done** — Phase 1.8)

### Admin file hub (`/files` and related routes)

- **List, upload, download, delete** files in hub storage (admin only). (**Done**)
- **Drag-and-drop and multi-select upload** (batch to same endpoint). (**Done**)
- **Nested folders in hub storage** (tree beyond flat `files/`). (**Vision** — wedge today is flat; see Phase 0.1 if promoted)

### File size limits and large media

- **Dual caps (implemented):** **`MaxUploadBytes`** (**100 MiB**) for **browser** `multipart` uploads (untrusted body size per request). **`MaxImportBytes`** (**8 GiB**) for **USB / folder import** (local path → hub `files/`, same machine). Code: `internal/files` (`SaveUpload` vs `ImportRegularFile` / `saveUploadWithLimit`). (**Done**)
- **Import is a streamed copy** to a `.partial` file then rename—not the whole source file loaded into RAM—so large audio/video from removable media does **not** require a chunking protocol on the import path. (**Done**)
- **Operator-tunable limits** (e.g. env or flags for `MaxImportBytes` / `MaxUploadBytes` without recompile). (**Vision** — follow-up if pilots need it)
- **Resumable or chunked HTTP uploads** (e.g. tus, custom chunk assembly) for multi‑GB files through the **browser** when USB is not used. (**Vision** — separate feature from USB import)

### Read-only public library

- **Admin enables/disables** library; **random access token** stored in KV. (**Done**)
- **Guest join** via invite link + **manual access code** (full token or **last 6 hex chars**); read-only listing and download. (**Done**)
- **Invite link base URL** avoids `localhost` when admin uses localhost (LAN IP / `hostname.local` / **`HUBABOX_PUBLIC_ORIGIN`**). (**Done**)
- **Guest display name** cookie after unlock or invite; **text chat** + **voice notes** (uploaded clips, not live audio) in SQLite + `library_chat_audio/`. (**Done**)
- **mDNS + LAN hints** on admin `/files`: auto-detected **private IPv4** list with **login / files / library** links; **OS `hostname`.local** guidance; mDNS instance name (discovery-only). (**Done**)

### USB / folder import (admin)

- **Large media:** prefer this path over browser upload for files above the HTTP cap; see **File size limits and large media**. (**Done**)
- **Watch folder path** in SQLite and/or **ops override** (`-import` / `HUBABOX_IMPORT`). (**Done**)
- **Top-level listing** (cap ~500 names), **selective import**, **import entire folder**, **optional auto-copy** + fsnotify when path set; **idle when no path** (no watch until configured). (**Done** — polish / hardening under Phase 1.8 / 4.x / 5.1 sign-off)
- **“Import on insert” OS hooks** beyond folder watch (e.g. volume arrival). (**Vision** — not specified in code)

### LAN discovery and networking

- **Optional mDNS / Zeroconf** `_http._tcp` (instance name configurable; default `HubaBox`). (**Done**)
- **TLS on LAN** (optional HTTPS, local certs or BYO). (**Vision** — not in phased plan; add Phase if required)

### Windows distribution and service behavior

- **Windows service path** when not interactive (`svc` integration). (**Done** — `main_windows.go`; polish with Phase 2)
- **Install / uninstall / verify PowerShell scripts** (service, firewall, `%ProgramData%\HubaBox`, smoke `verify-install.ps1`). (**Partial** — Phase 2.2; GUI installer still planned)
- **GUI installer** (NSIS / WiX / Inno)—optional after scripted install works. (**Planned** — Phase 2+ optional)
- **Installer configuration** of listen address, data dir, mDNS, public origin, import path (`install-service.ps1` / optional config file). Admin password still **browser `/setup`**. (**Partial** — Phase 2.3)
- **Windows smoke matrix** (VMs, reboot, firewall). (**Planned** — Phase 2.4)

### Linux distribution

- **Documented data dir and flags** on Linux. (**Planned** — Phase 3.1)
- **`.deb` + systemd unit** (postinst, user/group). (**Planned** — Phase 3.2)
- **Optional Docker image** + volume mounts for data. (**Planned** — Phase 3.3)
- **Avahi / mDNS notes** for common distros. (**Planned** — Phase 3.4)

### Reliability, backup, and upgrades

- **SQLite pragmas + backup strategy** (online copy, schedule); corruption recovery doc. (**Planned** — Phase 4.1)
- **Memory/CPU profiling** vs **~200MB RSS** service budget (where realistic). (**Planned** — Phase 4.2)
- **CI-friendly automated tests** (auth, files, library, mDNS where feasible). (**Partial** — GitHub Actions runs `gofmt`, `go vet`, `go test`, native build on Ubuntu + Windows, plus Windows cross-compile from Linux; tag **`v*`** triggers **release** workflow attaching **`hubabox-windows-amd64-bundle.zip`** + Linux binary; deeper per-area tests still planned — Phase 4.3)
- **DB schema migrations**; preserve data dir across upgrades. (**Planned** — Phase 4.4)
- **SQLite backup script + README** (`scripts/backup-sqlite.sh`, prefers `sqlite3 .backup`). (**Done** — Phase 1.8 / 4.1 overlap)
- **In-app or HTTP “backup now” trigger** (scheduled backups). (**Vision** — Phase 4.1+)

### OS integrations (post–core hub)

- **SMB / Windows file sharing** exposure of selected hub paths (`\\server\share`). (**Planned** — Phase 5.2)
- **Printer integration** or queue visibility (if wedge still fits). (**Planned** — Phase 5.3)

### Scale, media, caching, and “smart” layers

- **Full-text or metadata search** (e.g. Bleve) when file counts justify it. (**Planned** — Phase 6.1)
- **ffmpeg thumbnails / transcoding** for video (feature-flagged). (**Planned** — Phase 6.2)
- **Controlled caching** of specific content types (not whole-internet proxy by default). (**Planned** — Phase 6.3)
- **Education / offline content hooks** (Kiwix, Kolibri-style)—only if education wedge is active. (**Planned** — Phase 6.4)
- **Hub-to-hub delta sync**; **local LLM** (e.g. llama.cpp) after core maturity. (**Planned** — Phase 6.5)

### Vision memo capabilities (not yet phased—candidate backlog)

Items called out in `sureBox.md` that **do not** have a dedicated row above; promote into phases when/if the wedge expands:

- **Education:** offline Wikipedia mirror, Khan-style mirrors, exam-prep packs, local quizzes, teacher distribution flows.
- **Business:** invoice templates, LAN chat, richer “local backup” stories beyond SQLite/file copy.
- **Community:** curated local media hubs, announcements, lightweight marketplace patterns, faith-organization media workflows.
- **Network:** shared download / update caching (YouTube, OS updates, package mirrors)—legal and scope per deployment.
- **File layer extras:** deduplication, chunking, resumable sync, “mini-Dropbox for LAN” patterns.
- **WebSocket** or real-time channels if chat / live UI is added.
- **Packaging beyond `.deb`:** AppImage, bootable appliance images, preloaded hardware bundles (business / NGO channels).

---

## Phase 0 — Foundations (before implementation)

| Order | Task | Outcome |
| ----- | ---- | ------- |
| 0.1 | **Lock MVP wedge** (pick one primary job): e.g. (A) shared files + folders on LAN, or (B) read-only “library” + admin import (USB / upload). | Written one-page scope: in-scope / out-of-scope for v1. |
| 0.2 | **Define v1 user story**: install → open browser → accomplish X without internet. | Acceptance criteria you can demo. |
| 0.3 | **Repo + module layout**: treat **`hubabox-proj/`** as project root; Go module path, versioning; **GitHub Actions** CI (`cd hubabox-proj && go test ./...` plus vet/fmt/build matrix). | Repeatable builds. |
| 0.4 | **Threat model sketch** (LAN-only v1): who is admin, who is client, default bind addresses, TLS or not on LAN. | Security defaults documented. |

**Gate:** Do not start Phase 2 Windows packaging until Phase 1 “happy path” works on dev machines.

---

## Phase 1 — Core hub (cross-platform)

Build the smallest **real** hubaBox: HTTP server, persistence, LAN use.

| Order | Task | Depends on |
| ----- | ---- | ---------- |
| 1.1 | Go HTTP server: configurable **host:port**, health route, structured logging. | 0.3 |
| 1.2 | **SQLite**: schema for settings + users/sessions (or single-admin + tokens, depending on wedge). | 1.1 |
| 1.3 | **Embedded** static assets + HTML templates (`go:embed`); base layout (nav shell matching future “Files / …” areas). | 1.1 |
| 1.4 | **Auth shell**: embedded templates + CSS; first-run admin password + session cookie (single admin). HTMX/Tailwind optional later. | 1.2, 1.3 |
| 1.5 | **Admin file hub** (foundation): first-run admin password, session cookie, flat `files/` storage under data dir, list / upload / download / delete (admin only). | 1.4 |
| 1.6 | **mDNS / Zeroconf**: `_http._tcp` via `github.com/grandcat/zeroconf` (instance name `-mdns-name` / `HUBABOX_MDNS_NAME`, default `HubaBox`). Disable with `-mdns=false` or `HUBABOX_MDNS=0`. Admin `/files` shows LAN hints. | 1.5 |
| 1.7 | **Public library (read-only)**: admin enables on `/files` (KV token); guests use `/library` + access code (cookie ~30d); same files as admin, download-only. | 1.5 |
| 1.8 | Hardening pass: graceful shutdown, DB backup hook, basic rate limits on auth routes. | 1.7 |

**Gate:** Two clients on same LAN can complete the v1 user story without typing raw IP if mDNS works in environment (with IP fallback always available).

---

## Phase 2 — Windows distribution (adoption path)

| Order | Task | Depends on |
| ----- | ---- | ---------- |
| 2.1 | **Windows service** (`main_windows.go`): service name **`HubaBox`**, uses `svc.IsWindowsService()` + `golang.org/x/sys/windows/svc`. Interactive `hubabox.exe` still uses Ctrl+C like Linux. Installer / `sc create` wiring is next (2.2). | Phase 1 gate |
| 2.2 | **Windows install automation (first pass)**: `scripts/windows/install-service.ps1` + `uninstall-service.ps1` (elevated PowerShell: service via `New-Service`, firewall rule, `-data` under `%ProgramData%\HubaBox`); **`verify-install.ps1`** smoke-checks service, listen port, `GET /health`, and (when elevated) firewall rule. Release binaries: `make dist-windows-amd64`; pilot zip **`make dist-windows-bundle`** → `dist/hubabox-windows-amd64-bundle.zip` (exe + scripts + `README-WINDOWS.txt`). Full NSIS/WiX GUI installer remains optional later. | 2.1 |
| 2.3 | **Installer / config file (partial)**: `install-service.ps1` accepts `-Listen`, `-HubConfigFile` (KEY=value), `-MdnsOff`/`-MdnsOn`, `-MdnsName`, `-PublicOrigin`, `-ImportDir` (wired into service binary path); admin password remains **browser `/setup`**. GUI installer / full first-run wizard still optional. | 2.2 |
| 2.4 | Smoke test matrix: Win 10/11, clean VM, reboot persistence, firewall on. | 2.3 |

**Gate:** Non-developer can install and reach UI from another machine on same Wi‑Fi using hostname or IP.

---

## Phase 3 — Linux distribution

| Order | Task | Depends on |
| ----- | ---- | ---------- |
| 3.1 | Same binary behavior on Linux; document **data dir** and config path. | Phase 1 gate |
| 3.2 | **`.deb`** package: systemd unit, postinst for user/group, README fragment. | 3.1 |
| 3.3 | Optional **Dockerfile** (same binary); document volume mounts for data. | 3.1 |
| 3.4 | mDNS notes (**Avahi** install / permissions) for common distros. | 3.2 |

**Gate:** Install on Debian/Ubuntu VM matches Windows feature parity for v1 wedge.

---

## Phase 4 — Reliability & operations

| Order | Task | Depends on |
| ----- | ---- | ---------- |
| 4.1 | **SQLite** pragmas / backup strategy (online copy, schedule); corruption recovery doc. | 1.2 |
| 4.2 | Memory and CPU profiling; budget against **&lt; ~200MB** service RSS goal where realistic. | 1.1 |
| 4.3 | **CI baseline**: `.github/workflows/ci.yml` (`gofmt`, `go vet`, `go test`, build on Ubuntu + Windows; Windows cross-compile on Linux). Expand coverage: auth, file API, library, mDNS where feasible. | 1.8 |
| 4.4 | Upgrade path: migrate DB schema; preserve data dir across installs. | 2.2, 3.2 |

**Gate:** You can simulate power-kill / kill -9 and document what is lost vs preserved (honest ops story).

---

## Phase 5 — Windows integrations (after v1 wedge is stable)

Defer **new** large integrations until Phases **1–4** are acceptable in pilot. Items below include work **already started early** (USB); treat “done” as pilot sign-off + hardening, not greenfield build.

| Order | Task | Notes |
| ----- | ---- | ----- |
| 5.1 | **USB / folder import (admin)** | **Largely implemented in Phase 1** (`/files`: watch path in SQLite or `-import` / `HUBABOX_IMPORT`, listing + selective import, optional auto-copy + fsnotify, idle when no path). Remaining: formal hardening, UX polish, edge cases—close under Phase 4 / 1.8 as appropriate. |
| 5.2 | **SMB** exposure of selected shares (`\\server\share`). | Familiarity for users; non-trivial ACL + security review. |
| 5.3 | **Printer** integration / queue visibility (if still aligned with wedge). | OS-specific; validate with real printers. |

---

## Phase 6 — “Smart” and scale features (later)

| Order | Task | Notes |
| ----- | ---- | ----- |
| 6.1 | **Search** (e.g. Bleve) over indexed metadata when file counts justify it. | Avoid premature indexing cost. |
| 6.2 | **ffmpeg** pipeline for thumbnails / transcoding when video is in scope. | Resource-heavy; feature-flag. |
| 6.3 | **Caching** of *specific* content types you control (not “whole internet” day one). | Legal + technical design per source. |
| 6.4 | Integrations: **Kiwix** / **Kolibri**-style hooks only if education wedge is active. | Packaging + disk images. |
| 6.5 | **Delta sync** / hub-to-hub exchange; **local LLM** (e.g. llama.cpp) only after core is mature. | Optional long-term differentiators. |

---

## Dependency graph (high level)

```text
0.x scope & repo
      ↓
1.x core server + SQLite + UI + MVP wedge + mDNS
      ↓
   ┌──┴──┐
   ↓     ↓
2.x Win   3.x Linux
service   .deb/systemd
+ MSI     + Docker?
   └──┬──┘
      ↓
4.x reliability & upgrades
      ↓
5.x SMB / printers (+ USB import polish as needed)
      ↓
6.x search / media / caching / integrations
```

---

## Suggested “first ship” definition

**Minimum shippable hubaBox:** Phases **0–3** complete for **one** wedge, plus **4.1** and **4.4** at a basic level. Everything in Phase 5–6 is explicitly **post–first ship** unless your wedge cannot exist without it (rare).

---

## Traceability

`sureBox.md` (vision memo; product name **hubaBox**) and **this plan** both assume: **browser-only client**, **Go + SQLite**, **Windows + Linux**, **mDNS for LAN discovery**, and **deferring** heavy caching, SMB, and printers until the **core hub** is proven. Stack detail in the memo (e.g. HTMX/Tailwind) is **aspirational UI polish** unless listed as done in Phase 1+ rows above.

See **Repository layout**: code lives under `hubabox-proj/`, not the workspace root.

**Audit log (later):** pre-ship risks and checklist live in `audit-log-pre-ship.md` at the repo root; reconcile that file before treating audit logging as production-ready.

When reality diverges from this document (pilot feedback, scope pull-forward like USB), **edit this file first**; adjust Phase 0 wedge text and Phase 5–6 notes as needed. Keep **1→4** order stable unless there is a strong reason to replatform.
