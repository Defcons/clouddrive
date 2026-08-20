# OrientationMap — CloudDrive

<!--
  A THIN, POINTER-BASED index of this codebase. Read first, update after changes.
  Anchor to SYMBOL names (never line numbers). Only what's expensive to rediscover.
  Behaviour facts → KnowledgeBase.md · chronology → ResearchJournal.md.
  Feature detail already lives in FEATURES.md; hardening saga in IMPROVEMENTS.md — link, don't copy.
-->

_Last verified: 2026-08-20 @ ff7b489 — audit round 5 shipped to prod; followup branch adds M4/L1/L4/L5 + a deploy-ops landmine below._

## What this is
Self-hosted single-binary personal cloud / file explorer for the homelab. A Go 1.22 HTTP
server (`module clouddrive`) that **embeds the built React frontend** and stores everything as
files on disk (no database). Deployed via Docker Compose alongside a Syncthing container.
Entry point: `backend/main.go`. Data lives under `STORAGE_ROOT` (a mounted volume).

## Subsystems (where things live)
- **Server bootstrap / routing** → `backend/main.go` — env config, store wiring, route registration (`registerXxxRoutes`), embedded static file serving (`//go:embed static/*`), graceful shutdown, `--hash-password` CLI mode.
- **HTTP handlers** → `backend/handlers/*.go` — one file per area: `auth`, `mfa`, `files` (+ `chunked`, `thumbnail`, `versions`, `walk`), `share`, `permissions`, `trash`, `admin`, `notifications`, `audit`, `backuptiers`, `disk`, `webdav`, `version`. Each returns a `NewXxxHandler` constructor used in `main.go`.
- **Middleware** → `backend/middleware/` — `auth.go` (`AuthMiddleware.Wrap`, JWT), `csrf.go` (`CSRFMiddleware`), `ratelimit.go` (`RateLimiter`), `security.go` (`SecureHeaders`).
- **Services (logic + persistence)** → `backend/services/*.go` — `userstore`, `sessions`, `permissions`, `auditlog`, `trash`, `tags`, `versions`, `notifications`, `backuptiers`, `mfa`; shared JSON helpers in `jsonstore.go`. Every store persists to a JSON file under `STORAGE_ROOT`.
- **Models** → `backend/models/user.go` — `User` (bcrypt `Password`, `Role`, `HomeFolder`, `PwVersion`, `Quota`, MFA fields) + `PublicUser` (never serialize the hash).
- **Frontend** → `frontend/src/` — React 18 + TypeScript + Vite + Tailwind PWA. `App.tsx` (root), `api.ts` (API client), ~40 `components/*.tsx`, `hooks/*`, `types.ts`. Built to `frontend/dist/`.
- **Deploy** → `Dockerfile` (3-stage: build frontend → embed into Go `static/` → build binary → alpine), `docker-compose.yml` (clouddrive + syncthing), `.github/workflows/deploy.yml` (push `master` → auto-deploy).

