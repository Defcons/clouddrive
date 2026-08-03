# Knowledge Base — CloudDrive

<!--
  The distilled, canonical TRUTH about how CloudDrive behaves. The MODEL.
  Tag every claim FACT / HYPOTHESIS / ASSUMPTION / UNKNOWN — never mix tiers.
  Code-owned numbers: record with a symbol pointer, code wins conflicts.
  Navigation → OrientationMap.md · chronology → ResearchJournal.md. Cross-link, don't copy.
-->

_Last verified: 2026-08-03 @ 14b9eb5 — by Claude (Opus 4.8), triad seed._

## How to read this doc
- **[FACT]** — confirmed by code inspection or repeated evidence.
- **[HYP, NN%]** — hypothesis; carries confidence + evidence + settling experiment.
- **[ASSUMPTION]** — believed but unverified.
- **[UNKNOWN]** — open question.

## Architecture & storage
- **[FACT, 100%]** Single Go binary that serves both API and the embedded React frontend; no external database — all state is JSON/blob files under `STORAGE_ROOT` — _evidence: `//go:embed static/*` and every `services/*Store` persists via `saveJSONFile`_ (`backend/main.go`, `services/jsonstore.go`).
- **[FACT, 100%]** JSON stores are written atomically (temp file → `os.Rename`, mode 0600) and a corrupt store is preserved as `<path>.corrupt` rather than clobbered — _evidence: `saveJSONFile` / `loadJSONFile`_ (`services/jsonstore.go`).
- **[FACT, 95%]** User records store a bcrypt password hash, a role (`admin`|`user`), a home folder, a `PwVersion` (bumped on password change to invalidate old tokens), an optional byte `Quota` (0 = unlimited), and TOTP MFA fields — _evidence: `models/user.go`; hashing via `golang.org/x/crypto/bcrypt`._
- **[FACT, 90%]** Runtime config is entirely environment-driven: `PORT` (default 8080), `STORAGE_ROOT` (default `./data`), `JWT_SECRET` (required), `CLOUDDRIVE_USER`/`CLOUDDRIVE_PASS` (seed admin), `USERS_FILE`, `WEBDAV_ENABLED`, `ALLOWED_ORIGINS`, `TRUSTED_PROXIES`, `MAX_UPLOAD_BYTES` — _evidence: `os.Getenv` calls in `main.go`; documented in `.env.example`._

## Authentication & authorization
- **[FACT, 100%]** Sessions are JWTs in an **HttpOnly cookie** carrying `kind: "session"`; the auth middleware accepts only that kind, so pre-auth `mfa_challenge` and long-lived `trusted_device` tokens cannot act as a session — _evidence: `AuthHandler.signSessionToken`, `AuthMiddleware.Wrap` kind check_ (`handlers/auth.go`, `middleware/auth.go`). This closed a CRITICAL MFA bypass (ResearchJournal / IMPROVEMENTS.md Iter 2).
- **[FACT, 100%]** Session lifetime is 24h; the MFA-trusted-device skip is 90 days — _code-owned: `JWTLifetime` (`handlers/auth.go`), `trustedDeviceLifetime` (`handlers/mfa.go`); code wins._
- **[FACT, 95%]** All mutating endpoints require both a valid session and a CSRF token (`protectedWrite` = `Wrap` + `CSRFMiddleware.Protect`); read endpoints require only a session; `/share/` public endpoints require neither — _evidence: route registration in `main.go`._
- **[FACT, 95%]** Non-admin file access is confined to the user's home folder; admins reach root. Enforced by `safePath` (traversal-deny under root) + `pathWithinHome` + `checkAccess`, with a sibling-prefix guard (`/Nika` ≠ `/Nikabackup`) — _evidence: `handlers/files.go`, tests in `handlers/files_test.go` & `authz_test.go`._
- **[FACT, 90%]** Login, MFA challenge, and share-password verification are rate-limited (per-IP, with lockout) — _evidence: `NewRateLimiter(...)` instances and `WrapLogin` in `main.go` / `middleware/ratelimit.go`._

## Features (confirmed present)
- **[FACT, 90%]** Beyond basic file CRUD the server implements: cached image thumbnails, HTTP Range download/preview, chunked/resumable uploads, trash with restore, file version history, per-user quotas, admin user management, active-session list/revoke, filename + opt-in content search, shares (incl. password + collaborate-upload), tags, backup tiers, audit log, notifications, and opt-in WebDAV — _evidence: `registerXxxRoutes` in `main.go`; each feature detailed in `FEATURES.md`._
- **[FACT, 85%]** The frontend is a PWA (manifest + service worker `sw.js` + icons) — _evidence: `frontend/public/` and built `frontend/dist/`._

## Deployment
- **[FACT, 90%]** Runs as two Docker Compose services — the app (built from the multi-stage `Dockerfile`, port 8080) and a Syncthing container that syncs the same data volume — _evidence: `docker-compose.yml`._ Data and Syncthing config are host volumes; specifics live in that file, not here.
- **[FACT, 85%]** Public GitHub repo; pushing `master` triggers a GitHub Actions job that connects over Tailscale and SSHes to the homelab server to `git reset --hard` + `docker compose up --build` — _evidence: `.github/workflows/deploy.yml`._ Host/user/port/keys are GitHub Actions **secrets** (no values in the repo).

## Confirmed issues — OPEN
- **[OPEN]** Collaborate-share anonymous uploads don't enforce the owner's quota (`FEATURES.md` #6 NOTE).
- **[OPEN]** WebDAV bypasses MFA (Basic Auth only) and admin WebDAV sessions expose app dotdirs under root (`FEATURES.md` #5 NOTE).
- **[OPEN]** Version history keyed by path → rename/move orphans old versions (`FEATURES.md` #7 NOTE).
- **[OPEN]** Cross-reload upload resume exists server-side but the client doesn't persist the uploadId yet (`FEATURES.md` #3 NOTE).

## Open questions / future experiments
- **[UNKNOWN]** Is `.env` on the live server hand-maintained, or does the deploy leave the `.env.example` fallback in place? The workflow copies the example and warns if `.env` is missing — the real credential source on the host is not visible from the repo.
- **[UNKNOWN]** Are the frontend `dist/` build artifacts committed intentionally? They exist on disk but `frontend/dist/` is gitignored and not tracked — the served copy comes from the Docker build.

## Confidence summary
Best-understood (≥90%): storage model, auth/authz invariants, config surface, feature set. Weakest (UNKNOWN): live-host credential handling and whether committed `dist/` matters — neither blocks work, both answerable by inspecting the server or `git ls-files`.
