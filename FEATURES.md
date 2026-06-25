# CloudDrive Feature Build

Branch: `feat/enhancements` (off `loop/hardening`). Same verification gates as IMPROVEMENTS.md:
- Backend: `cd backend && GOOS=linux go build ./... && GOOS=linux go vet ./... && go test ./...`
- Frontend: `cd frontend && npm run build`

Goal: implement features #1–#9 from the recommendations, each polished + tested, one commit per feature. Never push master.

## Plan / status
1. **Thumbnail endpoint + cache** — fast image grids/lists (pure-Go scaler, cached). — TODO
2. **HTTP Range support** for download/preview — video seek + resumable. — ✅ DONE
3. **Chunked/resumable uploads** — large uploads survive drops. — TODO
4. **Admin user-management UI** — add/remove users, roles, home folders. — TODO
5. **WebDAV endpoint** — mount as a native drive (needs `x/net/webdav`). — TODO (dep check)
6. **Per-user storage quotas** — enforce caps on upload. — TODO
7. **File versioning** — keep previous copy on overwrite. — TODO
8. **Active-session management** — list + revoke signed-in devices. — TODO
9. **Full-text / content search** — search inside text/PDF. — TODO

## Done
### #2 — HTTP Range support
- `Download` (file branch) and `Preview` now use `http.ServeContent` instead of `io.Copy`, adding `Range`/`If-Range`/conditional-request handling, `Accept-Ranges`, and correct `Content-Length`. Video/audio previews can seek; interrupted downloads resume.
- Test: `handlers/range_test.go` — full request advertises `Accept-Ranges: bytes`; `Range: bytes=2-5` returns 206 with exactly those bytes + `Content-Range`.
