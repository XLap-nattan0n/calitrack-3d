package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"

	"github.com/jackc/pgx/v5"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/db"
)

type PGRepository struct{ pool *db.Pool }

func NewPGRepository(p *db.Pool) *PGRepository { return &PGRepository{pool: p} }

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func (r *PGRepository) ListEquipment(ctx context.Context) ([]Equipment, error) {
	return r.equipment(ctx, `select id, code, name_en, name_th from public.equipment order by id`)
}

func (r *PGRepository) UserEquipment(ctx context.Context, userID string) ([]Equipment, error) {
	return r.equipment(ctx,
		`select q.id, q.code, q.name_en, q.name_th
		   from public.user_equipment ue join public.equipment q on q.id = ue.equipment_id
		  where ue.user_id = $1 order by q.id`, userID)
}

func (r *PGRepository) equipment(ctx context.Context, sql string, args ...any) ([]Equipment, error) {
	pool, err := r.pool.Get(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Equipment{}
	for rows.Next() {
		var e Equipment
		if err := rows.Scan(&e.ID, &e.Code, &e.NameEn, &e.NameTh); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *PGRepository) ListMuscleGroups(ctx context.Context) ([]MuscleGroup, error) {
	pool, err := r.pool.Get(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `select id, code, name_en, name_th from public.muscle_groups order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MuscleGroup{}
	for rows.Next() {
		var g MuscleGroup
		if err := rows.Scan(&g.ID, &g.Code, &g.NameEn, &g.NameTh); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *PGRepository) ListMuscles(ctx context.Context) ([]Muscle, error) {
	pool, err := r.pool.Get(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`select id, group_id, code, name_en, name_th, coalesce(mesh_name, code) from public.muscles order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Muscle{}
	for rows.Next() {
		var m Muscle
		if err := rows.Scan(&m.ID, &m.GroupID, &m.Code, &m.NameEn, &m.NameTh, &m.MeshName); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *PGRepository) ReplaceUserEquipment(ctx context.Context, userID string, codes []string) error {
	pool, err := r.pool.Get(ctx)
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `delete from public.user_equipment where user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`insert into public.user_equipment (user_id, equipment_id)
		 select $1::uuid, id from public.equipment where code = any($2::text[])`, userID, codes); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const exerciseCols = `
  e.id::text, e.slug, e.name_th, e.name_en, e.category, e.difficulty::int, e.exercise_type,
  coalesce((select array_agg(q.code order by q.code)
              from public.exercise_equipment ee join public.equipment q on q.id = ee.equipment_id
             where ee.exercise_id = e.id), '{}'::text[]),
  coalesce((select json_agg(json_build_object('muscle_code', m.code, 'role', em.role)
                            order by (em.role = 'secondary'), m.code)
              from public.exercise_muscles em join public.muscles m on m.id = em.muscle_id
             where em.exercise_id = e.id), '[]'::json)::text`

// SearchExercises returns up to f.Limit+1 rows ordered by (difficulty, slug).
// An exercise matches when every equipment it needs is available; bodyweight is always available.
func (r *PGRepository) SearchExercises(ctx context.Context, f Filter) ([]ExerciseSummary, error) {
	pool, err := r.pool.Get(ctx)
	if err != nil {
		return nil, err
	}
	equipment := f.Equipment
	if equipment == nil {
		equipment = []string{}
	}
	rows, err := pool.Query(ctx, `select `+exerciseCols+`
  from public.exercises e
 where ($1 = '' or position(lower($1) in lower(e.name_en)) > 0 or position(lower($1) in e.slug) > 0)
   and ($2 = '' or e.category = $2)
   and ($3::int = 0 or e.difficulty = $3::int)
   and not exists (select 1
                     from public.exercise_equipment ee join public.equipment q on q.id = ee.equipment_id
                    where ee.exercise_id = e.id and q.code <> 'bodyweight' and not (q.code = any($4::text[])))
   and (($5 = '' and $6 = '') or exists (
         select 1 from public.exercise_muscles em join public.muscles m on m.id = em.muscle_id
          where em.exercise_id = e.id and ($5 = '' or m.code = $5) and ($6 = '' or em.role = $6)))
   and ($7::int = 0 or (e.difficulty::int, e.slug) > ($7::int, $8))
 order by e.difficulty, e.slug
 limit $9`,
		f.Q, f.Category, f.Difficulty, equipment, f.Muscle, f.Role, f.AfterDiff, f.AfterSlug, f.Limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExerciseSummary{}
	for rows.Next() {
		s, err := scanSummary(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *PGRepository) GetExercise(ctx context.Context, id string) (ExerciseDetail, error) {
	if !uuidRE.MatchString(id) {
		return ExerciseDetail{}, ErrNotFound
	}
	pool, err := r.pool.Get(ctx)
	if err != nil {
		return ExerciseDetail{}, err
	}
	var d ExerciseDetail
	var muscles string
	err = pool.QueryRow(ctx, `select `+exerciseCols+`,
  coalesce(e.description, ''), coalesce(e.instructions, ''), coalesce(e.safety_notes, ''),
  e.rec_sets, e.rec_reps_min, e.rec_reps_max, e.rec_hold_seconds
  from public.exercises e where e.id = $1::uuid`, id).Scan(
		&d.ID, &d.Slug, &d.NameTh, &d.NameEn, &d.Category, &d.Difficulty, &d.ExerciseType,
		&d.EquipmentCodes, &muscles,
		&d.Description, &d.Instructions, &d.SafetyNotes,
		&d.RecSets, &d.RecRepsMin, &d.RecRepsMax, &d.RecHoldSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return ExerciseDetail{}, ErrNotFound
	}
	if err != nil {
		return ExerciseDetail{}, err
	}
	if err := json.Unmarshal([]byte(muscles), &d.Muscles); err != nil {
		return ExerciseDetail{}, err
	}

	rows, err := pool.Query(ctx,
		`select p.next_exercise_id::text, p.relation
		   from public.exercise_progressions p join public.exercises n on n.id = p.next_exercise_id
		  where p.exercise_id = $1::uuid order by p.relation, n.difficulty, n.slug`, id)
	if err != nil {
		return ExerciseDetail{}, err
	}
	defer rows.Close()
	d.Progressions = []Progression{}
	for rows.Next() {
		var p Progression
		if err := rows.Scan(&p.ExerciseID, &p.Relation); err != nil {
			return ExerciseDetail{}, err
		}
		d.Progressions = append(d.Progressions, p)
	}
	return d, rows.Err()
}

func scanSummary(rows pgx.Rows) (ExerciseSummary, error) {
	var s ExerciseSummary
	var muscles string
	if err := rows.Scan(&s.ID, &s.Slug, &s.NameTh, &s.NameEn, &s.Category, &s.Difficulty, &s.ExerciseType,
		&s.EquipmentCodes, &muscles); err != nil {
		return s, err
	}
	return s, json.Unmarshal([]byte(muscles), &s.Muscles)
}
