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

## Open / found (verified hypotheses from parallel audit 2026-06-25 — fix next, each needs repro+test)
**Backend**
- HIGH (trash): `services/trash.go` `Restore` does NOT re-validate `OriginalPath` against root (no safePath) nor home-folder confinement, and `os.Rename` silently clobbers an existing file at the destination → data loss. Also `MoveToTrash` id = `<ms>_<base>` can collide within 1ms. Authz is `DeletedBy==username`, weaker than the file handler's home confinement.
- CRITICAL (notifications): `services/notifications.go` `Add` comment says "keep last 100" but never trims → unbounded slice + full-file O(n) rewrite per add (O(n²) over time).
- HIGH (audit log): `services/auditlog.go` if `OpenFile` fails, `Log` silently no-ops with no startup error → audit silently disabled. Make it log/fail-fast.
- HIGH (login enum): `services/userstore.go` `Authenticate` skips bcrypt when user not found → timing oracle for username enumeration. Fix: compare against a dummy hash.
- MED (rate limit): `middleware/ratelimit.go` keys on left-most XFF behind trusted proxy; if proxy appends (nginx default `$proxy_add_x_forwarded_for`) the client-supplied left-most value is trusted → limiter bypass. Prefer X-Real-IP / right-most hop.
- MED (trash list race): admin branch of `List` returns the live backing slice; encoded after lock released while mutators reslice in place → data race. Return a copy.
- LOW: notification ID 1-second granularity collision; tracked build artifact `frontend/tsconfig.tsbuildinfo` should be gitignored.

**Frontend**
- HIGH: session expiry (401) never redirects to login — app becomes a dead error-banner husk (`api.ts` + `FileExplorer.refresh`). Add 401 → onLogout.
- HIGH: stale-response race in `FileExplorer.refresh` (rapid folder nav) — older listing can overwrite newer. Add request-id/AbortController guard.
- MED (real logic bug): `useToast.ts` schedules auto-dismiss only `if (!action)`, so action toasts ignore their duration and live forever → UI clutter.
- MED: PreviewModal no focus trap / aria-modal / focus restore; text preview no abort + unbounded `<pre>`.
- LOW: filtered-empty state not shown; select-all uses `files` not `filteredFiles`; `formatSize` PB overflow; list-view `<img>` missing onError fallback.

**Perf**
- `handlers/files.go` `List` does an extra `os.ReadDir` per directory entry for itemCount (N+1 syscalls on large dirs).
