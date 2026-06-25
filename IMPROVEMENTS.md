# CloudDrive Hardening Log

Working branch: `loop/hardening`. **Never push `master`** — that auto-deploys to the live server (`.github/workflows/deploy.yml`).

## Verification gates (must pass before every commit)
- Backend: `cd backend && GOOS=linux go build ./... && GOOS=linux go vet ./...`
  (the app targets Linux; `disk.go`/`files.go` use Linux-only syscalls, so plain Windows `go build` fails — this is expected.)
- Backend tests: `cd backend && GOOS=linux go test ./...`
- Frontend: `cd frontend && npm run build` (runs `tsc -b` + `vite build`)

## Baseline (2026-06-25)
- Backend builds & vets clean under `GOOS=linux`. Frontend builds clean.
- Tests: **none existed** — establishing `go test` infra is a priority.
- Known minor: vite warns `api.ts` is both statically and dynamically imported (chunking no-op). Low priority.

---

## Done
### Iter 1 — Authz gaps in files.go + first tests
- **Bug (authz):** `Compress` only checked access on the *first* path's dir, not each source path → a non-admin could include files outside their home folder in a zip. Fixed: per-path `checkAccess` in the loop (matches `Move`/`Copy`).
- **Bug (authz):** `SetTags`/`GetTags` HTTP handlers had no access check → a non-admin could read/write tag metadata on any path under root. Fixed: `checkAccess` gate (SetTags → 403; GetTags → empty list, no leak).
- **Refactor (testability):** extracted `pathWithinHome(path, home)` pure helper from `checkAccess`; behavior unchanged.
- **Tests:** added `backend/handlers/files_test.go` — `pathWithinHome` table (incl. `/Nika` vs `/Nikabackup` sibling-prefix attack) and `safePath` traversal-denial table. Establishes `go test` infra.

### Iter 2 — CRITICAL: MFA bypass (pre-auth token accepted as session)
- **Bug (CRITICAL authz/auth):** `middleware.Wrap` accepted ANY HS256 token with a `sub` claim and never checked `kind`. The `mfa_challenge` token (issued after password, before TOTP) has `sub`, no `homeFolder` (→ `pathWithinHome` returns true, no home confinement), and no `pwv` (→ pwv gate passes for any user at PwVersion 0). Result: **password alone, without the TOTP code, granted ~5 min of unconfined file access**; the 30-day `trusted_device` token was likewise accepted as a session.
- **Fix:** session tokens now carry `kind: "session"` (auth.go `signSessionToken`); `middleware.Wrap` requires exactly that. The `Challenge` handler and `HasValidTrustedDevice` validate their own token kinds directly (not via Wrap), so they're unaffected. One-time effect: existing kind-less session cookies are invalidated → users re-login once.
- **Tests:** `middleware/auth_test.go` — session accepted; mfa_challenge / trusted_device / legacy-no-kind / wrong-secret / alg=none / missing all rejected; pwv enforcement (stale rejected, current accepted).

## Audited — solid, no change needed
- `handlers/share.go` — constant-time password compare, symlink resolution on share subpaths, no password in URLs, ownership-scoped list/revoke, zip-slip guards. Sound.
- JWT alg-confusion (all 3 parse sites assert HMAC), backup-code single-use, MFA enable/disable/regen re-verify password, CSRF (crypto/rand + constant-time + correct method scope), trusted-device forgery resistance. Sound.

### Iter 3 — Trash subsystem: traversal, clobber, id-collision, list race
- **Bug (data loss):** `Restore` used `os.Rename` straight onto the original path → silently overwrote a file created there since the delete. Now restores under a unique `" (restored)"` name (`uniqueDest`).
- **Bug (defense-in-depth traversal):** `Restore` joined `root + OriginalPath` with no containment recheck. Now `resolveRestoreDest` rejects any destination escaping the storage root (the manifest is the only input and could be tampered).
- **Bug (data loss):** `MoveToTrash` id = `<ms>_<base>` collided for same-named files deleted within 1ms → second move clobbered the first's trashed bytes. Now includes a 4-byte random suffix.
- **Bug (data race):** admin branch of `List` returned the live backing slice, JSON-encoded after the lock released while mutators reslice in place. Now returns a copy.
- **Tests:** `services/trash_test.go` — move/restore round-trip, relative-path traversal denied, no-clobber (restores alongside), unique ids (both payloads survive), permission denied for another user.
- Note: per-user home-folder *re-confinement* on restore (beyond root + DeletedBy) deferred — would need homeFolder threaded into Restore; current gates (DeletedBy + root containment + delete-time home check) are adequate.

### Iter 4 — Notifications: unbounded growth + id collisions
- **Bug (CRITICAL, resource):** `Add` documented a "keep last 100 per user" cap that was never implemented → the slice grew forever and `save()` rewrote the whole file (MarshalIndent) on every add → O(n²) disk over the app's life. Now `trimPerUser` caps each user's history at 100 (other users untouched, newest kept).
- **Bug (LOW):** notification id was 1-second granular (`<ts>_<user>`) → collisions within a second made `MarkRead` mark duplicates. Now includes a random suffix.
- **Tests:** `services/notifications_test.go` — per-user cap, other users unaffected, newest survives trim, ids unique, persistence round-trip.

