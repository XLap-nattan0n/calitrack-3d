// Applies supabase/migrations to an in-memory Postgres (PGlite) with Supabase stubs
// (roles, auth.users, auth.uid()) and runs supabase/tests/rls.sql.
// Usage: npm i --no-save @electric-sql/pglite && node supabase/scripts/test-pglite.mjs
// Authoritative check is still `supabase db reset` + psql on the Supabase CLI local stack.
import { PGlite } from '@electric-sql/pglite';
import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const db = new PGlite();

await db.exec(`
  create role anon nologin; create role authenticated nologin;
  grant usage on schema public to anon, authenticated;
  create schema auth;
  create table auth.users (id uuid primary key, email text, raw_user_meta_data jsonb);
  create function auth.uid() returns uuid language sql stable as $$
    select coalesce(nullif(current_setting('request.jwt.claim.sub', true), ''),
           (nullif(current_setting('request.jwt.claims', true), '')::jsonb ->> 'sub'))::uuid $$;
  grant usage on schema auth to anon, authenticated;
  -- Supabase default privileges: new public tables are granted to anon/authenticated
  alter default privileges in schema public grant all on tables to anon, authenticated;
`);

const dir = join(root, 'migrations');
for (const f of readdirSync(dir).filter((f) => f.endsWith('.sql')).sort()) {
  await db.exec(readFileSync(join(dir, f), 'utf8'));
  console.log('applied', f);
}

// destructive-statement guard
for (const f of readdirSync(dir)) {
  const sql = readFileSync(join(dir, f), 'utf8').replace(/--.*$/gm, '');
  if (/\b(drop\s+(table|schema|column)|truncate|delete\s+from)\b/i.test(sql))
    throw new Error(`destructive statement in ${f}`);
}

const res = await db.exec(readFileSync(join(root, 'tests/rls.sql'), 'utf8'));
console.log(res.at(-1).rows[0].result);
