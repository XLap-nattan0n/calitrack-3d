# Backend (Go + Chi + pgx on Vercel)

The Go server lives in `cmd/api`. Routes are defined in `internal/httpapi`. Vercel runs `cmd/api/main.go` as a Go server and listens on `$PORT`.

Layout: `internal/{config,apierr,auth,db,profile,httpapi}`. Handler → Service → Repository (pgx) per feature; `profile` is the first. Serves public `GET /api/v1/health` and Bearer-protected `GET/PATCH /api/v1/me`. Errors use `{"error":{"code","message","details"}}`.

Health: It runs `select 1` through a pgx pool on the Supabase pooler (port 6543, simple protocol) and returns `200 {"status":"ok","db":"ok"}`, or `503` if the database check fails.

## Environment
- `DATABASE_URL`: Supabase pooler URL on port 6543. Store it as a Vercel env var (Preview only) or a Paperclip secret. Never commit it. The password must be percent-encoded and must not be wrapped in `[...]`.
- `SUPABASE_JWT_ISSUER`: required, e.g. `https://<ref>.supabase.co/auth/v1`.
- `SUPABASE_JWT_SECRET` (HS256) and/or `SUPABASE_JWKS_URL` (RS256/ES256): at least one. Without them protected routes return 401.
- `SUPABASE_JWT_AUDIENCE`: defaults to `authenticated`.
- `PORT`: set by Vercel. Defaults to `8080` locally.

## Vercel setup
- The backend needs its own Vercel project or Services setup with Root Directory `backend`. Do not change the frontend project's Root Directory.
- `backend/vercel.json` pins the region to `syd1`.
- Preview deployments only. Do not promote to production.

## Run and verify
- Local tests: `cd backend && go test ./... && go build ./...`
- Local server: `PORT=8080 DATABASE_URL=... go run ./cmd/api`, then `curl -i http://127.0.0.1:8080/api/v1/health`
- Preview: `curl -i https://<preview>/api/v1/health` must return `200 {"status":"ok","db":"ok"}`. A `503` means the database check failed. Check the function logs for the reason.

## Rollback
- Revert the commit that changed `backend/` on the branch, or deploy the previous commit. The frontend project is not affected.
