# Deployment

Scope: Vercel **preview** deployments of the frontend and the Go API, plus the Supabase project they use. Production deploys need board approval and are not covered by this runbook.

## Components

| Component | Where it runs | Source |
|---|---|---|
| Frontend (Next.js 15) | Vercel project `calitrack-3d`, Root Directory `frontend` | `frontend/` |
| Go API | Vercel project `calitrack-3d-api`, Root Directory `backend` | `backend/`, region `syd1` (`backend/vercel.json`) |
| Database and auth | Supabase project (`calitrack-dev` for development) | `supabase/migrations/` |
| CI | GitHub Actions, `.github/workflows/ci.yml` | backend, frontend, migrations, supabase-db, secret-scan |

The browser calls same-origin `/api/v1/*`. `frontend/next.config.mjs` rewrites those requests server-side to `API_PROXY_TARGET`, so the backend needs no CORS.

## Environment variables

Set these in the Vercel dashboard, scoped to **Preview** only. Never put real values in git. Values are given to the person who manages Vercel through a secure channel.

Frontend project (`calitrack-3d`):

| Variable | Value |
|---|---|
| `NEXT_PUBLIC_SUPABASE_URL` | Supabase project URL |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | Supabase anon (public) key |
| `API_PROXY_TARGET` | Origin of the API preview deployment, e.g. `https://calitrack-3d-api-git-<branch>-<team>.vercel.app` |
| `NEXT_PUBLIC_MOCK_PLANS` | `false` (the Plans API is merged) |

API project (`calitrack-3d-api`):

| Variable | Value |
|---|---|
| `DATABASE_URL` | Supabase pooler URL on port 6543. Percent-encode the password; do not wrap it in `[...]` |
| `SUPABASE_JWT_ISSUER` | Required, e.g. `https://<ref>.supabase.co/auth/v1` |
| `SUPABASE_JWT_SECRET` and/or `SUPABASE_JWKS_URL` | At least one is required; without them protected routes return 401 |
| `SUPABASE_JWT_AUDIENCE` | Optional, defaults to `authenticated` |
| `PORT` | Set by Vercel |

Local templates: `.env.example` (backend) and `frontend/.env.example` (frontend). Both are git-ignored once copied to `.env` or `.env.local`.

## Deploy a preview

1. Open a PR into `main` from a feature branch. Never push directly to `main`.
2. CI must be green: backend, frontend, migrations, supabase-db, secret-scan.
3. Vercel builds a preview for each project and posts the preview URLs in the PR comment.
4. Confirm `GET <API preview>/api/v1/health` returns `200 {"status":"ok","db":"ok"}`. A `503` means the database check failed; check the function logs.
5. Run the smoke test (below) against the preview.

## Database migrations

- Migrations are additive. Each file is `supabase/migrations/<14-digit timestamp>_<snake_name>.sql`. `scripts/lint-migrations.sh` enforces the naming and rejects duplicates and empty files.
- Local check: `supabase start && supabase db reset`, then the seed and RLS checks in `docs/local-supabase.md`. The `supabase-db` CI job runs the same steps.
- Applying to a remote project (`calitrack-dev` or any other) is **not** part of a preview deploy. Do it only after the board approves it in the task, and then run `supabase db push` against the linked project with the approved credentials.
- Do not edit an applied migration. Add a new file with a later timestamp.

## Smoke test

`scripts/smoke-test.sh` checks the API health and anonymous rejection, the frontend pages, the 3D model asset, the `/api/v1` rewrite, and, when credentials are supplied, the authenticated flow: me, exercises, planner, session, analytics, records.

Run it:

```bash
API_BASE_URL=https://<api-preview> \
WEB_BASE_URL=https://<frontend-preview> \
VERCEL_PROTECTION_BYPASS=<secret, only if previews are protected> \
SUPABASE_URL=https://<ref>.supabase.co \
SUPABASE_ANON_KEY=<anon key> \
SMOKE_EMAIL=<dedicated test user> \
SMOKE_PASSWORD=<test user password> \
scripts/smoke-test.sh
```

- Needs `curl` and `jq`. Exit code 1 means at least one check failed. Skipped checks are listed as `SKIP` with the missing variables.
- Use a dedicated test user, not a personal account. The authenticated checks create one plan and start and finish one session, then delete the plan. The finished session stays in the history because the API has no delete endpoint.
- A `302` with "Protected by Vercel Authentication" means the preview is behind Vercel deployment protection. The script reports this as a failure, not a pass.

## Rollback

- **Preview:** revert the offending commit on the PR branch, or redeploy the previous commit. The frontend and API projects are independent, so you can roll back one without touching the other.
- **Database:** add a forward-fix migration. Do not drop or rewrite applied migrations.
- **Production:** not covered here. Promotion and rollback of production deployments need board approval.

## Known blockers

- Vercel preview deployments are protected by Vercel Authentication. Anonymous requests return `302`, so the smoke test cannot run unauthenticated. The board needs to choose one of these: disable protection for these preview projects, or provide an automation bypass secret through a secure channel.
- If the API preview stays protected, the frontend's server-side rewrite to `API_PROXY_TARGET` will also be blocked. Confirm this once the protection decision is made.
- Vercel env vars for the preview are not set by this runbook. The board or an approved owner sets them.
