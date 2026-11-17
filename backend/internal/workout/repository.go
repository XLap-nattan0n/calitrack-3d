package workout

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/db"
)

type PGRepository struct{ pool *db.Pool }

func NewPGRepository(p *db.Pool) *PGRepository { return &PGRepository{pool: p} }

// querier is satisfied by both the pool and a transaction.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (r *PGRepository) begin(ctx context.Context) (pgx.Tx, error) {
	pool, err := r.pool.Get(ctx)
	if err != nil {
		return nil, err
	}
	return pool.Begin(ctx)
}

func (r *PGRepository) q(ctx context.Context) (querier, error) {
	pool, err := r.pool.Get(ctx)
	if err != nil {
		return nil, err
	}
	return pool, nil
}

// ---------------------------------------------------------------- plans

const planCols = `p.id::text, to_char(p.plan_date, 'YYYY-MM-DD'), p.title, p.category,
  p.target_muscle_group_ids::int[], p.status`

func scanPlans(ctx context.Context, q querier, sql string, args ...any) ([]Plan, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plans := []Plan{}
	ids := []string{}
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.PlanDate, &p.Title, &p.Category, &p.TargetMuscleGroupIDs, &p.Status); err != nil {
			return nil, err
		}
		if p.TargetMuscleGroupIDs == nil {
			p.TargetMuscleGroupIDs = []int{}
		}
		p.Exercises = []PlanExercise{}
		plans = append(plans, p)
		ids = append(ids, p.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if len(plans) == 0 {
		return plans, nil
	}
	ex, err := q.Query(ctx, `select pe.plan_id::text, pe.id::text, pe.exercise_id::text, pe.position,
  pe.target_sets, pe.target_reps, pe.hold_seconds, pe.rest_seconds, pe.assist_band
  from public.plan_exercises pe where pe.plan_id = any($1::uuid[]) order by pe.plan_id, pe.position`, ids)
	if err != nil {
		return nil, err
	}
	defer ex.Close()
	byID := map[string]*Plan{}
	for i := range plans {
		byID[plans[i].ID] = &plans[i]
	}
	for ex.Next() {
		var planID string
		var e PlanExercise
		var sets *int
		if err := ex.Scan(&planID, &e.ID, &e.ExerciseID, &e.Position, &sets, &e.TargetReps, &e.HoldSeconds, &e.RestSeconds, &e.AssistBand); err != nil {
			return nil, err
		}
		if sets != nil {
			e.TargetSets = *sets
		}
		byID[planID].Exercises = append(byID[planID].Exercises, e)
	}
	return plans, ex.Err()
}

func getPlan(ctx context.Context, q querier, userID, id string) (Plan, error) {
	plans, err := scanPlans(ctx, q, `select `+planCols+` from public.workout_plans p where p.id = $1::uuid and p.user_id = $2::uuid`, id, userID)
	if err != nil {
		return Plan{}, err
	}
	if len(plans) == 0 {
		return Plan{}, ErrNotFound
	}
	return plans[0], nil
}

func (r *PGRepository) ListPlans(ctx context.Context, userID string, from, to *string) ([]Plan, error) {
	q, err := r.q(ctx)
	if err != nil {
		return nil, err
	}
	return scanPlans(ctx, q, `select `+planCols+` from public.workout_plans p
 where p.user_id = $1::uuid
   and ($2::date is null or p.plan_date >= $2::date)
   and ($3::date is null or p.plan_date <= $3::date)
 order by p.plan_date, p.title, p.id`, userID, from, to)
}

