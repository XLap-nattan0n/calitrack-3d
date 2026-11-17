# Vercel Go function spike (NAT-56)

`api/index.go` serves `GET /api/v1/health` through Chi, using a pgx pool on the Supabase pooler (port 6543, simple protocol). `backend/vercel.json` rewrites `/api/v1/*` to `/api/index` and pins the function region to `syd1`.

## Environment
- `DATABASE_URL`: Supabase pooler URL, port 6543, set as a Paperclip secret (never committed). The password must be percent-encoded and not wrapped in `[...]`.

## Vercel setup (Board)
- The existing Vercel project serves the frontend. Do not change its Root Directory.
- The backend preview needs a separate setup (a Services-style `vercel.json`, per the Board's decision on NAT-56). Until that lands, the `/api/v1/health` preview check is pending.
- Preview deployments only. Do not promote to production.

## Verify
- Local: `cd backend && go test ./... && go build ./...`
- Preview: `curl -i https://<preview>/api/v1/health` returns `200 {"status":"ok","db":"ok"}`. A `503` means the database check failed; the server log has the reason.

## Rollback
- Delete `backend/api/` and `backend/vercel.json`, then revert the `go.mod` and `go.sum` changes.
