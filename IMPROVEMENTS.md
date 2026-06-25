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

### Iter 9 — PreviewModal a11y + robustness
- **Bug (a11y):** modal had no `role="dialog"`/`aria-modal`, didn't move focus in on open or restore it on close, and didn't trap Tab → keyboard/screen-reader users could tab behind it and lost focus on close. Added dialog role + aria-label, focus-in/restore, and a minimal Tab focus trap.
- **Bug (race):** text-preview fetch had no stale guard — a slow load for a previous file could overwrite a newer one. Added a cancelled-flag guard (resets content on file change).
- **Bug (perf/visual):** text preview rendered the entire file into a `<pre>` → multi-MB logs froze the tab. Now truncates display at 200 KB with a download hint.
- Verified: `npm run build` clean.

### Iter 10 — Bulk-op feedback + list thumbnail fallback
- **Bug (MED):** bulk download/delete called `setError` inside the loop, so only the last failure showed and there was no success feedback. Now aggregate failures and show one summary toast (consistent with the rest of the app), plus a success toast on full success.
- **Bug (LOW, visual):** list-view image thumbnails had no `onError`, so a thumbnail that 403s/fails to decode showed a broken-image glyph. Now falls back to the file-type icon (mirrors the grid view).
- Verified: `npm run build` clean.

### Iter 11 — Download URL revoke timing + untrack build artifact
- **Bug (LOW):** `downloadFile` revoked the object URL synchronously after `click()`, which can abort large downloads in some browsers. Now deferred 1s.
- **Repo hygiene:** `frontend/tsconfig.tsbuildinfo` (a tsc build artifact) was tracked in git → `git rm --cached` + added to `.gitignore`.
- Verified: `npm run build` clean.