// checkReferences makes sure every referenced exercise / muscle group exists (otherwise 400, not 500).
func checkReferences(ctx context.Context, q querier, in PlanInput) error {
	ids := map[string]bool{}
	for _, e := range in.Exercises {
		ids[e.ExerciseID] = true
	}
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	var n int
	if err := q.QueryRow(ctx, `select count(*) from public.exercises where id = any($1::uuid[])`, list).Scan(&n); err != nil {
		return err
	}
	if n != len(list) {
		return &ErrUnknownReference{Field: "exercises"}
	}
	if len(in.TargetMuscleGroupIDs) > 0 {
		if err := q.QueryRow(ctx, `select count(*) from public.muscle_groups where id = any($1::int[]::smallint[])`, in.TargetMuscleGroupIDs).Scan(&n); err != nil {
			return err
		}
		if n != len(in.TargetMuscleGroupIDs) {
			return &ErrUnknownReference{Field: "target_muscle_group_ids"}
		}
	}
	return nil
}

func insertPlanExercises(ctx context.Context, tx pgx.Tx, planID string, in PlanInput) error {
	if len(in.Exercises) == 0 {
		return nil
	}
	b := &pgx.Batch{}
	for i, e := range in.Exercises {
		b.Queue(`insert into public.plan_exercises
  (plan_id, exercise_id, position, target_sets, target_reps, hold_seconds, rest_seconds, assist_band)
  values ($1::uuid, $2::uuid, $3::int, $4::int, $5::int, $6::int, $7::int, $8::text)`,
			planID, e.ExerciseID, i, e.TargetSets, e.TargetReps, e.HoldSeconds, e.RestSeconds, e.AssistBand)
	}
	res := tx.SendBatch(ctx, b)
	for range in.Exercises {
		if _, err := res.Exec(); err != nil {
			_ = res.Close()
			return err
		}
	}
	return res.Close()
}

