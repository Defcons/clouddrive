# Research Journal — CloudDrive

<!--
  Append-only chronological log: how we learned what we know. The HISTORY.
  Never rewrite old entries. Promote durable facts to KnowledgeBase.md.
  Navigation → OrientationMap.md · model → KnowledgeBase.md. Cross-link, don't copy.
-->

_Last verified: 2026-08-19 @ aa0100f — audit round 5 (findings logged; no code changed)._

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
| A5b | 2026-08-20: shipped round 5 to prod, then a **followup branch** (`security/audit-round5-followup`) landing M4 (pin CI action SHAs), L1 (rate-limiter cleanup no longer lifts lockout early), L4 (share-auth cookie stores a derived value, not the plaintext password — `TestSharePasswordCookieIsDerived`), L5 (per-IP throttle on the public directory-zip). | Merge permission granted to Claude via `.claude/settings.local.json` (`Bash(gh pr merge:*)`). | **Deploy incident learned:** the live deploys had been silently failing since Aug 3 — `git fetch` on the server hit `insufficient permission … .git/objects` (root-owned shard). `chown -R deploy:deploy /opt/clouddrive` fixed it; now recorded as a landmine in OrientationMap. | KEEP |
| A5 | 2026-08-19: full in-depth audit **round 5** (after the 4 rounds in IMPROVEMENTS.md). 6 workstreams — auth/authz/MFA/CSRF/sessions driven directly + 5 parallel subagents (path, concurrency, deploy/CI, frontend, DoS), each briefed with the known/accepted list so only NEW issues surface. No code touched. | A mature, 4-rounds-hardened repo still has a residual traversal/DoS tail that a fresh, breadth-first sweep will find. | **Confirmed.** 2 HIGH + 11 MED + 8 LOW, all re-read in source before logging (see `ToDo.md` "Audit round 5 findings"). Headline: **H1 `Compress` output-name path-traversal write** (`files.go:1135`, no `filepath.Base` — cross-tenant + escapes STORAGE_ROOT, as root uid) and **H2 thumbnail decode-bomb OOM** (`thumbnail.go:108`, `image.Decode` before any dimension guard). Recurring theme: resource caps present on the single-POST upload but absent on Extract / chunked-assembly / thumbnail-decode / JSON bodies / server timeouts. Deploy tail: floating action tags holding the deploy key, Syncthing GUI on `0.0.0.0`, root container, `.gitignore` gaps. | KEEP — fixes pending |

### Round 5 — confirmed CLEAN (traced, no action)
- **Auth core solid:** `kind:"session"` gate, `pwv` check, `jti` revocation, MFA challenge flow
  (kind-checked, TOTP/backup accepted, short-lived token), trusted-device cookie (sub+pwv bound),
  CSRF (crypto/rand + constant-time + method scope + session-keyed + cleanup). No handler ever
  serializes the password hash / MFA secret / backup codes — admin listing uses secret-free
  `AdminUserInfo`; `Check` uses context values only.
- **Authz chokepoint sound:** `userCanAccess` (`files.go:202`) composes `pathWithinHome` (home
  confinement) AND `permStore.CanAccess`, so `CanAccess`'s default-allow is safely backstopped.
- **`safePath` traversal/symlink correct** (both `files.go` & `share.go`); the EvalSymlinks-on-
  nonexistent TOCTOU is unreachable — no API lets a non-admin create a symlink (zip symlink entries
  are written as plain files). A malicious symlink synced in via Syncthing is caught by EvalSymlinks.
- **Frontend clean:** no `dangerouslySetInnerHTML`/`innerHTML`/`eval`; every server string renders as
  escaped JSX; CSRF token stays in module memory + `X-CSRF-Token` header only; no token in
  storage/URLs; service worker never caches `/api/` or `/share/`. (One LOW: PDF-preview iframe keyed on
  extension — already neutralized by the global `X-Content-Type-Options: nosniff` in `SecureHeaders`.)
- **JSON-store concurrency clean:** `saveJSONFile` temp+rename atomic; live `save()` paths marshal
  under lock; tag/permission/session accessors return copies. (Only latent: the *dead* `UserStore.save()`
  — L6.) Deploy verified NOT fork/PR-triggerable; no workflow script-injection; no secrets baked into
  the image; `JWT_SECRET` fail-fast + `CLOUDDRIVE_PASS` weak-warn defaults are safe.

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
