-- RLS / trigger / seed assertions. Plain SQL: run with psql against a migrated DB
-- (supabase db reset && psql "$DB_URL" -v ON_ERROR_STOP=1 -f supabase/tests/rls.sql)
-- or via scripts/test-pglite.mjs. Everything runs in a transaction and is rolled back.
begin;

-- helper: act as an authenticated user
create function pg_temp.act_as(uid uuid) returns void language plpgsql as $$
begin
  perform set_config('request.jwt.claim.sub', uid::text, true);
  perform set_config('request.jwt.claims', json_build_object('sub', uid, 'role', 'authenticated')::text, true);
  execute 'set local role authenticated';
end $$;
create function pg_temp.assert(cond boolean, msg text) returns void language plpgsql as $$
begin if not coalesce(cond, false) then raise exception 'ASSERT FAILED: %', msg; end if; end $$;
grant execute on function pg_temp.act_as(uuid), pg_temp.assert(boolean, text) to public;

-- 1. trigger creates profiles with locale default 'en'
insert into auth.users (id, email) values
  ('aaaaaaaa-0000-0000-0000-000000000001', 'a@example.com'),
  ('bbbbbbbb-0000-0000-0000-000000000002', 'b@example.com');
select pg_temp.assert((select count(*) from public.profiles
  where id in ('aaaaaaaa-0000-0000-0000-000000000001','bbbbbbbb-0000-0000-0000-000000000002')) = 2,
  'profiles trigger created 2 rows');
select pg_temp.assert((select locale from public.profiles where id = 'aaaaaaaa-0000-0000-0000-000000000001') = 'en',
  'locale default en');
select pg_temp.assert((select display_name from public.profiles where id = 'aaaaaaaa-0000-0000-0000-000000000001') = 'a',
  'display_name from email');

-- 2. seed present
select pg_temp.assert((select count(*) from public.equipment) = 4, '4 equipment');
select pg_temp.assert((select count(*) from public.muscle_groups) = 6, '6 muscle groups');
select pg_temp.assert((select count(*) from public.muscles where mesh_name is not null) >= 20, 'muscles seeded');

-- fixture data for user A and user B (as superuser, bypasses RLS)
insert into public.exercises (id, slug, name_en, category, difficulty, exercise_type)
  values ('eeeeeeee-0000-0000-0000-000000000001', 'rls-fixture-exercise', 'RLS fixture', 'push', 1, 'reps');
insert into public.workout_plans (id, user_id, plan_date, title, category) values
  ('11111111-0000-0000-0000-00000000000a', 'aaaaaaaa-0000-0000-0000-000000000001', current_date, 'A plan', 'push'),
  ('11111111-0000-0000-0000-00000000000b', 'bbbbbbbb-0000-0000-0000-000000000002', current_date, 'B plan', 'push');
insert into public.plan_exercises (plan_id, exercise_id, position) values
  ('11111111-0000-0000-0000-00000000000a', 'eeeeeeee-0000-0000-0000-000000000001', 0),
  ('11111111-0000-0000-0000-00000000000b', 'eeeeeeee-0000-0000-0000-000000000001', 0);
insert into public.workout_sessions (id, user_id, plan_id) values
  ('22222222-0000-0000-0000-00000000000a', 'aaaaaaaa-0000-0000-0000-000000000001', '11111111-0000-0000-0000-00000000000a'),
  ('22222222-0000-0000-0000-00000000000b', 'bbbbbbbb-0000-0000-0000-000000000002', '11111111-0000-0000-0000-00000000000b');
insert into public.session_exercises (id, session_id, user_id, exercise_id, position) values
  ('33333333-0000-0000-0000-00000000000a', '22222222-0000-0000-0000-00000000000a', 'aaaaaaaa-0000-0000-0000-000000000001', 'eeeeeeee-0000-0000-0000-000000000001', 0),
  ('33333333-0000-0000-0000-00000000000b', '22222222-0000-0000-0000-00000000000b', 'bbbbbbbb-0000-0000-0000-000000000002', 'eeeeeeee-0000-0000-0000-000000000001', 0);
insert into public.session_sets (session_exercise_id, user_id, set_number) values
  ('33333333-0000-0000-0000-00000000000a', 'aaaaaaaa-0000-0000-0000-000000000001', 1),
  ('33333333-0000-0000-0000-00000000000b', 'bbbbbbbb-0000-0000-0000-000000000002', 1);
insert into public.user_equipment (user_id, equipment_id)
  select 'bbbbbbbb-0000-0000-0000-000000000002', id from public.equipment where code = 'pullup_bar';

