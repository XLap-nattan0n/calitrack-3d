// Package workout implements plans, sessions/sets and personal records
// as Handler -> Service -> Repository. Every repository method is scoped by userID;
// rows owned by someone else are reported as ErrNotFound (HTTP 404).
package workout

import (
	"context"
	"encoding/base64"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation failed")
)

// ValidationError carries field-level details for the 400 response.
type ValidationError struct{ Details map[string]any }

func (e *ValidationError) Error() string { return ErrValidation.Error() }
func (e *ValidationError) Unwrap() error { return ErrValidation }

func invalid(field, msg string) error {
	return &ValidationError{Details: map[string]any{field: msg}}
}

const (
	DefaultLimit   = 20
	MaxLimit       = 100
	maxTitleLen    = 120
	maxTextLen     = 2000
	maxBandLen     = 50
	maxPlanItems   = 50
	maxTargetSets  = 100
	maxIntValue    = 100000
	dateLayout     = "2006-01-02"
	StatusActive   = "in_progress"
	StatusDone     = "completed"
	StatusAbandon  = "abandoned"
	cursorTimeFmt  = time.RFC3339Nano
	maxCursorBytes = 200
)

var (
	planCategories = map[string]bool{"push": true, "pull": true, "legs": true, "core": true, "full_body": true, "skills": true, "custom": true}
	sessionStatus  = map[string]bool{StatusActive: true, StatusDone: true, StatusAbandon: true}
	uuidRE         = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// ---- plans

// PlanExerciseInput mirrors the OpenAPI PlanExerciseInput schema.
type PlanExerciseInput struct {
	ExerciseID  string  `json:"exercise_id"`
	TargetSets  int     `json:"target_sets"`
	TargetReps  *int    `json:"target_reps"`
	HoldSeconds *int    `json:"hold_seconds"`
	RestSeconds *int    `json:"rest_seconds"`
	AssistBand  *string `json:"assist_band"`
}

// PlanInput mirrors the OpenAPI PlanInput schema; PlanDate is validated YYYY-MM-DD.
type PlanInput struct {
	PlanDate             string              `json:"plan_date"`
	Title                string              `json:"title"`
	Category             string              `json:"category"`
	TargetMuscleGroupIDs []int               `json:"target_muscle_group_ids"`
	Exercises            []PlanExerciseInput `json:"exercises"`
}

type PlanExercise struct {
	PlanExerciseInput
	ID       string `json:"id"`
	Position int    `json:"position"`
}

type Plan struct {
	ID                   string         `json:"id"`
	PlanDate             string         `json:"plan_date"`
	Title                string         `json:"title"`
	Category             string         `json:"category"`
	TargetMuscleGroupIDs []int          `json:"target_muscle_group_ids"`
	Status               string         `json:"status"`
	Exercises            []PlanExercise `json:"exercises"`
}

// ---- sessions

type SessionSet struct {
	ID              string  `json:"id"`
	SetNumber       int     `json:"set_number"`
	TargetReps      *int    `json:"target_reps"`
	ActualReps      *int    `json:"actual_reps"`
	DurationSeconds *int    `json:"duration_seconds"`
	Completed       bool    `json:"completed"`
	AssistBand      *string `json:"assist_band"`
	Notes           *string `json:"notes"`
}

type SessionExercise struct {
	ID           string       `json:"id"`
	ExerciseID   string       `json:"exercise_id"`
	ExerciseName string       `json:"exercise_name"`
	Position     int          `json:"position"`
	Sets         []SessionSet `json:"sets"`
}

type SessionSummary struct {
	ID              string     `json:"id"`
	PlanID          *string    `json:"plan_id"`
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
	ProgressPercent float64    `json:"progress_percent"`
}

type Session struct {
	SessionSummary
	Notes     *string           `json:"notes"`
	Exercises []SessionExercise `json:"exercises"`
}

// Optional distinguishes an absent JSON field from an explicit null.
type Optional[T any] struct {
	Set   bool
	Value *T
}

// SetPatch is the PATCH /sessions/{id}/sets/{setId} body.
type SetPatch struct {
	ActualReps      Optional[int]
	DurationSeconds Optional[int]
	Completed       Optional[bool]
	AssistBand      Optional[string]
	Notes           Optional[string]
}

type SetResult struct {
	Set             SessionSet `json:"set"`
	ProgressPercent float64    `json:"progress_percent"`
}

type SessionFilter struct {
	From, To     *string // inclusive dates (UTC)
	Status       string
	Limit        int
	AfterStarted *time.Time
	AfterID      string
}

type SessionPage struct {
	Items      []SessionSummary `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}

type SessionParams struct {
	From, To, Status, Limit, Cursor string
}

type PersonalRecord struct {
	ExerciseID      string    `json:"exercise_id"`
	ExerciseName    string    `json:"exercise_name"`
	BestReps        *int      `json:"best_reps"`
	BestHoldSeconds *int      `json:"best_hold_seconds"`
	AchievedAt      time.Time `json:"achieved_at"`
}

// Repository: every method is scoped by userID. Missing or foreign rows return ErrNotFound.
type Repository interface {
	ListPlans(ctx context.Context, userID string, from, to *string) ([]Plan, error)
	CreatePlan(ctx context.Context, userID string, in PlanInput) (Plan, error)
	GetPlan(ctx context.Context, userID, id string) (Plan, error)
	ReplacePlan(ctx context.Context, userID, id string, in PlanInput) (Plan, error)
	DeletePlan(ctx context.Context, userID, id string) error

	// StartSession snapshots the plan's exercises into session exercises/sets (empty session when planID is nil).
	StartSession(ctx context.Context, userID string, planID *string) (Session, error)
	ListSessions(ctx context.Context, userID string, f SessionFilter) ([]SessionSummary, error)
	ActiveSession(ctx context.Context, userID string) (Session, error)
	GetSession(ctx context.Context, userID, id string) (Session, error)
	// UpdateSet returns ErrSessionClosed if the session is not in progress.
	UpdateSet(ctx context.Context, userID, sessionID, setID string, p SetPatch) (SetResult, error)
	FinishSession(ctx context.Context, userID, id string, notes *string) (Session, error)

	Records(ctx context.Context, userID string) ([]PersonalRecord, error)
}

// ErrSessionClosed: the session is no longer in progress so its sets are read-only.
var ErrSessionClosed = errors.New("session is not in progress")

// ErrUnknownReference: a plan referenced an exercise or muscle group that does not exist.
type ErrUnknownReference struct{ Field string }

func (e *ErrUnknownReference) Error() string { return "unknown reference: " + e.Field }

type Service struct{ repo Repository }

func NewService(r Repository) *Service { return &Service{repo: r} }

func (s *Service) ListPlans(ctx context.Context, userID, from, to string) ([]Plan, error) {
	f, t, err := parseRange(from, to)
	if err != nil {
		return nil, err
	}
	return s.repo.ListPlans(ctx, userID, f, t)
}

func (s *Service) CreatePlan(ctx context.Context, userID string, in PlanInput) (Plan, error) {
	if err := validatePlan(&in); err != nil {
		return Plan{}, err
	}
	return s.repo.CreatePlan(ctx, userID, in)
}

func (s *Service) GetPlan(ctx context.Context, userID, id string) (Plan, error) {
	if !uuidRE.MatchString(id) {
		return Plan{}, ErrNotFound
	}
	return s.repo.GetPlan(ctx, userID, id)
}

func (s *Service) ReplacePlan(ctx context.Context, userID, id string, in PlanInput) (Plan, error) {
	if !uuidRE.MatchString(id) {
		return Plan{}, ErrNotFound
	}
	if err := validatePlan(&in); err != nil {
		return Plan{}, err
	}
	return s.repo.ReplacePlan(ctx, userID, id, in)
}

func (s *Service) DeletePlan(ctx context.Context, userID, id string) error {
	if !uuidRE.MatchString(id) {
		return ErrNotFound
	}
	return s.repo.DeletePlan(ctx, userID, id)
}

func (s *Service) StartSession(ctx context.Context, userID string, planID *string) (Session, error) {
	if planID != nil && !uuidRE.MatchString(*planID) {
		return Session{}, invalid("plan_id", "must be a UUID")
	}
	return s.repo.StartSession(ctx, userID, planID)
}

func (s *Service) ListSessions(ctx context.Context, userID string, p SessionParams) (SessionPage, error) {
	from, to, err := parseRange(p.From, p.To)
	if err != nil {
		return SessionPage{}, err
	}
	f := SessionFilter{From: from, To: to, Limit: DefaultLimit}
	if p.Status != "" {
		if !sessionStatus[p.Status] {
			return SessionPage{}, invalid("status", "must be one of in_progress, completed, abandoned")
		}
		f.Status = p.Status
	}
	if p.Limit != "" {
		n, err := strconv.Atoi(p.Limit)
		if err != nil || n < 1 || n > MaxLimit {
			return SessionPage{}, invalid("limit", "must be an integer between 1 and 100")
		}
		f.Limit = n
	}
	if p.Cursor != "" {
		t, id, ok := decodeCursor(p.Cursor)
		if !ok {
			return SessionPage{}, invalid("cursor", "invalid cursor")
		}
		f.AfterStarted, f.AfterID = &t, id
	}
	rows, err := s.repo.ListSessions(ctx, userID, f)
	if err != nil {
		return SessionPage{}, err
	}
	page := SessionPage{Items: rows}
	if len(rows) > f.Limit {
		page.Items = rows[:f.Limit]
		last := page.Items[f.Limit-1]
		c := encodeCursor(last.StartedAt, last.ID)
		page.NextCursor = &c
	}
	if page.Items == nil {
		page.Items = []SessionSummary{}
	}
	return page, nil
}

func (s *Service) ActiveSession(ctx context.Context, userID string) (Session, error) {
	return s.repo.ActiveSession(ctx, userID)
}

func (s *Service) GetSession(ctx context.Context, userID, id string) (Session, error) {
	if !uuidRE.MatchString(id) {
		return Session{}, ErrNotFound
	}
	return s.repo.GetSession(ctx, userID, id)
}

func (s *Service) UpdateSet(ctx context.Context, userID, sessionID, setID string, p SetPatch) (SetResult, error) {
	if !uuidRE.MatchString(sessionID) || !uuidRE.MatchString(setID) {
		return SetResult{}, ErrNotFound
	}
	if err := validatePatch(p); err != nil {
		return SetResult{}, err
	}
	r, err := s.repo.UpdateSet(ctx, userID, sessionID, setID, p)
	if err != nil {
		return SetResult{}, err
	}
	r.ProgressPercent = round2(r.ProgressPercent)
	return r, nil
}

func (s *Service) FinishSession(ctx context.Context, userID, id string, notes *string) (Session, error) {
	if !uuidRE.MatchString(id) {
		return Session{}, ErrNotFound
	}
	if notes != nil {
		if err := checkText("notes", *notes, maxTextLen); err != nil {
			return Session{}, err
		}
	}
	return s.repo.FinishSession(ctx, userID, id, notes)
}

func (s *Service) Records(ctx context.Context, userID string) ([]PersonalRecord, error) {
	r, err := s.repo.Records(ctx, userID)
	if r == nil && err == nil {
		r = []PersonalRecord{}
	}
	return r, err
}

// ---- validation helpers

// Progress is completed/total as a percentage with two decimals; 0 when there are no sets.
func Progress(completed, total int) float64 {
	if total <= 0 {
		return 0
	}
	return round2(float64(completed) * 100 / float64(total))
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

func cleanText(s string) bool {
	return utf8.ValidString(s) && strings.IndexFunc(s, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }) < 0
}

func checkText(field, s string, max int) error {
	if !cleanText(s) {
		return invalid(field, "contains invalid characters")
	}
	if utf8.RuneCountInString(s) > max {
		return invalid(field, "too long")
	}
	return nil
}

func parseDate(field, s string) (string, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return "", invalid(field, "must be a date in YYYY-MM-DD format")
	}
	return t.Format(dateLayout), nil
}

func parseRange(from, to string) (*string, *string, error) {
	var f, t *string
	if from != "" {
		d, err := parseDate("from", from)
		if err != nil {
			return nil, nil, err
		}
		f = &d
	}
	if to != "" {
		d, err := parseDate("to", to)
		if err != nil {
			return nil, nil, err
		}
		t = &d
	}
	if f != nil && t != nil && *f > *t {
		return nil, nil, invalid("from", "must not be after to")
	}
	return f, t, nil
}

func validatePlan(in *PlanInput) error {
	d, err := parseDate("plan_date", in.PlanDate)
	if err != nil {
		return err
	}
	in.PlanDate = d
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return invalid("title", "is required")
	}
	if err := checkText("title", in.Title, maxTitleLen); err != nil {
		return err
	}
	if !planCategories[in.Category] {
		return invalid("category", "must be one of push, pull, legs, core, full_body, skills, custom")
	}
	if in.Exercises == nil {
		return invalid("exercises", "is required")
	}
	if len(in.Exercises) > maxPlanItems {
		return invalid("exercises", "too many exercises (max 50)")
	}
	seen := map[int]bool{}
	ids := []int{}
	for _, id := range in.TargetMuscleGroupIDs {
		if id < 1 || id > math.MaxInt16 {
			return invalid("target_muscle_group_ids", "invalid muscle group id")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	in.TargetMuscleGroupIDs = ids
	for i := range in.Exercises {
		if err := validatePlanExercise(i, &in.Exercises[i]); err != nil {
			return err
		}
	}
	return nil
}

func validatePlanExercise(i int, e *PlanExerciseInput) error {
	field := "exercises[" + strconv.Itoa(i) + "]"
	if !uuidRE.MatchString(e.ExerciseID) {
		return invalid(field+".exercise_id", "must be a UUID")
	}
	e.ExerciseID = strings.ToLower(e.ExerciseID)
	if e.TargetSets < 1 || e.TargetSets > maxTargetSets {
		return invalid(field+".target_sets", "must be between 1 and 100")
	}
	if e.TargetReps != nil && (*e.TargetReps < 1 || *e.TargetReps > maxIntValue) {
		return invalid(field+".target_reps", "must be a positive integer")
	}
	if e.HoldSeconds != nil && (*e.HoldSeconds < 1 || *e.HoldSeconds > maxIntValue) {
		return invalid(field+".hold_seconds", "must be a positive integer")
	}
	if e.RestSeconds != nil && (*e.RestSeconds < 0 || *e.RestSeconds > maxIntValue) {
		return invalid(field+".rest_seconds", "must be a non-negative integer")
	}
	if e.AssistBand != nil {
		if err := checkText(field+".assist_band", *e.AssistBand, maxBandLen); err != nil {
			return err
		}
	}
	return nil
}

func validatePatch(p SetPatch) error {
	if p.ActualReps.Set && (p.ActualReps.Value == nil || *p.ActualReps.Value < 0 || *p.ActualReps.Value > maxIntValue) {
		return invalid("actual_reps", "must be a non-negative integer")
	}
	if p.DurationSeconds.Set && (p.DurationSeconds.Value == nil || *p.DurationSeconds.Value < 0 || *p.DurationSeconds.Value > maxIntValue) {
		return invalid("duration_seconds", "must be a non-negative integer")
	}
	if p.Completed.Set && p.Completed.Value == nil {
		return invalid("completed", "must be a boolean")
	}
	if p.AssistBand.Set && p.AssistBand.Value != nil {
		if err := checkText("assist_band", *p.AssistBand.Value, maxBandLen); err != nil {
			return err
		}
	}
	if p.Notes.Set && p.Notes.Value != nil {
		if err := checkText("notes", *p.Notes.Value, maxTextLen); err != nil {
			return err
		}
	}
	if !p.ActualReps.Set && !p.DurationSeconds.Set && !p.Completed.Set && !p.AssistBand.Set && !p.Notes.Set {
		return invalid("body", "at least one field is required")
	}
	return nil
}

func encodeCursor(t time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.UTC().Format(cursorTimeFmt) + "|" + id))
}

func decodeCursor(c string) (time.Time, string, bool) {
	if len(c) > maxCursorBytes {
		return time.Time{}, "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, "", false
	}
	ts, id, ok := strings.Cut(string(raw), "|")
	t, err := time.Parse(cursorTimeFmt, ts)
	if !ok || err != nil || !uuidRE.MatchString(id) {
		return time.Time{}, "", false
	}
	return t, id, true
}
