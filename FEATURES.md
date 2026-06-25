# CloudDrive Feature Build

Branch: `feat/enhancements` (off `loop/hardening`). Same verification gates as IMPROVEMENTS.md:
- Backend: `cd backend && GOOS=linux go build ./... && GOOS=linux go vet ./... && go test ./...`
- Frontend: `cd frontend && npm run build`

Goal: implement features #1–#9 from the recommendations, each polished + tested, one commit per feature. Never push master.

## Plan / status
1. **Thumbnail endpoint + cache** — fast image grids/lists (pure-Go scaler, cached). — ✅ DONE
2. **HTTP Range support** for download/preview — video seek + resumable. — ✅ DONE
3. **Chunked/resumable uploads** — large uploads survive drops. — ✅ DONE
4. **Admin user-management UI** — add/remove users, roles, home folders. — ✅ DONE
5. **WebDAV endpoint** — mount as a native drive (needs `x/net/webdav`). — TODO (dep check)
6. **Per-user storage quotas** — enforce caps on upload. — ✅ DONE
7. **File versioning** — keep previous copy on overwrite. — ✅ DONE
8. **Active-session management** — list + revoke signed-in devices. — ✅ DONE
9. **Full-text / content search** — search inside text files. — ✅ DONE

## Done
### #3 — Chunked / resumable uploads
- Backend: `POST /api/files/upload/chunk` stores parts under `.uploads/<id>/<index>.part` (temp+rename per chunk, 64 MB/chunk cap, ownership claimed via `.owner` file so a guessed id can't be hijacked); `GET .../status` lists received indices (resume); `POST .../complete` verifies all parts, enforces quota on the assembled size, snapshots a version on overwrite, assembles temp+rename, cleans up. All access-checked; uploadId regex-validated (no traversal).
- Frontend: `uploadFileChunked` (8 MB chunks, 3 retries/chunk, progress, 401→login). `handleUpload` routes files >8 MB through it, small files stay on the batched single POST.
- Tests: `handlers/chunked_test.go` — assemble 3 chunks → exact bytes + staging cleaned, missing chunk → 400, foreign-writer → 403.
- NOTE: per-chunk retry survives transient drops within a session; cross-reload resume is possible via the status endpoint but the client doesn't yet persist the uploadId (follow-up).

### #9 — Content / full-text search
- `Search` gains an opt-in `&content=1` pass: after filename matches, it scans text files (~25 known extensions, ≤512 KB each, home-scoped, permission-checked, symlink/dot-skipped, 100-result cap) for the query and appends matches with a one-line `Snippet` (deduped against filename hits). Dependency-free.
- Frontend: a "Search inside file contents" checkbox in the search dropdown (default off to keep instant search fast) + snippet shown under content matches.
- Tests: `handlers/search_test.go` — content match found, binary-ext ignored, filename-only excludes content, dedupe.
- NOTE: text only; PDF/Office content indexing needs a parser dependency (follow-up).

### #8 — Active session management
- `SessionStore` (persisted `.sessions.json`): Create→id, IsValid, Touch (in-mem last-seen), List, Revoke (owner/admin), RevokeAllForUser, PruneExpired (startup). Session tokens now carry a `jti`; `AuthMiddleware.SetSessionValidator` (optional — nil keeps tests/old behavior) rejects revoked/missing jti and records activity. Login/Challenge create a session; Logout revokes the current one (parses the cookie token directly, since logout isn't auth-wrapped); password change revokes all.
- Endpoints: `GET /api/auth/sessions` (own, current flagged), `POST /api/auth/sessions/revoke`. Frontend: "Active sessions" section in Settings — device/OS label, IP, last-seen, "this device" badge, sign-out per session.
- Tests: `services/sessions_test.go` (lifecycle, cross-user revoke denied, admin/revoke-all, persistence) + `middleware/auth_test.go` (validator: active allowed, revoked/missing-jti 401).
- Shared `saveJSONFile` helper added (companion to loadJSONFile).

### #7 — File versioning
- `VersionStore` (`<root>/.versions/<sha256(path)>/<nanotime>.bin`, retains newest 10). `Upload` snapshots an existing file before overwriting it. Restore snapshots the current file first (reversible). Version id is the nanotime — validated numeric to block path traversal.
- Endpoints: `GET /api/files/versions`, `GET /api/files/versions/download` (ServeContent), `POST /api/files/versions/restore`. All checkAccess-gated.
- Frontend: "Version history" context-menu item → `VersionsModal` (list with timestamps/sizes/"latest" badge, per-version download + restore-with-confirm; focus-trapped via useDialog).
- Tests: `services/versions_test.go` — save/list/restore round-trip (+ reversible snapshot), retention cap, bad-id (traversal) rejected.
- NOTE: versions keyed by path → a rename/move starts a fresh history (old versions orphaned under `.versions`); acceptable for v1.

### #4 — Admin user management
- Backend: `UserStore.ListUsers/CreateUser/UpdateUser/DeleteUser` with validation (unique username, ≥8-char password, role ∈ {admin,user}) and **last-admin protection** (can't demote/delete the only admin); password change bumps PwVersion (invalidates sessions). `AdminHandler` (every method re-checks `role==admin`) on `GET/POST/DELETE /api/admin/users` + `POST /api/admin/users/update`; can't delete your own account.
- Frontend: `UserManagement` component (admin-only section in SettingsModal) — list with role/MFA/quota badges, add-user form, inline edit (home/role/quota/password), delete with confirm. Quota shown/entered in MB/GB.
- Tests: `services/usercrud_test.go` — create (+dup/weak-pw/bad-role rejects), update bumps PwVersion, last-admin protection, delete.

### #6 — Per-user storage quotas
- Added `Quota int64` (bytes, 0=unlimited) to the User model + `UserStore.GetQuota`. `FileHandler.SetQuotaLookup` injects it (no constructor change); `Upload` rejects with 507 if `dirSize(home)+incoming > quota`. Only quota'd users pay the home-folder measurement cost. Caller's quota is also surfaced in `/api/disk`.
- Tests: `handlers/quota_test.go` — over-quota upload rejected (file not written), within-quota allowed, unset quota unlimited.
- NOTE: enforced on authenticated uploads; collaborate-share anonymous uploads don't yet check the owner's quota (documented follow-up).

### #1 — Thumbnail endpoint + cache
- `GET /api/files/thumbnail?path=` (auth-wrapped): decodes jpg/png/gif (stdlib), downscales to ≤256px longest side with a pure-Go area-averaging scaler, encodes JPEG q82, caches to `<root>/.thumbs/<sha256(path|size|mtime|dim)>.jpg`. Edited files regenerate (key includes size+mtime). Types stdlib can't decode (webp/svg/bmp) or that fail to decode fall back to streaming the original via ServeContent — no regression. `.thumbs` is a dotdir, excluded from listing/search/recent/disk.
- Frontend: `getThumbnailUrl()`; FileExplorer list+grid image tiles now load thumbnails instead of full originals (preview modal still uses full-res). The existing onError → FileIcon fallback covers any failure.
- Tests: `handlers/thumbnail_test.go` — 1000×500 → 256×128 JPEG + on-disk cache reused; small image not upscaled.

### #2 — HTTP Range support
- `Download` (file branch) and `Preview` now use `http.ServeContent` instead of `io.Copy`, adding `Range`/`If-Range`/conditional-request handling, `Accept-Ranges`, and correct `Content-Length`. Video/audio previews can seek; interrupted downloads resume.
- Test: `handlers/range_test.go` — full request advertises `Accept-Ranges: bytes`; `Range: bytes=2-5` returns 206 with exactly those bytes + `Content-Range`.
