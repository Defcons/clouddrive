# ToDo — clouddrive

<!-- STRICT deferral ledger (~/.claude/CLAUDE.md §5): the MOMENT anything is set aside /
     deferred / decided-not-now, it gets an entry here — session history routinely loses it.
     Lifecycle: done → check off, prune on the next touch. Never a write-only graveyard. -->

_Touched 2026-08-19 — audit round 5 done; 8 findings fixed + tested on branch `security/audit-round5-fixes` (uncommitted), rest tracked below._

## Open

### 🛠️ Audit round 5 findings (2026-08-19) — 8 FIXED on branch `security/audit-round5-fixes`; rest below
Gates green after the fixes (Linux build/vet + native `go test ./...`, incl. 2 new regression tests:
`TestCompressNameCannotTraverse`, `TestVersioningRestoreOldestAtCap`). NOT yet committed/pushed.
Full in-depth audit: 6 workstreams (auth self-driven + 5 parallel subagents over path/concurrency/
deploy/frontend/DoS). Every item below was personally re-read in source before logging. Narrative +
"confirmed clean" list in `ResearchJournal.md` (round 5). Fix on a BRANCH; gates at bottom.

**HIGH**
- [x] **H1 (FIXED) — `Compress` path-traversal write.** `handlers/files.go:1121` uses `req.Name` verbatim
  (only appends `.zip`); `filepath.Join(absDir, zipName)` at `:1135` is never `filepath.Base`'d or
  `safePath`-rechecked. `name:"../bob/x.zip"` writes into another tenant's home; `"../../../tmp/x.zip"`
  escapes STORAGE_ROOT — and the container runs as **root** (see H-deploy), so it overwrites any
  existing `*.zip` on the host as uid 0. Iter 1 fixed Compress's per-*source* check, never the output
  name. **Fix: `zipName := filepath.Base(req.Name)`** (one line). No test covers it — add one.
- [x] **H2 (FIXED) — Thumbnail decode-bomb OOM.** `handlers/thumbnail.go:108` `image.Decode` runs before any
  dimension check; a ~30000×30000 PNG (tiny on disk) allocates multi-GB RGBA → process OOM. Auth-gated
  but home-confined users qualify, and the frontend auto-requests thumbnails on every listing.
  **Fix: `image.DecodeConfig` first, reject W×H over a cap (~40 MP) before `image.Decode`.**

**MEDIUM**
- [x] **M1 (FIXED) — `Extract` decompression-bomb + quota bypass.** `files.go:1085` unbounded `io.Copy` per
  entry; no per-entry/total/entry-count cap and no quota gate (zip-slip itself IS guarded at `:1067`).
  Small DEFLATE bomb → fills shared `/data` volume. Fix: `io.LimitReader` + running total + the
  `dirSize(home)+extracted>quota` gate `Upload` already uses.
- [x] **M2 (FIXED) — Chunked upload ignores `MAX_UPLOAD_BYTES` + `.uploads` staging leak.** `chunked.go`
  caps 64 MiB/chunk but `index` is unbounded and `UploadComplete` (`:185`) checks only quota; default
  quota is 0 = unlimited, so assembled size is uncapped. `os.RemoveAll(dir)` runs only on success
  (`:238`) — abandoned/failed uploads persist forever, no janitor. The `:16` comment claiming the cap
  is enforced is false. Fix: enforce `assembled<=maxUploadBytes()`; periodic sweep of `.uploads`.
- [x] **M3 (FIXED) — Spoofable audit-log / session IP.** `handlers/auth.go:51` `clientIP` AND its twin
  `handlers/files.go:131` `getClientIP` trust client `X-Forwarded-For`/`X-Real-IP` from any peer;
  only the rate-limiter's `getIP` was hardened (iter 6). Every audit entry's IP is forgeable
  (audit is JSON-marshaled so no log injection; session-list IP self-heals via middleware Touch→getIP).
  Fix: export one `middleware.RealIP(r)` (trusted-proxy logic) and use it in all three sites.
- [x] **M4 (FIXED) — CI actions on floating tags hold the deploy key.** `.github/workflows/deploy.yml:24,32`
  (`tailscale/github-action@v3`, `appleboy/ssh-action@v1`). A compromised action release runs in the
  job holding `DEPLOY_SSH_KEY` + Tailscale secrets → root on the live homelab. Fix: pin to commit SHAs.
- [x] **M5 (RESOLVED via GUI password, not bind) — Syncthing GUI exposed.** The loopback-only bind was
  REVERTED (2026-08-20): it broke the operator's terminal-only (Proxmox root → `pct enter`) access, and
  the real weakness was "no password," not "reachable". Fix is a Syncthing GUI user+password set on the
  host (bcrypt hash in `/opt/syncthing-config/config.xml` `<gui>`, or the Settings→GUI panel). Port is
  back to `8384:8384`. `docker-compose.yml:22` publishes `8384` on `0.0.0.0`; the
  official image serves the GUI with no password until one is set → any LAN/Tailscale peer controls
  sync of `/data`. Fix: `"127.0.0.1:8384:8384"`.
- [ ] **M6 — clouddrive container runs as root + broad `/data` mount.** Implemented on branch
  `security/audit-round6-m6-nonroot` (runs as uid/gid 1000 to match Syncthing's PUID/PGID on the shared
  `/data`). MERGE ONLY AFTER running `chown -R 1000:1000 /data` on the host — otherwise the non-root app
  can't write files clouddrive previously created as root and the container fails. See its PR. `Dockerfile` (no `USER`,
  final stage) + `docker-compose.yml:9`. Amplifies any write bug (incl. H1) to host-root. The static
  `CGO_ENABLED=0` binary needs no root. Fix: `adduser` + `USER`.
