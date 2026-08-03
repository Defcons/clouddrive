# Research Journal — CloudDrive

<!--
  Append-only chronological log: how we learned what we know. The HISTORY.
  Never rewrite old entries. Promote durable facts to KnowledgeBase.md.
  Navigation → CodeMap.md · model → KnowledgeBase.md. Cross-link, don't copy.
-->

_Last verified: 2026-08-03 @ 14b9eb5 — by Claude (Opus 4.8), triad seed._

## What this is / mission
CloudDrive is a self-hosted, single-binary personal cloud / file explorer for the homelab
(Go backend embedding a React PWA, JSON-file storage, Docker + Syncthing deploy). This journal
records how understanding and hardening of it accumulate over time. It is not a tuning rig, so
entries are milestones/investigations rather than controlled single-variable experiments.

## Protocol / rules of the rig
- Verification gates before any commit (from `IMPROVEMENTS.md` / `FEATURES.md`):
  - Backend: `cd backend && GOOS=linux go build ./... && GOOS=linux go vet ./... && go test ./...`
  - Frontend: `cd frontend && npm run build`
  - `GOOS=linux` is mandatory — the app uses Linux-only syscalls, so a plain Windows build fails by design.
- **Never push `master` casually** — it auto-deploys to the live server. Do work on a branch.

## Current state (read this first)
- **Mature, feature-complete for v1.** The `master` history shows: an initial build (file explorer +
  Syncthing sync), a security **hardening loop** (`loop/hardening` → `IMPROVEMENTS.md`), then a
  **9-feature enhancement pass** (`feat/enhancements` → `FEATURES.md`, all 9 done), plus deploy
  fixes and an internal-IP redaction. Head is `14b9eb5` (trusted-device MFA skip extended 30→90 days).
- **Tests exist and pass** under `GOOS=linux` across handlers/middleware/services (added during the
  hardening loop, where none existed at the 2026-06-25 baseline).
- **Open threads:** the documented follow-ups — collaborate-upload quota, WebDAV/MFA, path-keyed
  version history, client-side resumable-upload persistence (see KnowledgeBase.md "Confirmed issues — OPEN").
- **Next action:** none pending; this entry only seeds the triad. Future work should append below.

## Iteration ledger
| iter | change (ONE variable) | hypothesis | result | verdict |
|---|---|---|---|---|
| — | 2026-08-03: seeded the documentation triad (CodeMap / KnowledgeBase / ResearchJournal). No code touched. | A fresh session can orient in seconds instead of re-reading `main.go` + 116 source files. | Triad created at repo root, anchored to real symbols; verified `kind:"session"` invariant, path-confinement helpers, reserved dotdirs, atomic JSON writes, env fail-fast, 24h/90d token lifetimes against code. | KEEP |

<!--
  Prior history (pre-triad) is preserved in its original docs, not re-litigated here:
  - Security hardening iterations (authz gaps, CRITICAL MFA-bypass, trash traversal/clobber, etc.)
    → IMPROVEMENTS.md ("Hardening Log").
  - The 9 enhancement features (thumbnails, Range, chunked upload, admin UI, WebDAV, quotas,
    versioning, session mgmt, content search) → FEATURES.md.
  Future findings append to the ledger above and promote durable facts to KnowledgeBase.md.
-->

## Open questions / backlog
- Confirm how `.env` is provisioned on the live host (the deploy falls back to copying `.env.example`).
- Decide whether the committed-but-gitignored `frontend/dist/` artifacts should be removed from the working tree (served copy is built in Docker).
- The four documented feature follow-ups (KnowledgeBase.md "Confirmed issues — OPEN") remain unscheduled.
