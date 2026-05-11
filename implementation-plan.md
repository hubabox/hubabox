# hubaBox — implementation plan

This document turns the product intent (**hub in a box**: one PC as a LAN-first digital hub, browser as client) into a **sequenced build order**. Later steps assume earlier ones are stable.

**Principles (carry through all phases)**

- One **narrow MVP wedge** first; resist feature sprawl until the hub is boringly reliable.
- **Server + browser UI** only; no Electron / heavy SPA for the product shell.
- **Go** backend, **SQLite** first, **HTMX + Tailwind** (Alpine.js only if needed).
- Ship **Windows** and **Linux** from one codebase when practical; **Windows installer + service** is the main adoption path for many pilots.
- **Reliability and small footprint** over feature count; **zero-friction LAN access** is non-negotiable for v1.

## Repository layout

- **Workspace root** (the folder that contains this file and `sureBox.md`): product notes only. It is **not** the Go module root; do not run `go build` here.
- **`hubabox-proj/`**: **project root** for hubaBox — `go.mod`, `cmd/`, `internal/`, and future CI scripts should treat this directory as `$PROJECT_ROOT` (`cd hubabox-proj && go build ./...`). Embedded HTML/CSS lives under **`internal/server/web/`** (required for `go:embed` paths relative to the `server` package).

---

## Phase 0 — Foundations (before implementation)

| Order | Task | Outcome |
| ----- | ---- | ------- |
| 0.1 | **Lock MVP wedge** (pick one primary job): e.g. (A) shared files + folders on LAN, or (B) read-only “library” + admin import (USB / upload). | Written one-page scope: in-scope / out-of-scope for v1. |
| 0.2 | **Define v1 user story**: install → open browser → accomplish X without internet. | Acceptance criteria you can demo. |
| 0.3 | **Repo + module layout**: treat **`hubabox-proj/`** as project root; Go module path, versioning, CI stub (`cd hubabox-proj && go test ./...`). | Repeatable builds. |
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
| 1.4 | **HTMX + Tailwind** wired; login / setup wizard if multi-user; otherwise first-run password. | 1.2, 1.3 |
| 1.5 | **Admin file hub** (foundation): first-run admin password, session cookie, flat `files/` storage under data dir, list / upload / download / delete (admin only). | 1.4 |
| 1.5b | **Library layer** (next): read-only browsing for guests, same storage, policy + routes. | 1.5 |
| 1.6 | **mDNS / Zeroconf**: advertise `hubabox.local` (or chosen name); fallback to showing IP in UI. | 1.5 |
| 1.7 | Hardening pass: graceful shutdown, DB backup hook, basic rate limits on auth routes. | 1.6 |

**Gate:** Two clients on same LAN can complete the v1 user story without typing raw IP if mDNS works in environment (with IP fallback always available).

---

## Phase 2 — Windows distribution (adoption path)

| Order | Task | Depends on |
| ----- | ---- | ---------- |
| 2.1 | **Windows service** integration (start/stop, recovery); logs location documented. | Phase 1 gate |
| 2.2 | **Installer** (NSIS / WiX / Inno): copy binary, install service, optional “open firewall for port N.” | 2.1 |
| 2.3 | First-run or installer step: set listen address, admin password, data directory. | 2.2 |
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
| 4.3 | Automated tests for auth, file API, mDNS registration (where testable in CI). | 1.7 |
| 4.4 | Upgrade path: migrate DB schema; preserve data dir across installs. | 2.2, 3.2 |

**Gate:** You can simulate power-kill / kill -9 and document what is lost vs preserved (honest ops story).

---

## Phase 5 — Windows integrations (after v1 wedge is stable)

Do **not** start these until Phases 1–4 are acceptable in pilot; each item is its own mini-project.

| Order | Task | Notes |
| ----- | ---- | ----- |
| 5.1 | **USB import** folder watcher or “import on insert” flow for admins. | High value for offline content; scope permissions carefully. |
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
5.x SMB / USB / printers (as needed)
      ↓
6.x search / media / caching / integrations
```

---

## Suggested “first ship” definition

**Minimum shippable hubaBox:** Phases **0–3** complete for **one** wedge, plus **4.1** and **4.4** at a basic level. Everything in Phase 5–6 is explicitly **post–first ship** unless your wedge cannot exist without it (rare).

---

## Traceability

Product and stack assumptions align with `sureBox.md` next to this file in the workspace root (CommunityBox placeholder): **browser-only client**, **Go + SQLite + HTMX**, **Windows + Linux**, **mDNS**, **defer** heavy caching / SMB / printers until the core hub is proven.

See **Repository layout** above: implementation lives under `hubabox-proj/`, not in the workspace root.

When this plan diverges from reality (pilot feedback), update the **MVP wedge** in Phase 0 and re-sequence 5–6 only; keep 1→4 order stable.