- [x] **M7 (FIXED, + `.dockerignore` L7 part) — `.gitignore` misses `data/` and `users.json` (public repo).** Default STORAGE_ROOT is
  `./data`; a local `go run` writes `backend/data/users.json` (bcrypt hashes + TOTP seeds). Currently
  nothing sensitive is tracked, but a stray `git add -A` would publish it permanently. Fix: add
  `data/`, `backend/data/`, `users.json` (and mirror in `.dockerignore`).
- [~] **M8 (PARTIAL) — Off-quota dotdir accumulation.** Runtime `CleanExpired`/`PruneExpired`/upload-sweep
  now run hourly (round-6 maintenance ticker), so the 30-day trash guarantee holds without a restart.
  STILL OPEN: counting `.trash`/`.versions` bytes toward the user's quota (closes the delete→refill
  parking trick) — deferred because the dotdirs are keyed globally, so per-user attribution + a
  free-space semantics decision are needed first. `.trash`/`.versions` live at STORAGE_ROOT, excluded
  from `dirSize(home)`; `TrashStore.CleanExpired` runs only at startup (`main.go:101`). delete→refill
  cycles park N×quota off-quota for up to 30 days (or until restart). Fix: count them toward quota
  and/or run `CleanExpired` on a ticker.
- [x] **M9 (FIXED) — Version-restore data loss at the retention cap.** `services/versions.go:143` — restoring
  the OLDEST of 10 versions snapshots the current file (→11), `pruneLocked` deletes the oldest (= the
  `src` being restored), then `copyFileTo(src,…)` fails ENOENT: the requested version is destroyed and
  the restore doesn't happen. Fix: copy `src` out before the snapshot+prune.
- [x] **M10 (FIXED — bounded `ReadTimeout` 15m; `WriteTimeout` deliberately off so streaming down/zips aren't truncated. Per-route deadlines = fuller fix, deferred) — No `ReadTimeout`/`WriteTimeout` on `http.Server`.** `main.go:194` sets only
  ReadHeaderTimeout + IdleTimeout → slow-body/slow-read holds a goroutine per connection indefinitely.
  Fix: conservative Read/WriteTimeout (or handler deadlines for streaming download/zip paths).
- [x] **M11 (FIXED) — JSON bodies decoded without `MaxBytesReader`.** Mkdir/Rename/Move/Copy/Extract/Compress/
  SetTags (`files.go`), share Create/Revoke, chunked UploadComplete, notifications MarkRead all
  `json.NewDecoder(r.Body)` unbounded → a multi-GB body is buffered into RAM (Upload/share-upload DO
  cap, so the pattern was just omitted). Fix: shared `MaxBytesReader(~1 MB)` decode helper.

**LOW** (polish / defense-in-depth — detail in ResearchJournal round 5)
- [x] **Fixed on branch `security/audit-round5-followup`: L1, L4, L5.** L1 cleanup no longer evicts a
  locked-out entry early; L4 the share-auth cookie stores a derived value, not the plaintext password
  (`TestSharePasswordCookieIsDerived`); L5 the public directory-zip endpoint is per-IP rate-limited
  (`shareDownloadLimiter`). Still open: L2, L3, L6, L7 (base-image pinning / compose hardening), L8.
- [x] **Fixed on branch `security/audit-round6-hardening`: L2, L3, L6 + M5, M10, M8-partial + L7 compose/
  workflow hardening.** L2 hourly maintenance ticker (trash/sessions/uploads); L3 audit log rotates at
  5 MiB to `.audit.log.1` and `GetRecent` reads both generations, bounded (`TestAuditLogRotation`); L6
  deleted the dead `UserStore.save()`. STILL OPEN: L7 base-image DIGEST pinning (needs a registry/docker
  to resolve — deferred); L8 PDF-iframe `sandbox` (SKIPPED — already neutralized by the global `nosniff`;
  sandboxing risks breaking the PDF viewer for no real gain).
- [ ] L1 rate-limiter `cleanup` lifts lockout early when `lockout>2*window` (both limiters are;
  `ratelimit.go:49`). L2 sessions pruned only at startup (growth + stale "active sessions"). L3 audit
  log unbounded + `GetRecent` reads whole file under the write lock (`auditlog.go:66`). L4 share
  password stored as the literal cookie value (`share.go:383`; random token, HttpOnly/Secure/scoped →
  low; prefer an opaque/HMAC token). L5 no-password `/share/` has no rate limit + re-walks the dir zip
  each hit. L6 dead `UserStore.save()` has an unlock-then-marshal race pattern (`userstore.go:92`) —
  delete it. L7 deploy hardening: no `permissions:{}`, unpinned base images / `syncthing:latest`, no
  `no-new-privileges`/`cap_drop`, `.dockerignore` misses `.env`/`data`. L8 PDF preview iframe keyed on
  extension — already mitigated by global `nosniff`; optional `sandbox` for depth.

**Verification gates (run before any commit):**
- Backend: `cd backend && GOOS=linux go build ./... && GOOS=linux go vet ./... && go test ./...`
- Frontend: `cd frontend && npm run build`
- Work on a BRANCH — never push `master` (it auto-deploys to the live server).

## Blocked / needs the user
(nothing yet)

## Done (prune on next touch)
(nothing yet)