## Invariants & gotchas
- **session-token-kind**: session JWTs carry `kind: "session"` (`AuthHandler.signSessionToken`); `AuthMiddleware.Wrap` accepts **only** that kind. Pre-auth `mfa_challenge` / `trusted_device` tokens must never pass `Wrap` — this was a real CRITICAL MFA-bypass (see IMPROVEMENTS.md Iter 2).
- **path-confinement**: every file operation must go through `FileHandler.safePath` (resolve + traversal-deny under `STORAGE_ROOT`) **and** `checkAccess` / `pathWithinHome` (non-admins confined to their `HomeFolder`; sibling-prefix like `/Nika` vs `/Nikabackup` is guarded). Never join user path onto root without both.
- **protected-write**: all mutating routes are wrapped by `protectedWrite` = `AuthMiddleware.Wrap` + `CSRFMiddleware.Protect`. Read routes use `Wrap` only. Public `/share/` routes are unauthenticated by design.
- **cors-same-origin**: the frontend is served same-origin by this server, so CORS is **off by default**. `ALLOWED_ORIGINS` exists only for public `/share/`; never combine `AllowCredentials` with a wildcard origin.
- **reserved-dotdirs**: `.versions`, `.trash`, `.thumbs`, `.uploads` (dirs) and `.sessions.json` / `users.json` / other store files live under `STORAGE_ROOT` and are excluded from listing/search/recent/disk. Don't surface them as user files.
- **atomic-json-writes**: stores persist via `saveJSONFile` (marshal → temp → rename, mode 0600). `loadJSONFile` preserves a corrupt file as `<path>.corrupt` and starts empty rather than overwriting recoverable data.
- **path-metadata-follows**: per-path metadata (permissions, tags, backup tiers) is re-keyed on rename/move (`movePathKeys`) and pruned on permanent delete (`prunePathKeys`, wired via `trashStore.SetMetadataPruner`) so a recreated path can't inherit stale state.
- **env-fail-fast**: `JWT_SECRET` must be set, ≥32 chars, not the old default, or the server refuses to start. Weak/placeholder `CLOUDDRIVE_PASS` only warns.
- **webdav-opt-in**: `/webdav` mounts only when `WEBDAV_ENABLED=1`. It uses HTTP Basic Auth and **bypasses MFA** (protocol limit) — HTTPS only.
- **trusted-proxy**: only peers in `TRUSTED_PROXIES` may set `X-Forwarded-For` / `X-Real-IP`; otherwise the rate limiter uses the direct connection IP (prevents header-spoofed bypass).
- **realip-unified**: handlers get the client IP for audit-log / session records via `middleware.RealIP` — the SAME trusted-proxy logic as the rate limiter's `getIP`. Never read raw `X-Forwarded-For`/`X-Real-IP` in a handler (that was spoofable; fixed round 5). `handlers.clientIP` / `handlers.getClientIP` are thin wrappers over it.
- **write-name-base**: every handler that creates a file from a client-supplied name MUST `filepath.Base` it before joining to the target dir (Upload, Mkdir, Rename, chunked UploadComplete, **Compress**). `Compress` was the lone gap — an arbitrary cross-tenant / out-of-root `*.zip` write — fixed round 5 (`TestCompressNameCannotTraverse`).
- **resource-caps**: request-driven work is bounded — JSON bodies via `decodeJSON` (1 MiB), whole-file uploads via `maxUploadBytes()` (single POST **and** chunked assembly **and** `Extract` output), image thumbnails via `maxThumbPixels` (`image.DecodeConfig` before `image.Decode`). Add the matching cap when you add a new request-sized operation.

## Contracts between modules
- **frontend → backend**: `npm run build` emits `frontend/dist/`; the Dockerfile copies it into `backend/static/` which the Go binary embeds and serves. The checked-in `backend/static/.gitkeep` is the only tracked file there (`static/*` is gitignored) — the real assets exist only in the built image.
- **auth**: browser holds an **HttpOnly session cookie** (JS can't read it); the client calls `GET /api/auth/check` for identity and `GET /api/csrf` for the write token (`X-CSRF-Token`). See `frontend/src/api.ts`.
- **stores → handlers**: services are injected into handlers in `main.go` (e.g. `FileHandler.SetQuotaLookup(userStore.GetQuota)`, `SetVersionStore`) — optional setters keep constructors stable and tests simple.

## Known landmines / deferred
- **Version history is keyed by path** (`sha256(path)` under `.versions`) → a rename/move orphans old versions and starts fresh history. Accepted for v1.
- **Collaborate-share anonymous uploads don't check the owner's quota** yet (documented follow-up in FEATURES.md #6).
- **WebDAV has no MFA and admin sessions see app dotdirs under root** (documented; app-passwords are the proper follow-up).
- **Linux-only build**: `disk.go`/`files.go` use Linux syscalls — verify with `GOOS=linux go build/vet/test ./...`; plain Windows `go build` fails by design (see IMPROVEMENTS.md verification gates).
- **`master` auto-deploys** to the live homelab server on push (`.github/workflows/deploy.yml`). Do feature/hardening work on a branch; never push `master` casually.
- **Deploy git-perms wedge (operational, seen 2026-08-20):** the deploy SSHes in as the `deploy` user and runs `git fetch` in `/opt/clouddrive`. If any `.git/objects` shard drifts to root ownership (a `git`/`docker` command run as root there), fetch dies with `insufficient permission for adding an object to repository database .git/objects` and `set -e` aborts BEFORE the container is touched (live site stays up on the old build — deploys just silently stop landing). Fix on the box, as root: `chown -R deploy:deploy /opt/clouddrive`. Prevention: never run git as root in that checkout.