### Iter 12 — CRITICAL authz gaps in backup-tier + permissions read (fresh audit)
- **Bug (CRITICAL authz):** `backuptiers.go` `Set`/`Get` had NO per-path check → any non-admin could set/read the backup tier of ANY path (e.g. another user's home). `List` returned the whole tier map → leaked every tiered folder's path. Fixed: `Set`/`Get` now gate on `userCanAccess`; `List` is admin-only.
- **Bug (CRITICAL info-disclosure):** `permissions.go` `GetPermission` returned a path's owner + full `allowedUsers` to ANY authenticated user. Fixed: gate on `userCanAccess`, return `isPrivate:false` (no leak) on denial.
- **Refactor:** extracted the shared `userCanAccess(r, permStore, path)` gate; `FileHandler.checkAccess` now delegates to it (identical behavior) so handlers can't drift out of the authz pattern.
- **Tests:** `handlers/authz_test.go` — integration-style through the real auth middleware: GetPermission no-leak cross-home, backup-tier Set denied cross-home, List admin-only.
- Audited SOUND (no change): `walkFilesNoSymlinks` (Lstat-based symlink skip), `tags.go` HTTP handlers (already gated), all three JSON stores' intra-store concurrency (mutex + tmp+rename).

### Iter 13 — Corrupt-JSON data loss across all stores
- **Bug (data integrity):** all five JSON stores (permissions/tags/backuptiers/notifications/trash) ignored `json.Unmarshal` errors in `load()` → a corrupt/truncated file silently loaded as empty and the next `save()` overwrote it, permanently discarding recoverable data. Added a shared `loadJSONFile` helper that logs the error and preserves the bad file as `<path>.corrupt` before starting empty; routed all five `load()` methods through it.
- **Tests:** `services/jsonstore_test.go` — valid load, missing-file no-op, corrupt→preserved-as-.corrupt, and a TagStore integration proving the store survives + stays usable.

## Open / found (remaining — lower priority)
**Backend** (verified, deferred)
- LOW: stale keys in permissions/tags/backuptiers stores never pruned on Delete/Rename/Move (a recreated path inherits old ACL/tier).
- LOW (hardening): `middleware/security.go` CSP allows `style-src 'unsafe-inline'`.
### Iter 14 — Search race + modal backdrop data-loss
- **Bug (MED race):** SearchResults search-as-you-type had no stale guard — a slow earlier query could resolve after a newer one (and setState after unmount). Added a cancelled-flag guard.
- **Bug (MED data loss):** ShareModal/SettingsModal/BatchRename closed on any backdrop `onClick`, so selecting text in an input and releasing on the backdrop discarded unsaved input. Switched to `onMouseDown` with a target check (mirrors `Modal.tsx`).
- Verified: `npm run build` clean.

### Iter 15 — Quick-access storage crash + ContextMenu Escape
- **Bug (crash):** `addQuickAccess`/`removeQuickAccess` wrote to localStorage unguarded → in Safari private mode or at quota the throw propagated out of a click handler and could blank the app via ErrorBoundary. Wrapped writes in `writeQuickAccess` (try/catch; quick access is non-critical).
- **a11y:** ContextMenu now closes on Escape (was outside-mousedown only).
- Verified: `npm run build` clean.

### Iter 16 — /api/disk cross-user leak (fresh audit round 2)
- **Bug (HIGH info-disclosure):** `GET /api/disk` returned the per-user breakdown (every user's folder name + byte size) and global totals to ANY authenticated user — no admin gate (unlike `audit.go`/`backuptiers.go`). Fixed: non-admins now see only their own home folder's usage; admins see all. Filesystem total/free is unchanged (not user-sensitive).
- **Tests:** added to `handlers/authz_test.go` — non-admin sees only own usage, admin sees all.
- Audited SOUND (no change): `/api/audit` (admin-gated), notifications handlers (recipient scoped to session username server-side — no cross-user read/mark), `/api/version` (no sensitive data).

### Iter 17 — NotificationBell polling hardening
- Added `.catch` + mounted guards to the 30s unread poll and the on-open fetch (network failure no longer throws an unhandled rejection every 30s; no setState-after-unmount), and wrapped mark-all-read in try/catch.

### Iter 18 — TrashView/RecentFiles error feedback
- **Bug (MED):** TrashView restore/delete/empty swallowed errors (`catch {}`) → destructive actions failed silently (user assumes success). Added a local error banner shown on any failure (these modals are self-contained, no shared toast).
- **Bug (MED):** RecentFiles load failure showed a false "No recent files" empty state. Now distinguishes a load error ("Couldn't load…") from genuinely empty + mounted guard.
- Also switched both modals' backdrop close to `onMouseDown`+target-check (no accidental close on drag-release).
- Verified: `npm run build` clean.

### Iter 19 — LoginPage MFA dead-end + FileInfoPanel image fallback
- **Bug (MED):** LoginPage called `onLogin()` inside the try, so a post-auth navigation error surfaced as "Invalid code"/"Invalid credentials" — and since the TOTP is consumed on success, the user got stuck re-entering a used code. `onLogin()` now runs after the try/finally, gated on an `authed` flag (both password and MFA steps).
- **Bug (LOW visual):** FileInfoPanel image preview had no `onError` → broken-image glyph on a failed/denied preview. Now falls back to the file-type icon.
- Verified: `npm run build` clean.

### Iter 20 — a11y + formatSize consistency
- TagPicker color-swatch tag toggles now expose `aria-label` (tag name) + `aria-pressed` (selection state) — were color/title only.
- TrashView and RecentFiles had their own copies of `formatSize` with the same PB-overflow fixed earlier in FileExplorer; clamped both.
- Verified: `npm run build` clean.

### Iter 21 — Full green sweep + Ctrl+A consistency
- Verified the whole branch end-to-end: backend `go test ./...` passes natively, Linux build/vet/test-compile clean, frontend `npm run build` clean. 60 test cases.
- Ctrl+A now selects the visible `filteredFiles` (consistent with the header select-all checkbox), with `filteredFiles` added to the keydown effect deps to avoid a stale closure.

### Iter 22 — Final-audit fixes (UpdateToast / AuditLogModal / useTheme)
- **Bug (MED):** UpdateToast's in-flight `/api/version` fetch could `setVisible` after unmount. Added a cancelled-flag guard.
- **Bug (MED):** AuditLogModal swallowed load errors (empty catch) → a fetch failure looked like an empty log to the admin. Added an error state ("Couldn't load…") + mounted guard + safer backdrop close.
- **Bug (LOW crash):** useTheme read+wrote localStorage unguarded → could throw in private mode/disabled storage. Wrapped both in try/catch (falls back to OS theme; theme still applies in-session).
- Audited SOLID (no change): UploadZone, FileFilter, useLongPress, BulkContextMenu (presentational / correct cleanup).
- Verified: `npm run build` clean.

## Open / found (remaining — low priority / needs design input)
**Frontend** (LOW)
- TrashView/RecentFiles hand-rolled modals lack focus trap (would adopt `Modal.tsx`, but its fixed chrome changes layout — needs runtime check).
- ShareModal `generated` latch (can't change expiry without reopening) + dead "Generating…" branch.
- ContextMenu no arrow-key nav.
- `formatSize` differs intentionally (FileExplorer `—` for 0 vs `0 B` elsewhere) — NOT safe to extract to one util without a behavior decision.

### Iter 23 — Migrate metadata on rename/move (private-folder-goes-public bug)
- **Bug (HIGH correctness/security):** Rename/Move used `os.Rename` but never updated the permission/tag/backup-tier stores, so the entry was orphaned at the old path. A **renamed/moved private folder silently became public** (no entry at the new path → CanAccess open), and lost its tags/tier. Added `MovePath(old,new)` to all three stores (re-keys the path + descendants via a shared generic `movePathKeys`), and `FileHandler.migrateMetadata` called after successful Rename and per-item Move.
- **Tests:** `services/movepath_test.go` — permission move (folder + descendant migrate, old gone, sibling untouched), sibling-prefix not matched (`/a/b` vs `/a/bc`), tag move.
- Resolves the Move half of the earlier "stale keys" item; the **Delete** half remains a design decision (see below).

### Iter 24 — Prune metadata on permanent delete (stale-key story complete)
- **Bug (correctness):** per-path metadata (permissions/tags/tier) was never dropped when a file was permanently removed, so a path of the same name created later could inherit a stale ACL/tier/tags. Resolved the design question the safe way: trashed items KEEP their metadata (Restore round-trips unchanged), and metadata is pruned only on PERMANENT removal — `PermanentDelete`, `EmptyTrash`, and the 30-day `CleanExpired`.
- Added `PrunePath(path)` (exact + descendants, shared generic `prunePathKeys`) to the three stores, a `SetMetadataPruner` callback on `TrashStore`, wired in main.go to prune all three. Restore deliberately does NOT prune.
- **Tests:** `services/movepath_test.go` — PrunePath removes path+descendants but not a sibling-prefix; permanent delete triggers the pruner with the original path.
- Stale-key story now complete: Move/Rename migrate (iter 23), permanent delete prunes (iter 24), Restore preserves.

### Iter 25 — Share upload was CSP-broken; removed all inline JS
- **Bug (functional):** the collaborate-share upload UI in `serveBrowsePage` used inline `<script>` + `onclick`/`onchange` handlers, but the global CSP is `script-src 'self'` (no `'unsafe-inline'`) — so the browser blocked all of it and the upload button did nothing. Replaced with a no-JS multipart `<form>` (visible file input + submit) and made `ShareHandler.Upload` do Post/Redirect/Get back to the browse page (was returning raw JSON the form submit would navigate to). The share surface now has ZERO inline JS, so `script-src 'self'` is honored with no exceptions.
- **Tests:** `handlers/share_test.go` — browse page contains no `<script`/`on*=`/`addEventListener` and still renders a working file+submit upload form.
- **Decision (CSP style-src):** left `style-src 'unsafe-inline'` as-is. The share pages style with inline `<style>`/`style=""`; externalizing all of it is a large, low-value refactor (style injection is far less dangerous than script, and all interpolated values are `html.EscapeString`'d). Documented rather than chased.
- **Decision (Copy metadata):** left Copy NOT inheriting permissions/tags/tier. Traced it — not a leak: non-admins can only copy within their own home (checkAccess gates source+dest), and a copy is reasonably a fresh public object. Changing it would be a behavior surprise on a hunch.

### Iter 26 — Modal focus-trap a11y (shared useDialog hook)
- Added `src/hooks/useDialog.ts`: wires Escape-close + focus-in-on-open + focus-restore-on-close + Tab-trap onto an existing container ref — **no layout change** (the reason full `Modal.tsx` adoption was deferred).
- Applied to all six hand-rolled modals (ShareModal, SettingsModal, BatchRename, TrashView, RecentFiles, AuditLogModal): each container now has `ref` + `role="dialog"` + `aria-modal="true"` + `tabIndex=-1`, and their duplicate Escape effects were removed. PreviewModal keeps its own equivalent inline impl (iter 9).
- Verified: `npm run build` clean.

## Open / found (remaining — low priority, deferred with reason)
**Frontend** (LOW)
- ContextMenu/BulkContextMenu/FileFilter dropdowns lack arrow-key nav (consistent, low impact; Escape + outside-click already work).
**Backend** (deliberate, documented)
- CSP `style-src 'unsafe-inline'` retained (share pages style inline; values escaped — low risk). Copy doesn't inherit metadata (not a leak; by design).

---
**Status:** all CRITICAL/HIGH/MED findings across 4 audit rounds are fixed with tests; remaining items are LOW polish or deliberate decisions. Branch is green (backend `go test ./...`, Linux build/vet/test-compile; frontend `npm run build`).
**Backend**
- LOW: stale keys in permissions/tags/backuptiers stores never pruned on Delete/Rename/Move; CSP `style-src 'unsafe-inline'` (share pages use inline styles — needs care).
**Frontend** (verified, deferred)
- MED: ShareModal/SettingsModal/BatchRename still hand-roll overlays without focus trap/restore/aria (full `Modal.tsx` adoption deferred — its fixed `max-w-md` chrome would change their layouts; needs runtime verification).
- MED: ShareModal `generated` one-way latch hides the form + has a dead "Generating…" branch.
- LOW: ContextMenu still lacks arrow-key nav; Ctrl+A selects `files` not `filteredFiles`.
**Backend** (verified, deferred)
- LOW: stale keys in permissions/tags/backuptiers stores never pruned on Delete/Rename/Move; CSP `style-src 'unsafe-inline'` (share pages use inline styles — needs care).
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
