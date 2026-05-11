# Audit log — pre-ship concerns (implement later)

This file captures **design and operational risks** for an audit/event log feature **before** it is treated as production-ready. Use it as a checklist when implementing `audit_events` (or similar) and again before a labeled release.

---

## 1. Privacy vs usefulness

- **What you log** (IP, user-agent, filenames, library joins, admin actions) can reveal sensitive usage patterns.
- **Decide explicitly**: data categories, who may view `/admin/audit`, and whether guests’ activity is identifiable enough for your policy.
- **Pre-ship**: document one paragraph of “what we store and why” for operators; consider **minimum necessary** fields for v1.

---

## 2. Volume and SQLite

- High **download** volume can create many rows quickly.
- **Pre-ship**: define **retention** (e.g. last N days, or max row count) and/or archival export then purge.
- **Pre-ship**: add **indexes** suited to common queries (e.g. `(created_at DESC)`, `(event_type, created_at)`).
- Monitor **DB file growth** on low-disk hubs.

---

## 3. Attribution limits

- Today the model is roughly: **admin session** vs **library guest** (cookie) vs unauthenticated endpoints.
- Library downloads may only be attributable to **IP + user-agent** unless you add stronger identity later.
- **Pre-ship**: document “who this event refers to” per event type; avoid implying non-repudiation you do not have.

---

## 4. Correctness when logging fails

- If the DB is locked, full, or errors on insert: choose **strict** (fail the request so action and log stay consistent) vs **best-effort** (complete user action, drop or queue log).
- **Pre-ship**: pick one policy per event class (e.g. strict for admin security events, best-effort for high-volume downloads if needed) and test both paths.

---

## 5. Tampering and trust

- Append-only SQLite is **not** proof against someone with **filesystem** or **admin** access.
- **Pre-ship**: state honestly that logs are for **operations and support**, not legal-grade tamper evidence, unless you add **external log shipping**, signing, or WORM storage later.

---

## 6. Security of the audit UI

- `/admin/audit` must stay **admin-only** (same gate as `/files`).
- **Pre-ship**: no leakage via CSV/export without auth; rate-limit or paginate to avoid scrape-style load.

---

## 7. Legal / regional (if you distribute widely)

- Some jurisdictions care about **logging minors**, **schools**, or **workplace monitoring**.
- **Pre-ship**: if you target schools/enterprises, get a one-page **operator guidance** (consent, signage, retention) from counsel when you are close to ship — this doc is technical, not legal advice.

---

## Suggested implementation order (when you build it)

1. Schema + append-only inserts for a **small set** of event types.  
2. Admin-only list view with **pagination** and simple **filters**.  
3. Retention job or documented manual purge.  
4. Optional **CSV export** for backups.  
5. Revisit strict vs best-effort logging after real pilot load.

---

## Traceability

These items came from the “challenges” discussion around adding an audit log to **hubaBox** (`hubabox-proj/`). Implementation details belong in `implementation-plan.md` once work is scheduled; this file stays **concerns and pre-ship gates** only.
