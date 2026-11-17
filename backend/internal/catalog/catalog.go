// Package catalog implements the reference-data and exercise endpoints
// (equipment, muscle groups, muscles, /me/equipment, /exercises) as Handler -> Service -> Repository.
package catalog

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation failed")
)

// ValidationError carries field-level details for the 400 response.
type ValidationError struct{ Details map[string]any }

func (e *ValidationError) Error() string { return ErrValidation.Error() }
func (e *ValidationError) Unwrap() error { return ErrValidation }

const (
	DefaultLimit = 20
	MaxLimit     = 100
	maxQueryLen  = 100
	bodyweight   = "bodyweight"
)

var (
	equipmentCodes = map[string]bool{"pullup_bar": true, "parallel_bars": true, "resistance_bands": true, bodyweight: true}
	categories     = map[string]bool{"push": true, "pull": true, "legs": true, "core": true, "full_body": true, "skills": true}
	roles          = map[string]bool{"primary": true, "secondary": true}
)

type Equipment struct {
	ID     int     `json:"id"`
	Code   string  `json:"code"`
	NameEn string  `json:"name_en"`
	NameTh *string `json:"name_th"`
}

type MuscleGroup struct {
	ID     int     `json:"id"`
	Code   string  `json:"code"`
	NameEn string  `json:"name_en"`
	NameTh *string `json:"name_th"`
}

type Muscle struct {
	ID       int     `json:"id"`
	GroupID  int     `json:"group_id"`
	Code     string  `json:"code"`
	NameEn   string  `json:"name_en"`
	NameTh   *string `json:"name_th"`
	MeshName string  `json:"mesh_name"`
}

type ExerciseMuscle struct {
	MuscleCode string `json:"muscle_code"`
	Role       string `json:"role"`
}

type ExerciseSummary struct {
	ID             string           `json:"id"`
	Slug           string           `json:"slug"`
	NameEn         string           `json:"name_en"`
	NameTh         *string          `json:"name_th"`
	Category       string           `json:"category"`
	Difficulty     int              `json:"difficulty"`
	ExerciseType   string           `json:"exercise_type"`
	EquipmentCodes []string         `json:"equipment_codes"`
	Muscles        []ExerciseMuscle `json:"muscles"`
}

type Progression struct {
	ExerciseID string `json:"exercise_id"`
	Relation   string `json:"relation"`
}

type ExerciseDetail struct {
	ExerciseSummary
	Description    string        `json:"description"`
	Instructions   string        `json:"instructions"`
	SafetyNotes    string        `json:"safety_notes"`
	RecSets        *int          `json:"rec_sets"`
	RecRepsMin     *int          `json:"rec_reps_min"`
	RecRepsMax     *int          `json:"rec_reps_max"`
	RecHoldSeconds *int          `json:"rec_hold_seconds"`
	Progressions   []Progression `json:"progressions"`
}

// Filter is the validated search input handed to the repository.
// Equipment lists what the user can use; "bodyweight" is always implied.
// Results are ordered by (Difficulty, Slug); the cursor is the last seen pair.
type Filter struct {
	Q          string
	Category   string
	Difficulty int // 0 = any
	Equipment  []string
	Muscle     string
	Role       string
	Limit      int
	AfterDiff  int
	AfterSlug  string
}

// SearchParams is the raw (already syntactically parsed) request input.
type SearchParams struct {
	Q          string
	Category   string
	Difficulty string
	Equipment  []string // nil when the parameter was omitted
	Muscle     string
	Role       string
	Limit      string
	Cursor     string
}

type ExercisePage struct {
	Items      []ExerciseSummary `json:"items"`
	NextCursor *string           `json:"next_cursor"`
}

// Repository: user-owned methods are scoped by userID.
type Repository interface {
	ListEquipment(ctx context.Context) ([]Equipment, error)
	ListMuscleGroups(ctx context.Context) ([]MuscleGroup, error)
	ListMuscles(ctx context.Context) ([]Muscle, error)
	UserEquipment(ctx context.Context, userID string) ([]Equipment, error)
	ReplaceUserEquipment(ctx context.Context, userID string, codes []string) error
	SearchExercises(ctx context.Context, f Filter) ([]ExerciseSummary, error)
	GetExercise(ctx context.Context, id string) (ExerciseDetail, error)
}

type Service struct{ repo Repository }

func NewService(r Repository) *Service { return &Service{repo: r} }

func (s *Service) Equipment(ctx context.Context) ([]Equipment, error) {
	return s.repo.ListEquipment(ctx)
}

func (s *Service) MuscleGroups(ctx context.Context) ([]MuscleGroup, error) {
	return s.repo.ListMuscleGroups(ctx)
}