### Iter 5 — Audit-log silent disable + login timing enumeration
- **Bug (HIGH):** `auditlog.go` `NewAuditLogger` returned a no-op logger on `OpenFile` failure with no signal → audit could be silently disabled. Now logs `slog.Error` loudly at construction.
- **Bug (HIGH):** `userstore.go` `Authenticate` skipped bcrypt for unknown usernames → response-time oracle for username enumeration. Now compares against a fixed dummy bcrypt hash (DefaultCost) on the no-match path. Also returns a snapshot copy of the user (clarity; avoids pointer-to-loop-var sharing).
- **Tests:** `services/userstore_test.go` (correct/wrong/unknown + snapshot isolation) and `services/auditlog_test.go` (round-trip newest-first + degrades safely without panic).
- Deferred (latent, not live): `Authenticate`/`GetUser` returned `User.BackupCodes` slice still aliases store backing array — handlers don't read it today; proper fix is a `PublicUser` return type (bigger change).

### Iter 6 — Rate-limiter XFF spoofing (right-most hop)
- **Bug (MED):** behind a trusted proxy with no `X-Real-IP`, `getIP` used the LEFT-most `X-Forwarded-For` entry, which is client-controlled when the proxy appends (`$proxy_add_x_forwarded_for`) → an attacker rotates it to evade the login limiter. Now uses the right-most entry (the hop the trusted proxy actually observed; unforgeable). `X-Real-IP` is still preferred first; untrusted-peer path still ignores headers.
- **Tests:** `middleware/ratelimit_test.go` — getIP table (spoof ignored / X-Real-IP preferred / right-most XFF / peer fallback) + lockout-and-reset.

### Iter 7 — Frontend robustness: session expiry, stale-race, toast dismiss
- **Bug (HIGH):** an expired/invalidated session (401) left the app a dead error-banner husk — no return to login. Added `setOnAuthExpired` hook in `api.ts`; `listFiles` (the navigation/refresh heartbeat) calls `checkAuthExpired` on 401, clearing local auth state and bouncing to `LoginPage` via App's registered handler.
- **Bug (HIGH):** `FileExplorer.refresh` had no stale-response guard — a slow listing for folder A could overwrite folder B after fast navigation. Added a monotonic `refreshSeq` ref; only the latest request applies its result.
- **Bug (MED, logic):** `useToast.addToast` scheduled auto-dismiss only `if (!action)`, so action toasts (Undo) ignored their 8s duration and lived forever. Now always auto-dismisses.
- Verified: `npm run build` (tsc -b + vite) clean.

### Iter 8 — Frontend filter/visual correctness
- **Bug (visual):** with a filter active that matched nothing, the list rendered headers + zero rows and no message (empty check used `files`, body uses `filteredFiles`). Now shows a "No files match this filter" state distinct from the empty-folder state.
- **Bug (mechanical):** the header select-all checkbox toggled/compared against `files` (all loaded) while the body shows `filteredFiles` — so it selected hidden rows and its checked state was wrong under a filter. Now operates on `filteredFiles` and shows an indeterminate state for partial selection.
- **Bug (visual):** `formatSize` could index past the units array for ≥1 PB → "undefined". Added PB + clamped the index.
- Verified: `npm run build` clean.

## Open / found (remaining — lower priority)
**Frontend** (verified, deferred)
- MED: PreviewModal lacks focus trap / aria-modal / focus restore; text preview no AbortController + unbounded `<pre>`.
- MED: bulk delete/download overwrite each other's error banners; no per-item failure summary; bulk download fires N anchor clicks (popup-flood risk).
- LOW: list-view `<img>` missing onError fallback (grid has one); downloadFile revokes object URL synchronously; Ctrl+A selects `files` not `filteredFiles` (left as-is to avoid effect stale-closure risk).
**Backend / perf**
- `handlers/files.go` `List` does an extra `os.ReadDir` per directory entry (itemCount) — N+1 syscalls on large dirs.
**Repo**
- LOW: `frontend/tsconfig.tsbuildinfo` tracked in git (build artifact) — candidate for gitignore.

**Frontend**
- HIGH: session expiry (401) never redirects to login — app becomes a dead error-banner husk (`api.ts` + `FileExplorer.refresh`). Add 401 → onLogout.
- HIGH: stale-response race in `FileExplorer.refresh` (rapid folder nav) — older listing can overwrite newer. Add request-id/AbortController guard.
- MED (real logic bug): `useToast.ts` schedules auto-dismiss only `if (!action)`, so action toasts ignore their duration and live forever → UI clutter.
- MED: PreviewModal no focus trap / aria-modal / focus restore; text preview no abort + unbounded `<pre>`.
- LOW: filtered-empty state not shown; select-all uses `files` not `filteredFiles`; `formatSize` PB overflow; list-view `<img>` missing onError fallback.

**Perf**
- `handlers/files.go` `List` does an extra `os.ReadDir` per directory entry for itemCount (N+1 syscalls on large dirs).
