-- Row Level Security. Reference tables: read-only for authenticated users
-- (writes only via migrations). User tables: owner-only (user_id = auth.uid()).

-- No anonymous access to anything; explicit grants only.
revoke all on all tables in schema public from anon;

-- ---- reference tables --------------------------------------------------
do $$
declare t text;
begin
  foreach t in array array[
    'equipment', 'muscle_groups', 'muscles', 'exercises',
    'exercise_equipment', 'exercise_muscles', 'exercise_progressions'
  ] loop
    execute format('alter table public.%I enable row level security', t);
    execute format('revoke all on public.%I from authenticated', t);
    execute format('grant select on public.%I to authenticated', t);
    execute format(
      'create policy %I on public.%I for select to authenticated using (true)',
      t || '_read', t);
  end loop;
end $$;

-- ---- profiles ----------------------------------------------------------
-- Rows are created by the auth.users trigger (security definer), so no INSERT/DELETE for users.
alter table public.profiles enable row level security;
revoke all on public.profiles from authenticated;
grant select, update (display_name, locale) on public.profiles to authenticated;
create policy profiles_select_own on public.profiles
  for select to authenticated using (id = auth.uid());
create policy profiles_update_own on public.profiles
  for update to authenticated using (id = auth.uid()) with check (id = auth.uid());

-- ---- user-owned tables with user_id -----------------------------------
do $$
declare t text;
begin
  foreach t in array array[
    'user_equipment', 'workout_plans', 'workout_sessions',
    'session_exercises', 'session_sets'
  ] loop
    execute format('alter table public.%I enable row level security', t);
    execute format('revoke all on public.%I from authenticated', t);
    execute format('grant select, insert, update, delete on public.%I to authenticated', t);
    execute format(
      'create policy %I on public.%I for all to authenticated using (user_id = auth.uid()) with check (user_id = auth.uid())',
      t || '_owner', t);
  end loop;
end $$;

-- ---- plan_exercises: ownership via parent plan ------------------------
alter table public.plan_exercises enable row level security;
revoke all on public.plan_exercises from authenticated;
grant select, insert, update, delete on public.plan_exercises to authenticated;
create policy plan_exercises_owner on public.plan_exercises
  for all to authenticated
  using (exists (select 1 from public.workout_plans p
                 where p.id = plan_id and p.user_id = auth.uid()))
  with check (exists (select 1 from public.workout_plans p
                      where p.id = plan_id and p.user_id = auth.uid()));