-- 3. user A cannot read B's data (and sees own)
select pg_temp.act_as('aaaaaaaa-0000-0000-0000-000000000001');
select pg_temp.assert((select count(*) from public.profiles) = 1, 'A sees only own profile');
select pg_temp.assert((select count(*) from public.workout_plans) = 1, 'A sees 1 plan');
select pg_temp.assert((select count(*) from public.workout_plans where user_id = 'bbbbbbbb-0000-0000-0000-000000000002') = 0, 'A cannot read B plan');
select pg_temp.assert((select count(*) from public.plan_exercises) = 1, 'A sees only own plan_exercises');
select pg_temp.assert((select count(*) from public.workout_sessions) = 1, 'A sees only own session');
select pg_temp.assert((select count(*) from public.session_exercises) = 1, 'A sees only own session_exercises');
select pg_temp.assert((select count(*) from public.session_sets) = 1, 'A sees only own sets');
select pg_temp.assert((select count(*) from public.user_equipment) = 0, 'A cannot see B user_equipment');
select pg_temp.assert((select count(*) from public.exercises where slug = 'rls-fixture-exercise') = 1, 'A can read reference exercises');
select pg_temp.assert((select count(*) from public.muscles) >= 20, 'A can read reference muscles');

-- 4. A cannot modify B's rows (silently 0 rows affected) ...
do $$ declare n int; begin
  update public.workout_plans set title = 'hacked' where id = '11111111-0000-0000-0000-00000000000b';
  get diagnostics n = row_count; perform pg_temp.assert(n = 0, 'A cannot update B plan');
  delete from public.workout_sessions where id = '22222222-0000-0000-0000-00000000000b';
  get diagnostics n = row_count; perform pg_temp.assert(n = 0, 'A cannot delete B session');
  update public.profiles set display_name = 'hacked' where id = 'bbbbbbbb-0000-0000-0000-000000000002';
  get diagnostics n = row_count; perform pg_temp.assert(n = 0, 'A cannot update B profile');
  update public.session_sets set completed = true where user_id = 'bbbbbbbb-0000-0000-0000-000000000002';
  get diagnostics n = row_count; perform pg_temp.assert(n = 0, 'A cannot update B sets');
end $$;

-- ... and cannot insert rows for B / into B's parents
do $$ begin
  begin
    insert into public.workout_plans (user_id, plan_date, title, category)
      values ('bbbbbbbb-0000-0000-0000-000000000002', current_date, 'x', 'push');
    raise exception 'ASSERT FAILED: A inserted plan for B';
  exception when insufficient_privilege then null; end;
  begin
    insert into public.plan_exercises (plan_id, exercise_id, position)
      values ('11111111-0000-0000-0000-00000000000b', 'eeeeeeee-0000-0000-0000-000000000001', 5);
    raise exception 'ASSERT FAILED: A inserted plan_exercise into B plan';
  exception when insufficient_privilege then null; end;
  begin
    insert into public.workout_sessions (user_id, plan_id)
      values ('aaaaaaaa-0000-0000-0000-000000000001', '11111111-0000-0000-0000-00000000000b');
    raise exception 'ASSERT FAILED: A created session pointing at B plan';
  exception when foreign_key_violation then null; end;
  begin
    insert into public.session_sets (session_exercise_id, user_id, set_number)
      values ('33333333-0000-0000-0000-00000000000b', 'aaaaaaaa-0000-0000-0000-000000000001', 9);
    raise exception 'ASSERT FAILED: A inserted set under B session_exercise';
  exception when foreign_key_violation then null; end;
end $$;

-- 5. reference tables are not writable by users; anon has no access
do $$ begin
  begin
    insert into public.equipment (code, name_en) values ('x', 'X');
    raise exception 'ASSERT FAILED: user wrote reference table';
  exception when insufficient_privilege then null; end;
  begin
    insert into public.profiles (id) values (gen_random_uuid());
    raise exception 'ASSERT FAILED: user inserted profile';
  exception when insufficient_privilege then null; end;
end $$;

-- A can write own data
insert into public.user_equipment (user_id, equipment_id)
  select 'aaaaaaaa-0000-0000-0000-000000000001', id from public.equipment where code = 'bodyweight';
select pg_temp.assert((select count(*) from public.user_equipment) = 1, 'A can write own equipment');

reset role;
select pg_temp.act_as('bbbbbbbb-0000-0000-0000-000000000002');
select pg_temp.assert((select count(*) from public.user_equipment) = 1, 'B sees only own equipment');
select pg_temp.assert((select count(*) from public.workout_plans where title = 'hacked') = 0, 'B data untouched');
reset role;

-- anon sees nothing
set local role anon;
do $$ begin
  begin perform 1 from public.workout_plans; raise exception 'ASSERT FAILED: anon read plans';
  exception when insufficient_privilege then null; end;
  begin perform 1 from public.exercises; raise exception 'ASSERT FAILED: anon read exercises';
  exception when insufficient_privilege then null; end;
end $$;
reset role;

-- 6. deleting a plan keeps the session (plan_id nulled, user_id intact)
delete from public.workout_plans where id = '11111111-0000-0000-0000-00000000000a';
select pg_temp.assert((select plan_id is null and user_id is not null from public.workout_sessions
  where id = '22222222-0000-0000-0000-00000000000a'), 'plan delete keeps session');

rollback;
select 'RLS TESTS PASSED' as result;
