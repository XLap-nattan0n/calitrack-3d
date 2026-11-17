-- CaliTrack 3D — core schema (14 tables per ERD in NAT-53 plan rev 2).
-- Additive only: no DROP / TRUNCATE / DELETE statements.
-- name_th columns are nullable (future i18n, unused in MVP). UI language is English.

-- ---------------------------------------------------------------- profiles
create table public.profiles (
  id           uuid primary key references auth.users (id) on delete cascade,
  display_name text,
  locale       text not null default 'en',
  created_at   timestamptz not null default now()
);

-- --------------------------------------------------------- reference data
create table public.equipment (
  id      smallint generated always as identity primary key,
  code    text not null unique,
  name_th text,
  name_en text not null
);

create table public.muscle_groups (
  id      smallint generated always as identity primary key,
  code    text not null unique,
  name_th text,
  name_en text not null
);

create table public.muscles (
  id        smallint generated always as identity primary key,
  group_id  smallint not null references public.muscle_groups (id),
  code      text not null unique,
  name_th   text,
  name_en   text not null,
  mesh_name text            -- mesh name inside the anatomy GLB
);
create index muscles_group_id_idx on public.muscles (group_id);

create table public.exercises (
  id               uuid primary key default gen_random_uuid(),
  slug             text not null unique,
  name_th          text,
  name_en          text not null,
  description      text,
  category         text not null
    check (category in ('push', 'pull', 'legs', 'core', 'full_body', 'skills')),
  difficulty       smallint not null check (difficulty between 1 and 5),
  exercise_type    text not null check (exercise_type in ('reps', 'hold')),
  rec_sets         int check (rec_sets > 0),
  rec_reps_min     int check (rec_reps_min > 0),
  rec_reps_max     int check (rec_reps_max > 0),
  rec_hold_seconds int check (rec_hold_seconds > 0),
  instructions     text,
  safety_notes     text,
  check (rec_reps_min is null or rec_reps_max is null or rec_reps_min <= rec_reps_max)
);
create index exercises_category_idx   on public.exercises (category);
create index exercises_difficulty_idx on public.exercises (difficulty);

create table public.exercise_equipment (
  exercise_id  uuid     not null references public.exercises (id) on delete cascade,
  equipment_id smallint not null references public.equipment (id),
  primary key (exercise_id, equipment_id)
);
create index exercise_equipment_equipment_id_idx on public.exercise_equipment (equipment_id);

create table public.exercise_muscles (
  exercise_id uuid     not null references public.exercises (id) on delete cascade,
  muscle_id   smallint not null references public.muscles (id),
  role        text     not null check (role in ('primary', 'secondary')),
  primary key (exercise_id, muscle_id)
);
create index exercise_muscles_muscle_id_idx on public.exercise_muscles (muscle_id);

create table public.exercise_progressions (
  exercise_id      uuid not null references public.exercises (id) on delete cascade,
  next_exercise_id uuid not null references public.exercises (id) on delete cascade,
  relation         text not null check (relation in ('easier', 'harder')),
  primary key (exercise_id, next_exercise_id),
  check (exercise_id <> next_exercise_id)
);
create index exercise_progressions_next_idx on public.exercise_progressions (next_exercise_id);

-- ---------------------------------------------------------- user-owned data
create table public.user_equipment (
  user_id      uuid     not null references public.profiles (id) on delete cascade,
  equipment_id smallint not null references public.equipment (id),
  primary key (user_id, equipment_id)
);
create index user_equipment_equipment_id_idx on public.user_equipment (equipment_id);

create table public.workout_plans (
  id                      uuid primary key default gen_random_uuid(),
  user_id                 uuid not null references public.profiles (id) on delete cascade,
  plan_date               date not null,
  title                   text not null,
  category                text not null
    check (category in ('push', 'pull', 'legs', 'core', 'full_body', 'skills', 'custom')),
  target_muscle_group_ids smallint[] not null default '{}',
  status                  text not null default 'planned'
    check (status in ('planned', 'completed', 'skipped')),
  unique (id, user_id)    -- target for same-owner composite FKs
);
create index workout_plans_user_date_idx on public.workout_plans (user_id, plan_date);

create table public.plan_exercises (
  id           uuid primary key default gen_random_uuid(),
  plan_id      uuid not null references public.workout_plans (id) on delete cascade,
  exercise_id  uuid not null references public.exercises (id),
  position     int  not null check (position >= 0),
  target_sets  int  check (target_sets > 0),
  target_reps  int  check (target_reps > 0),
  hold_seconds int  check (hold_seconds > 0),
  rest_seconds int  check (rest_seconds >= 0),
  assist_band  text,
  unique (plan_id, position) deferrable initially deferred
);
create index plan_exercises_exercise_id_idx on public.plan_exercises (exercise_id);

create table public.workout_sessions (
  id          uuid primary key default gen_random_uuid(),
  user_id     uuid not null references public.profiles (id) on delete cascade,
  plan_id     uuid,
  started_at  timestamptz not null default now(),
  finished_at timestamptz,
  status      text not null default 'in_progress'
    check (status in ('in_progress', 'completed', 'abandoned')),
  notes       text,
  unique (id, user_id),
  -- plan must belong to the same user; deleting a plan keeps the session
  foreign key (plan_id, user_id) references public.workout_plans (id, user_id)
    on delete set null (plan_id),
  check (finished_at is null or finished_at >= started_at)
);
create index workout_sessions_user_started_idx on public.workout_sessions (user_id, started_at);
create index workout_sessions_active_idx on public.workout_sessions (user_id) where status = 'in_progress';
create index workout_sessions_plan_id_idx on public.workout_sessions (plan_id);

-- user_id is denormalized on session_* for RLS / analytics; composite FKs keep it consistent.
create table public.session_exercises (
  id          uuid primary key default gen_random_uuid(),
  session_id  uuid not null,
  user_id     uuid not null references public.profiles (id) on delete cascade,
  exercise_id uuid not null references public.exercises (id),
  position    int  not null check (position >= 0),
  unique (id, user_id),
  unique (session_id, position) deferrable initially deferred,
  foreign key (session_id, user_id) references public.workout_sessions (id, user_id)
    on delete cascade
);
create index session_exercises_exercise_id_idx on public.session_exercises (exercise_id);
create index session_exercises_user_id_idx on public.session_exercises (user_id);

create table public.session_sets (
  id                  uuid primary key default gen_random_uuid(),
  session_exercise_id uuid not null,
  user_id             uuid not null references public.profiles (id) on delete cascade,
  set_number          int  not null check (set_number > 0),
  target_reps         int  check (target_reps > 0),
  actual_reps         int  check (actual_reps >= 0),
  duration_seconds    int  check (duration_seconds >= 0),
  completed           boolean not null default false,
  assist_band         text,
  notes               text,
  unique (session_exercise_id, set_number) deferrable initially deferred,
  foreign key (session_exercise_id, user_id) references public.session_exercises (id, user_id)
    on delete cascade
);
create index session_sets_user_id_idx on public.session_sets (user_id);
