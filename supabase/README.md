# supabase/

Migrations (additive only, no destructive statements): `schema` (14 tables) → `rls` → `profiles trigger` → `seed muscles/equipment`.

## Test
- Supabase CLI local: `supabase start && supabase db reset && psql "postgresql://postgres:postgres@127.0.0.1:54322/postgres" -v ON_ERROR_STOP=1 -f supabase/tests/rls.sql`
- No Docker: `npm i --no-save @electric-sql/pglite && node supabase/scripts/test-pglite.mjs` (PGlite with stubbed `auth` schema/roles).
