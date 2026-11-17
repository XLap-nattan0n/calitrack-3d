# Running the Supabase database locally

Requires Docker and the Supabase CLI.

## Start, migrate, and check

```bash
supabase start
supabase db reset                      # applies every file in supabase/migrations/ and rebuilds the DB
DB_URL=$(supabase status -o json | jq -r .DB_URL)
psql "$DB_URL" -v ON_ERROR_STOP=1 -f supabase/tests/seed_exercises.sql
psql "$DB_URL" -v ON_ERROR_STOP=1 -f supabase/tests/rls.sql
```

`seed_exercises.sql` checks that there are at least 40 exercises and that each has a primary muscle and at least one equipment row. `rls.sql` checks row-level security and runs inside a transaction that is rolled back.

Stop the stack with `supabase stop`.

## Without Docker

```bash
npm i --no-save @electric-sql/pglite
node supabase/scripts/test-pglite.mjs
```

This runs the migrations and `rls.sql` on in-memory PGlite with stubbed `auth` objects. It does not replace the `supabase db reset` check, which is authoritative.

## CI

The `supabase-db` job in `.github/workflows/ci.yml` runs the same steps on every pull request. A failing migration makes `supabase db reset` exit non-zero, which fails the job.

## Rollback

Migrations are additive. To return to a clean local state, run `supabase db reset` again. Do not edit an applied migration; add a new file with a later timestamp instead.
