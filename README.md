# CaliTrack 3D

Calisthenics workout tracking platform. All UI text is English.

## Layout
| Path | Owner | Contents |
|---|---|---|
| `frontend/` | Frontend Engineer | TypeScript web app |
| `backend/` | Backend Engineer | Go API |
| `supabase/` | Backend Engineer | SQL migrations (`migrations/<14-digit-timestamp>_name.sql`) |
| `docs/` | CTO | Architecture, ERD, API contract |
| `.github/` | DevOps | CI workflows |

## Rules
- Never commit secrets. Copy `.env.example` to `.env` locally (git-ignored).
- Work on feature branches and open a PR into `main`; CI must pass.
- Only edit the folder you own. Contract changes go through the CTO.
- No production deploys without board approval.

## CI
`.github/workflows/ci.yml`: backend (`go vet`, `go test`, `go build`), frontend (`tsc --noEmit`, lint, build), migration lint, secret scan (gitleaks).

## Deployment
Preview deploys, env vars, migrations, rollback, and the preview smoke test (`scripts/smoke-test.sh`): see [docs/deployment.md](docs/deployment.md).
