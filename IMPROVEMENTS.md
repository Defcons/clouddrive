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

## Open / found (not yet done)
- (none recorded yet — audit continues next iterations: trash, share, mfa, ratelimit handlers; frontend error/empty/loading states; large-dir List perf walks child dirs for itemCount on every entry.)
