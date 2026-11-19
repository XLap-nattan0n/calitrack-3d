// Package analytics computes user-scoped training statistics from session_sets.
// Only sessions with status 'completed' and sets with completed = true are counted;
// all dates are UTC.
package analytics

import (
	"context"
	"errors"
	"regexp"
	"time"
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
	DefaultRange = "30d"
	dateLayout   = "2006-01-02"
)

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type WeekCount struct {
	WeekStart string `json:"week_start"`
	Workouts  int    `json:"workouts"`
}

type Summary struct {
	TotalWorkouts   int         `json:"total_workouts"`
	CompletedSets   int         `json:"completed_sets"`
	WeeklyFrequency []WeekCount `json:"weekly_frequency"`
}

type ProgressPoint struct {
	Date            string `json:"date"`
	BestReps        *int   `json:"best_reps"`
	BestHoldSeconds *int   `json:"best_hold_seconds"`
	TotalSets       int    `json:"total_sets"`
}

type Progress struct {
	Series []ProgressPoint `json:"series"`
}

type MuscleFrequency struct {
	MuscleCode string `json:"muscle_code"`
	GroupCode  string `json:"group_code"`
	Sessions   int    `json:"sessions"`
}

// Repository: every method is scoped by userID. since == nil means no lower bound.
type Repository interface {
	// Summary returns the totals and the (sparse) number of workouts per ISO week (Monday, UTC).
	Summary(ctx context.Context, userID string, since *time.Time) (total, sets int, weeks []WeekCount, err error)
	// ExerciseExists reports whether the (public) exercise exists.
	ExerciseExists(ctx context.Context, exerciseID string) (bool, error)
	Progress(ctx context.Context, userID, exerciseID string, since *time.Time) ([]ProgressPoint, error)
	MuscleFrequency(ctx context.Context, userID string, since *time.Time) ([]MuscleFrequency, error)
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(r Repository) *Service { return &Service{repo: r, now: time.Now} }

// parseRange maps the OpenAPI range enum to a lower bound (nil for "all").
func (s *Service) parseRange(v string) (*time.Time, error) {
	if v == "" {
		v = DefaultRange
	}
	now := s.now().UTC()
	var t time.Time
	switch v {
	case "7d":
		t = now.AddDate(0, 0, -7)
	case "30d":
		t = now.AddDate(0, 0, -30)
	case "90d":
		t = now.AddDate(0, 0, -90)
	case "1y":
		t = now.AddDate(-1, 0, 0)
	case "all":
		return nil, nil
	default:
		return nil, &ValidationError{Details: map[string]any{"range": "must be one of 7d, 30d, 90d, 1y, all"}}
	}
	return &t, nil
}

func (s *Service) Summary(ctx context.Context, userID, rng string) (Summary, error) {
	since, err := s.parseRange(rng)
	if err != nil {
		return Summary{}, err
	}
	total, sets, weeks, err := s.repo.Summary(ctx, userID, since)
	if err != nil {
		return Summary{}, err
	}
	return Summary{TotalWorkouts: total, CompletedSets: sets, WeeklyFrequency: fillWeeks(weeks, since, s.now().UTC())}, nil
}

// weekStart returns the Monday (UTC, midnight) of t's week.
func weekStart(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return t.AddDate(0, 0, -((int(t.Weekday()) + 6) % 7))
}

// fillWeeks adds zero-count weeks so charts get a continuous series. For "all" it starts at the first workout.
func fillWeeks(weeks []WeekCount, since *time.Time, now time.Time) []WeekCount {
	counts := map[string]int{}
	var first time.Time
	for _, w := range weeks {
		counts[w.WeekStart] = w.Workouts
		if d, err := time.Parse(dateLayout, w.WeekStart); err == nil && (first.IsZero() || d.Before(first)) {
			first = d
		}
	}
	out := []WeekCount{}
	var start time.Time
	switch {
	case since != nil:
		start = weekStart(*since)
	case !first.IsZero():
		start = first
	default:
		return out
	}
	for d := start; !d.After(now); d = d.AddDate(0, 0, 7) {
		k := d.Format(dateLayout)
		out = append(out, WeekCount{WeekStart: k, Workouts: counts[k]})
	}
	return out
}

func (s *Service) Progress(ctx context.Context, userID, exerciseID, rng string) (Progress, error) {
	if !uuidRE.MatchString(exerciseID) {
		return Progress{}, ErrNotFound
	}
	since, err := s.parseRange(rng)
	if err != nil {
		return Progress{}, err
	}
	ok, err := s.repo.ExerciseExists(ctx, exerciseID)
	if err != nil {
		return Progress{}, err
	}
	if !ok {
		return Progress{}, ErrNotFound
	}
	series, err := s.repo.Progress(ctx, userID, exerciseID, since)
	if err != nil {
		return Progress{}, err
	}
	if series == nil {
		series = []ProgressPoint{}
	}
	return Progress{Series: series}, nil
}

func (s *Service) MuscleFrequency(ctx context.Context, userID, rng string) ([]MuscleFrequency, error) {
	since, err := s.parseRange(rng)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.MuscleFrequency(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []MuscleFrequency{}
	}
	return items, nil
}
