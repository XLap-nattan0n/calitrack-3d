package analytics

import (
	"context"
	"time"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/db"
)

type PGRepository struct{ pool *db.Pool }

func NewPGRepository(p *db.Pool) *PGRepository { return &PGRepository{pool: p} }

// Every query filters on user_id explicitly (the API connects with a role that bypasses RLS).
// $2 is the lower bound on started_at; NULL means "all time".

func (r *PGRepository) Summary(ctx context.Context, userID string, since *time.Time) (int, int, []WeekCount, error) {
	pool, err := r.pool.Get(ctx)
	if err != nil {
		return 0, 0, nil, err
	}
	var total, sets int
	err = pool.QueryRow(ctx, `select
  (select count(*) from public.workout_sessions s
    where s.user_id = $1::uuid and s.status = 'completed' and ($2::timestamptz is null or s.started_at >= $2::timestamptz)),
  (select count(*) from public.session_sets ss
    join public.session_exercises se on se.id = ss.session_exercise_id and se.user_id = ss.user_id
    join public.workout_sessions s on s.id = se.session_id and s.user_id = se.user_id
    where ss.user_id = $1::uuid and ss.completed and s.status = 'completed'
      and ($2::timestamptz is null or s.started_at >= $2::timestamptz))`, userID, since).Scan(&total, &sets)
	if err != nil {
		return 0, 0, nil, err
	}
	rows, err := pool.Query(ctx, `select to_char(date_trunc('week', s.started_at at time zone 'UTC'), 'YYYY-MM-DD'), count(*)::int
  from public.workout_sessions s
  where s.user_id = $1::uuid and s.status = 'completed' and ($2::timestamptz is null or s.started_at >= $2::timestamptz)
  group by 1 order by 1`, userID, since)
	if err != nil {
		return 0, 0, nil, err
	}
	defer rows.Close()
	weeks := []WeekCount{}
	for rows.Next() {
		var w WeekCount
		if err := rows.Scan(&w.WeekStart, &w.Workouts); err != nil {
			return 0, 0, nil, err
		}
		weeks = append(weeks, w)
	}
	return total, sets, weeks, rows.Err()
}

func (r *PGRepository) ExerciseExists(ctx context.Context, exerciseID string) (bool, error) {
	pool, err := r.pool.Get(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	err = pool.QueryRow(ctx, `select exists (select 1 from public.exercises where id = $1::uuid)`, exerciseID).Scan(&ok)
	return ok, err
}

func (r *PGRepository) Progress(ctx context.Context, userID, exerciseID string, since *time.Time) ([]ProgressPoint, error) {
	pool, err := r.pool.Get(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `select to_char(s.started_at at time zone 'UTC', 'YYYY-MM-DD') as day,
  max(ss.actual_reps)::int, max(ss.duration_seconds)::int, count(*)::int
  from public.session_sets ss
  join public.session_exercises se on se.id = ss.session_exercise_id and se.user_id = ss.user_id
  join public.workout_sessions s on s.id = se.session_id and s.user_id = se.user_id
  where ss.user_id = $1::uuid and se.exercise_id = $2::uuid and ss.completed and s.status = 'completed'
    and ($3::timestamptz is null or s.started_at >= $3::timestamptz)
  group by day order by day`, userID, exerciseID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProgressPoint{}
	for rows.Next() {
		var p ProgressPoint
		if err := rows.Scan(&p.Date, &p.BestReps, &p.BestHoldSeconds, &p.TotalSets); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MuscleFrequency counts distinct sessions in which a muscle (primary or secondary) was trained.
func (r *PGRepository) MuscleFrequency(ctx context.Context, userID string, since *time.Time) ([]MuscleFrequency, error) {
	pool, err := r.pool.Get(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `select m.code, g.code, count(distinct s.id)::int as sessions
  from public.session_sets ss
  join public.session_exercises se on se.id = ss.session_exercise_id and se.user_id = ss.user_id
  join public.workout_sessions s on s.id = se.session_id and s.user_id = se.user_id
  join public.exercise_muscles em on em.exercise_id = se.exercise_id
  join public.muscles m on m.id = em.muscle_id
  join public.muscle_groups g on g.id = m.group_id
  where ss.user_id = $1::uuid and ss.completed and s.status = 'completed'
    and ($2::timestamptz is null or s.started_at >= $2::timestamptz)
  group by m.code, g.code order by sessions desc, m.code`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MuscleFrequency{}
	for rows.Next() {
		var f MuscleFrequency
		if err := rows.Scan(&f.MuscleCode, &f.GroupCode, &f.Sessions); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