func (r *PGRepository) CreatePlan(ctx context.Context, userID string, in PlanInput) (Plan, error) {
	tx, err := r.begin(ctx)
	if err != nil {
		return Plan{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := checkReferences(ctx, tx, in); err != nil {
		return Plan{}, err
	}
	var id string
	if err := tx.QueryRow(ctx, `insert into public.workout_plans (user_id, plan_date, title, category, target_muscle_group_ids)
  values ($1::uuid, $2::date, $3, $4, $5::int[]::smallint[]) returning id::text`,
		userID, in.PlanDate, in.Title, in.Category, in.TargetMuscleGroupIDs).Scan(&id); err != nil {
		return Plan{}, err
	}
	if err := insertPlanExercises(ctx, tx, id, in); err != nil {
		return Plan{}, err
	}
	p, err := getPlan(ctx, tx, userID, id)
	if err != nil {
		return Plan{}, err
	}
	return p, tx.Commit(ctx)
}

func (r *PGRepository) GetPlan(ctx context.Context, userID, id string) (Plan, error) {
	q, err := r.q(ctx)
	if err != nil {
		return Plan{}, err
	}
	return getPlan(ctx, q, userID, id)
}

func (r *PGRepository) ReplacePlan(ctx context.Context, userID, id string, in PlanInput) (Plan, error) {
	tx, err := r.begin(ctx)
	if err != nil {
		return Plan{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var got string
	err = tx.QueryRow(ctx, `update public.workout_plans
   set plan_date = $3::date, title = $4, category = $5, target_muscle_group_ids = $6::int[]::smallint[]
 where id = $1::uuid and user_id = $2::uuid returning id::text`,
		id, userID, in.PlanDate, in.Title, in.Category, in.TargetMuscleGroupIDs).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, ErrNotFound
	}
	if err != nil {
		return Plan{}, err
	}
	if err := checkReferences(ctx, tx, in); err != nil {
		return Plan{}, err
	}
	if _, err := tx.Exec(ctx, `delete from public.plan_exercises where plan_id = $1::uuid`, id); err != nil {
		return Plan{}, err
	}
	if err := insertPlanExercises(ctx, tx, id, in); err != nil {
		return Plan{}, err
	}
	p, err := getPlan(ctx, tx, userID, id)
	if err != nil {
		return Plan{}, err
	}
	return p, tx.Commit(ctx)
}

func (r *PGRepository) DeletePlan(ctx context.Context, userID, id string) error {
	q, err := r.q(ctx)
	if err != nil {
		return err
	}
	tag, err := q.Exec(ctx, `delete from public.workout_plans where id = $1::uuid and user_id = $2::uuid`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ------------------------------------------------------------- sessions

const progressCols = `
  (select count(*) from public.session_sets ss join public.session_exercises se on se.id = ss.session_exercise_id
    where se.session_id = s.id and se.user_id = s.user_id)::int,
  (select count(*) from public.session_sets ss join public.session_exercises se on se.id = ss.session_exercise_id
    where se.session_id = s.id and se.user_id = s.user_id and ss.completed)::int`

const summaryCols = `s.id::text, s.plan_id::text, s.status, s.started_at, s.finished_at,` + progressCols

func scanSummary(row pgx.Row, extra ...any) (SessionSummary, error) {
	var s SessionSummary
	var total, done int
	dst := append([]any{&s.ID, &s.PlanID, &s.Status, &s.StartedAt, &s.FinishedAt, &total, &done}, extra...)
	if err := row.Scan(dst...); err != nil {
		return s, err
	}
	s.ProgressPercent = Progress(done, total)
	return s, nil
}

func getSession(ctx context.Context, q querier, userID, where string, args ...any) (Session, error) {
	var s Session
	sum, err := scanSummary(q.QueryRow(ctx, `select `+summaryCols+`, s.notes from public.workout_sessions s
 where s.user_id = $1::uuid and `+where+` limit 1`, append([]any{userID}, args...)...), &s.Notes)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	s.SessionSummary = sum
	s.Exercises = []SessionExercise{}

	rows, err := q.Query(ctx, `select se.id::text, se.exercise_id::text, e.name_en, se.position
  from public.session_exercises se join public.exercises e on e.id = se.exercise_id
 where se.session_id = $1::uuid and se.user_id = $2::uuid order by se.position`, s.ID, userID)
	if err != nil {
		return Session{}, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var e SessionExercise
		if err := rows.Scan(&e.ID, &e.ExerciseID, &e.ExerciseName, &e.Position); err != nil {
			return Session{}, err
		}
		e.Sets = []SessionSet{}
		s.Exercises = append(s.Exercises, e)
		ids = append(ids, e.ID)
	}
	if err := rows.Err(); err != nil {
		return Session{}, err
	}
	rows.Close()
	if len(ids) == 0 {
		return s, nil
	}
	idx := map[string]int{}
	for i, e := range s.Exercises {
		idx[e.ID] = i
	}
	sets, err := q.Query(ctx, `select ss.session_exercise_id::text, `+setCols+`
  from public.session_sets ss where ss.user_id = $1::uuid and ss.session_exercise_id = any($2::uuid[])
 order by ss.session_exercise_id, ss.set_number`, userID, ids)
	if err != nil {
		return Session{}, err
	}
	defer sets.Close()
	for sets.Next() {
		var seID string
		var st SessionSet
		if err := sets.Scan(append([]any{&seID}, setDest(&st)...)...); err != nil {
			return Session{}, err
		}
		i := idx[seID]
		s.Exercises[i].Sets = append(s.Exercises[i].Sets, st)
	}
	return s, sets.Err()
}

const setCols = `ss.id::text, ss.set_number, ss.target_reps, ss.actual_reps, ss.duration_seconds, ss.completed, ss.assist_band, ss.notes`

func setDest(s *SessionSet) []any {
	return []any{&s.ID, &s.SetNumber, &s.TargetReps, &s.ActualReps, &s.DurationSeconds, &s.Completed, &s.AssistBand, &s.Notes}
}

func (r *PGRepository) StartSession(ctx context.Context, userID string, planID *string) (Session, error) {
	tx, err := r.begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if planID != nil {
		var one int
		err := tx.QueryRow(ctx, `select 1 from public.workout_plans where id = $1::uuid and user_id = $2::uuid for share`, *planID, userID).Scan(&one)
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		if err != nil {
			return Session{}, err
		}
	}
	var id string
	if err := tx.QueryRow(ctx, `insert into public.workout_sessions (user_id, plan_id) values ($1::uuid, $2::uuid) returning id::text`,
		userID, planID).Scan(&id); err != nil {
		return Session{}, err
	}
	if planID != nil {
		// Snapshot: copy exercises and expand target_sets into individual sets.
		if _, err := tx.Exec(ctx, `insert into public.session_exercises (session_id, user_id, exercise_id, position)
  select $1::uuid, $2::uuid, pe.exercise_id, pe.position from public.plan_exercises pe where pe.plan_id = $3::uuid`,
			id, userID, *planID); err != nil {
			return Session{}, err
		}
		if _, err := tx.Exec(ctx, `insert into public.session_sets (session_exercise_id, user_id, set_number, target_reps, assist_band)
  select se.id, se.user_id, g, pe.target_reps, pe.assist_band
    from public.session_exercises se
    join public.plan_exercises pe on pe.plan_id = $3::uuid and pe.position = se.position
   cross join generate_series(1, coalesce(pe.target_sets, 1)) g
   where se.session_id = $1::uuid and se.user_id = $2::uuid`,
			id, userID, *planID); err != nil {
			return Session{}, err
		}
	}
	s, err := getSession(ctx, tx, userID, `s.id = $2::uuid`, id)
	if err != nil {
		return Session{}, err
	}
	return s, tx.Commit(ctx)
}

func (r *PGRepository) ListSessions(ctx context.Context, userID string, f SessionFilter) ([]SessionSummary, error) {
	q, err := r.q(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, `select `+summaryCols+` from public.workout_sessions s
 where s.user_id = $1::uuid
   and ($2::text = '' or s.status = $2::text)
   and ($3::date is null or s.started_at >= ($3::date)::timestamp at time zone 'UTC')
   and ($4::date is null or s.started_at < (($4::date) + 1)::timestamp at time zone 'UTC')
   and ($5::timestamptz is null or (s.started_at, s.id) < ($5::timestamptz, $6::uuid))
 order by s.started_at desc, s.id desc
 limit $7::int`,
		userID, f.Status, f.From, f.To, f.AfterStarted, nullUUID(f.AfterID), f.Limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SessionSummary{}
	for rows.Next() {
		s, err := scanSummary(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func nullUUID(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (r *PGRepository) ActiveSession(ctx context.Context, userID string) (Session, error) {
	q, err := r.q(ctx)
	if err != nil {
		return Session{}, err
	}
	return getSession(ctx, q, userID, `s.status = 'in_progress' order by s.started_at desc`)
}

func (r *PGRepository) GetSession(ctx context.Context, userID, id string) (Session, error) {
	q, err := r.q(ctx)
	if err != nil {
		return Session{}, err
	}
	return getSession(ctx, q, userID, `s.id = $2::uuid`, id)
}

func (r *PGRepository) UpdateSet(ctx context.Context, userID, sessionID, setID string, p SetPatch) (SetResult, error) {
	tx, err := r.begin(ctx)
	if err != nil {
		return SetResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	err = tx.QueryRow(ctx, `select s.status from public.workout_sessions s
  join public.session_exercises se on se.session_id = s.id and se.user_id = s.user_id
  join public.session_sets ss on ss.session_exercise_id = se.id and ss.user_id = s.user_id
 where s.id = $1::uuid and s.user_id = $2::uuid and ss.id = $3::uuid
 for update of s`, sessionID, userID, setID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return SetResult{}, ErrNotFound
	}
	if err != nil {
		return SetResult{}, err
	}
	if status != StatusActive {
		return SetResult{}, ErrSessionClosed
	}

	args := []any{setID, userID}
	var sets []string
	add := func(col, typ string, v any) {
		args = append(args, v)
		sets = append(sets, col+" = $"+strconv.Itoa(len(args))+"::"+typ)
	}
	if p.ActualReps.Set {
		add("actual_reps", "int", p.ActualReps.Value)
	}
	if p.DurationSeconds.Set {
		add("duration_seconds", "int", p.DurationSeconds.Value)
	}
	if p.Completed.Set {
		add("completed", "boolean", p.Completed.Value)
	}
	if p.AssistBand.Set {
		add("assist_band", "text", p.AssistBand.Value)
	}
	if p.Notes.Set {
		add("notes", "text", p.Notes.Value)
	}
	var res SetResult
	if err := tx.QueryRow(ctx, `update public.session_sets ss set `+strings.Join(sets, ", ")+`
 where ss.id = $1::uuid and ss.user_id = $2::uuid returning `+setCols, args...).Scan(setDest(&res.Set)...); err != nil {
		return SetResult{}, err
	}
	var total, done int
	if err := tx.QueryRow(ctx, `select count(*)::int, (count(*) filter (where ss.completed))::int
  from public.session_sets ss join public.session_exercises se on se.id = ss.session_exercise_id
 where se.session_id = $1::uuid and se.user_id = $2::uuid`, sessionID, userID).Scan(&total, &done); err != nil {
		return SetResult{}, err
	}
	res.ProgressPercent = Progress(done, total)
	return res, tx.Commit(ctx)
}

func (r *PGRepository) FinishSession(ctx context.Context, userID, id string, notes *string) (Session, error) {
	tx, err := r.begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	var planID *string
	err = tx.QueryRow(ctx, `select status, plan_id::text from public.workout_sessions
 where id = $1::uuid and user_id = $2::uuid for update`, id, userID).Scan(&status, &planID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	switch status {
	case StatusActive:
		if _, err := tx.Exec(ctx, `update public.workout_sessions
   set status = 'completed', finished_at = greatest(now(), started_at), notes = coalesce($3::text, notes)
 where id = $1::uuid and user_id = $2::uuid`, id, userID, notes); err != nil {
			return Session{}, err
		}
		if planID != nil {
			if _, err := tx.Exec(ctx, `update public.workout_plans set status = 'completed'
 where id = $1::uuid and user_id = $2::uuid`, *planID, userID); err != nil {
				return Session{}, err
			}
		}
	case StatusDone: // finishing twice is a no-op
	default:
		return Session{}, ErrSessionClosed
	}
	s, err := getSession(ctx, tx, userID, `s.id = $2::uuid`, id)
	if err != nil {
		return Session{}, err
	}
	return s, tx.Commit(ctx)
}

// -------------------------------------------------------------- records

// Records: best reps (reps exercises) / best hold (hold exercises) over completed sets of
// non-abandoned sessions; achieved_at is the latest session time that reached the best value.
func (r *PGRepository) Records(ctx context.Context, userID string) ([]PersonalRecord, error) {
	q, err := r.q(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, `
with sets as (
  select se.exercise_id, e.name_en, coalesce(s.finished_at, s.started_at) as at,
         case when e.exercise_type = 'reps' then ss.actual_reps end as reps,
         case when e.exercise_type = 'hold' then ss.duration_seconds end as hold
    from public.session_sets ss
    join public.session_exercises se on se.id = ss.session_exercise_id and se.user_id = ss.user_id
    join public.workout_sessions s on s.id = se.session_id and s.user_id = se.user_id
    join public.exercises e on e.id = se.exercise_id
   where ss.user_id = $1::uuid and ss.completed and s.status <> 'abandoned'
), best as (
  select exercise_id, max(reps) as best_reps, max(hold) as best_hold from sets group by exercise_id
)
select s.exercise_id::text, min(s.name_en), b.best_reps, b.best_hold, max(s.at)
  from sets s join best b on b.exercise_id = s.exercise_id
 where (s.reps is not null and s.reps = b.best_reps) or (s.hold is not null and s.hold = b.best_hold)
 group by s.exercise_id, b.best_reps, b.best_hold
having coalesce(b.best_reps, 0) > 0 or coalesce(b.best_hold, 0) > 0
 order by min(s.name_en), s.exercise_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersonalRecord{}
	for rows.Next() {
		var p PersonalRecord
		if err := rows.Scan(&p.ExerciseID, &p.ExerciseName, &p.BestReps, &p.BestHoldSeconds, &p.AchievedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