func (s *Service) Muscles(ctx context.Context) ([]Muscle, error) { return s.repo.ListMuscles(ctx) }

func (s *Service) MyEquipment(ctx context.Context, userID string) ([]Equipment, error) {
	return s.repo.UserEquipment(ctx, userID)
}

// SetMyEquipment replaces the user's selection; an empty list clears it.
func (s *Service) SetMyEquipment(ctx context.Context, userID string, codes []string) ([]Equipment, error) {
	if codes == nil {
		return nil, &ValidationError{Details: map[string]any{"equipment_codes": "required array"}}
	}
	clean, bad := normalizeEquipment(codes)
	if len(bad) > 0 {
		return nil, &ValidationError{Details: map[string]any{"equipment_codes": "unknown codes: " + strings.Join(bad, ", ")}}
	}
	if err := s.repo.ReplaceUserEquipment(ctx, userID, clean); err != nil {
		return nil, err
	}
	return s.repo.UserEquipment(ctx, userID)
}

func (s *Service) GetExercise(ctx context.Context, id string) (ExerciseDetail, error) {
	return s.repo.GetExercise(ctx, id)
}

func (s *Service) SearchExercises(ctx context.Context, userID string, p SearchParams) (ExercisePage, error) {
	f, err := s.buildFilter(p)
	if err != nil {
		return ExercisePage{}, err
	}
	if p.Equipment == nil {
		mine, err := s.repo.UserEquipment(ctx, userID)
		if err != nil {
			return ExercisePage{}, err
		}
		for _, e := range mine {
			f.Equipment = append(f.Equipment, e.Code)
		}
	}
	rows, err := s.repo.SearchExercises(ctx, f) // returns up to Limit+1 rows
	if err != nil {
		return ExercisePage{}, err
	}
	page := ExercisePage{Items: rows}
	if len(rows) > f.Limit {
		page.Items = rows[:f.Limit]
		last := page.Items[f.Limit-1]
		c := encodeCursor(last.Difficulty, last.Slug)
		page.NextCursor = &c
	}
	if page.Items == nil {
		page.Items = []ExerciseSummary{}
	}
	return page, nil
}

func (s *Service) buildFilter(p SearchParams) (Filter, error) {
	bad := map[string]any{}
	f := Filter{Limit: DefaultLimit, Category: p.Category, Role: p.Role, Muscle: strings.TrimSpace(p.Muscle)}

	f.Q = strings.TrimSpace(p.Q)
	if len(f.Q) > maxQueryLen {
		bad["q"] = "max 100 characters"
	}
	if p.Category != "" && !categories[p.Category] {
		bad["category"] = "must be one of push, pull, legs, core, full_body, skills"
	}
	if p.Role != "" && !roles[p.Role] {
		bad["role"] = "must be primary or secondary"
	}
	if p.Difficulty != "" {
		d, err := strconv.Atoi(p.Difficulty)
		if err != nil || d < 1 || d > 5 {
			bad["difficulty"] = "must be an integer from 1 to 5"
		}
		f.Difficulty = d
	}
	if p.Limit != "" {
		l, err := strconv.Atoi(p.Limit)
		if err != nil || l < 1 || l > MaxLimit {
			bad["limit"] = "must be an integer from 1 to 100"
		} else {
			f.Limit = l
		}
	}
	if p.Equipment != nil {
		clean, unknown := normalizeEquipment(p.Equipment)
		if len(unknown) > 0 {
			bad["equipment"] = "unknown codes: " + strings.Join(unknown, ", ")
		}
		f.Equipment = clean
	}
	if p.Cursor != "" {
		d, slug, ok := decodeCursor(p.Cursor)
		if !ok {
			bad["cursor"] = "invalid cursor"
		}
		f.AfterDiff, f.AfterSlug = d, slug
	}
	if len(bad) > 0 {
		return Filter{}, &ValidationError{Details: bad}
	}
	return f, nil
}

// normalizeEquipment trims, de-duplicates and splits valid codes from unknown ones.
func normalizeEquipment(in []string) (valid, unknown []string) {
	seen := map[string]bool{}
	for _, c := range in {
		c = strings.TrimSpace(c)
		if seen[c] {
			continue
		}
		seen[c] = true
		if equipmentCodes[c] {
			valid = append(valid, c)
		} else {
			unknown = append(unknown, c)
		}
	}
	return valid, unknown
}

func encodeCursor(difficulty int, slug string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(difficulty) + ":" + slug))
}

func decodeCursor(c string) (int, string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return 0, "", false
	}
	d, slug, ok := strings.Cut(string(raw), ":")
	n, err := strconv.Atoi(d)
	if !ok || err != nil || n < 1 || n > 5 || slug == "" {
		return 0, "", false
	}
	return n, slug, true
}
