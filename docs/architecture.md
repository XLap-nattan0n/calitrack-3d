# CaliTrack 3D — Architecture, Auth & ERD

Approved plan: NAT-53 (rev 2). UI language is **English**; `name_th` columns are nullable and unused in the MVP.
The API contract is [`openapi.yaml`](./openapi.yaml) — the single source of truth for FE/BE. Contract changes go through the CTO.

## 1. System

```mermaid
flowchart LR
  user(["User (browser/mobile)"])
  fe["Next.js App Router (Vercel)"]
  api["Go REST API /api/v1 (Vercel Go Function)"]
  sauth(["Supabase Auth"])
  db[("Supabase Postgres")]
  stor[("Supabase Storage: GLB model, images")]
  user --> fe
  fe -->|"login/signup, session"| sauth
  fe -->|"fetch + Bearer JWT"| api
  api -->|"verify JWT (JWKS)"| sauth
  api -->|"pgx, user_id scoped"| db
  fe -->|"load 3D model"| stor
```

Conventions: Handler → Service → Repository (pgx), Chi router, `userID` from context on every user-data query, one error format, FE and API on the same Vercel domain (rewrite `/api/v1/*`).

### Vercel Go Functions
- Go runs as a serverless function, not a persistent process. One function `backend/api/index.go` builds the Chi router (cached in a global) and `vercel.json` rewrites `/api/v1/(.*)` to it. Local dev: `cmd/server/main.go` serves the same router.
- DB via Supabase pooler (transaction mode, port 6543): small pgxpool, `default_query_exec_mode=simple_protocol` (no prepared statements).
- Spike S0 must validate this (incl. cold start) before the backend scaffold is built. If it fails, report to the board; do not change stack unilaterally.

## 2. Authentication

```mermaid
sequenceDiagram
  participant B as Browser (Next.js)
  participant SA as Supabase Auth
  participant G as Go API
  participant D as Postgres
  B->>SA: signUp / signInWithPassword (@supabase/ssr cookie)
  SA-->>B: access_token (JWT) + refresh
  B->>G: Authorization: Bearer JWT
  G->>SA: JWKS (cached): verify signature, exp, iss, aud=authenticated
  G->>D: query WHERE user_id = sub
  D-->>G: rows
  G-->>B: JSON
```

- Go verifies every JWT (ES256/RS256 via JWKS, or HS256 if the project is legacy) and uses `sub` as `user_id`.
- No service-role key in FE or Go code; Go uses a least-privilege DB role. Secrets only in Vercel env, never in git.
- Next.js middleware protects `/(app)/*`, redirecting to `/login`.
- RLS on every user-data table (`user_id = auth.uid()`) as defense-in-depth; reference tables readable by authenticated users, writable only via migrations.
- `profiles` row is created by a trigger on `auth.users`.
- Other users' resources return 404.

## 3. ERD

```mermaid
erDiagram
  PROFILES ||--o{ WORKOUT_PLANS : owns
  PROFILES ||--o{ WORKOUT_SESSIONS : performs
  PROFILES ||--o{ USER_EQUIPMENT : has
  EQUIPMENT ||--o{ USER_EQUIPMENT : selected_in
  EQUIPMENT ||--o{ EXERCISE_EQUIPMENT : required_by
  EXERCISES ||--o{ EXERCISE_EQUIPMENT : needs
  EXERCISES ||--o{ EXERCISE_MUSCLES : targets
  MUSCLES ||--o{ EXERCISE_MUSCLES : involved_in
  MUSCLE_GROUPS ||--o{ MUSCLES : contains
  EXERCISES ||--o{ EXERCISE_PROGRESSIONS : progresses
  WORKOUT_PLANS ||--o{ PLAN_EXERCISES : contains
  EXERCISES ||--o{ PLAN_EXERCISES : used_in
  WORKOUT_PLANS ||--o{ WORKOUT_SESSIONS : started_as
  WORKOUT_SESSIONS ||--o{ SESSION_EXERCISES : includes
  SESSION_EXERCISES ||--o{ SESSION_SETS : has
  EXERCISES ||--o{ SESSION_EXERCISES : performed_as
  PROFILES { uuid id PK "= auth.users.id" text display_name text locale "default en" timestamptz created_at }
  EQUIPMENT { smallint id PK text code UK text name_th "nullable" text name_en }
  USER_EQUIPMENT { uuid user_id FK smallint equipment_id FK }
  MUSCLE_GROUPS { smallint id PK text code UK text name_th "nullable" text name_en }
  MUSCLES { smallint id PK smallint group_id FK text code UK text name_th "nullable" text name_en text mesh_name "mesh name in GLB" }
  EXERCISES { uuid id PK text slug UK text name_th "nullable" text name_en text description text category smallint difficulty "1-5" text exercise_type "reps or hold" int rec_sets int rec_reps_min int rec_reps_max int rec_hold_seconds text instructions text safety_notes }
  EXERCISE_EQUIPMENT { uuid exercise_id FK smallint equipment_id FK }
  EXERCISE_MUSCLES { uuid exercise_id FK smallint muscle_id FK text role "primary or secondary" }
  EXERCISE_PROGRESSIONS { uuid exercise_id FK uuid next_exercise_id FK text relation "easier or harder" }
  WORKOUT_PLANS { uuid id PK uuid user_id FK date plan_date text title text category "incl custom" smallint[] target_muscle_group_ids text status }
  PLAN_EXERCISES { uuid id PK uuid plan_id FK uuid exercise_id FK int position int target_sets int target_reps int hold_seconds int rest_seconds text assist_band }
  WORKOUT_SESSIONS { uuid id PK uuid user_id FK uuid plan_id FK "nullable" timestamptz started_at timestamptz finished_at text status "in_progress completed abandoned" text notes }
  SESSION_EXERCISES { uuid id PK uuid session_id FK uuid exercise_id FK uuid user_id FK int position }
  SESSION_SETS { uuid id PK uuid session_exercise_id FK uuid user_id FK int set_number int target_reps int actual_reps int duration_seconds bool completed text assist_band text notes }
```

Notes:
- `session_*` tables are snapshots of what was actually recorded; later plan edits do not change history.
- `user_id` is denormalized onto `session_exercises` and `session_sets` for RLS, indexing and fast ownership checks.
- Personal records and analytics are computed from `session_sets` via SQL views/queries (no separate tables in the MVP).
- Indexes: `workout_plans(user_id, plan_date)`, `workout_sessions(user_id, started_at)`, `exercise_muscles(muscle_id)`, `session_exercises(exercise_id)`.
- All FKs declared; `ON DELETE CASCADE` only parent→child for user data. Every schema change is a migration in `supabase/migrations/`.

## 4. 3D ↔ data link
`muscles.mesh_name` maps to mesh names in the GLB; the viewer uses `muscle.code` as the shared key. Selecting an exercise highlights its primary (red) and secondary (orange) muscles; clicking a mesh calls `GET /exercises?muscle=<code>`.

## 5. Working rules
Feature branch + PR, CI green and CTO review before QA, one owner per folder (`frontend/ backend/ supabase/ .github/`), no direct pushes to `main`.
