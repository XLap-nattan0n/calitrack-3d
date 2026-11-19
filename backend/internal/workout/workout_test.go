package workout

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/auth"
)

const (
	userA  = "aaaaaaaa-0000-0000-0000-000000000001"
	userB  = "bbbbbbbb-0000-0000-0000-000000000002"
	exID   = "11111111-1111-1111-1111-111111111111"
	badUID = "not-a-uuid"
)

func uid(n int) string {
	return "00000000-0000-0000-0000-" + strings.Repeat("0", 12-len(strconv.Itoa(n))) + strconv.Itoa(n)
}

// memRepo is an in-memory Repository that enforces the same user_id scoping as the SQL one.
type memRepo struct {
	n        int
	plans    map[string]Plan
	planUser map[string]string
	sessions map[string]*Session
	sessUser map[string]string
	gotF     SessionFilter
	gotPlan  PlanInput
}

func newMem() *memRepo {
	return &memRepo{plans: map[string]Plan{}, planUser: map[string]string{}, sessions: map[string]*Session{}, sessUser: map[string]string{}}
}

func (m *memRepo) id() string { m.n++; return uid(m.n) }

func (m *memRepo) toPlan(id string, in PlanInput) Plan {
	p := Plan{ID: id, PlanDate: in.PlanDate, Title: in.Title, Category: in.Category, TargetMuscleGroupIDs: in.TargetMuscleGroupIDs, Status: "planned", Exercises: []PlanExercise{}}
	for i, e := range in.Exercises {
		p.Exercises = append(p.Exercises, PlanExercise{PlanExerciseInput: e, ID: m.id(), Position: i})
	}
	return p
}

func (m *memRepo) ListPlans(_ context.Context, u string, from, to *string) ([]Plan, error) {
	out := []Plan{}
	for id, p := range m.plans {
		if m.planUser[id] == u && (from == nil || p.PlanDate >= *from) && (to == nil || p.PlanDate <= *to) {
			out = append(out, p)
		}
	}
	return out, nil
}
func (m *memRepo) CreatePlan(_ context.Context, u string, in PlanInput) (Plan, error) {
	m.gotPlan = in
	id := m.id()
	m.plans[id], m.planUser[id] = m.toPlan(id, in), u
	return m.plans[id], nil
}
func (m *memRepo) GetPlan(_ context.Context, u, id string) (Plan, error) {
	if m.planUser[id] != u {
		return Plan{}, ErrNotFound
	}
	return m.plans[id], nil
}
func (m *memRepo) ReplacePlan(_ context.Context, u, id string, in PlanInput) (Plan, error) {
	if m.planUser[id] != u {
		return Plan{}, ErrNotFound
	}
	m.plans[id] = m.toPlan(id, in)
	return m.plans[id], nil
}
func (m *memRepo) DeletePlan(_ context.Context, u, id string) error {
	if m.planUser[id] != u {
		return ErrNotFound
	}
	delete(m.plans, id)
	delete(m.planUser, id)
	return nil
}
func (m *memRepo) StartSession(_ context.Context, u string, planID *string) (Session, error) {
	s := &Session{SessionSummary: SessionSummary{ID: m.id(), PlanID: planID, Status: StatusActive, StartedAt: time.Now().UTC()}, Exercises: []SessionExercise{}}
	if planID != nil {
		p, ok := m.plans[*planID]
		if !ok || m.planUser[*planID] != u {
			return Session{}, ErrNotFound
		}
		for _, e := range p.Exercises {
			se := SessionExercise{ID: m.id(), ExerciseID: e.ExerciseID, Position: e.Position, Sets: []SessionSet{}}
			for n := 1; n <= e.TargetSets; n++ {
				se.Sets = append(se.Sets, SessionSet{ID: m.id(), SetNumber: n, TargetReps: e.TargetReps})
			}
			s.Exercises = append(s.Exercises, se)
		}
	}
	m.sessions[s.ID], m.sessUser[s.ID] = s, u
	return *s, nil
}
func (m *memRepo) ListSessions(_ context.Context, u string, f SessionFilter) ([]SessionSummary, error) {
	m.gotF = f
	out := []SessionSummary{}
	for id, s := range m.sessions {
		if m.sessUser[id] == u {
			out = append(out, s.SessionSummary)
		}
	}
	return out, nil
}
func (m *memRepo) ActiveSession(_ context.Context, u string) (Session, error) {
	for id, s := range m.sessions {
		if m.sessUser[id] == u && s.Status == StatusActive {
			return *s, nil
		}
	}
	return Session{}, ErrNotFound
}
func (m *memRepo) GetSession(_ context.Context, u, id string) (Session, error) {
	if m.sessUser[id] != u {
		return Session{}, ErrNotFound
	}
	return *m.sessions[id], nil
}
func (m *memRepo) UpdateSet(_ context.Context, u, sid, setID string, p SetPatch) (SetResult, error) {
	if m.sessUser[sid] != u {
		return SetResult{}, ErrNotFound
	}
	s := m.sessions[sid]
	if s.Status != StatusActive {
		return SetResult{}, ErrSessionClosed
	}
	total, done := 0, 0
	var res SetResult
	found := false
	for i := range s.Exercises {
		for j := range s.Exercises[i].Sets {
			st := &s.Exercises[i].Sets[j]
			if st.ID == setID {
				found = true
				if p.ActualReps.Set {
					st.ActualReps = p.ActualReps.Value
				}
				if p.Completed.Set {
					st.Completed = *p.Completed.Value
				}
				if p.Notes.Set {
					st.Notes = p.Notes.Value
				}
				res.Set = *st
			}
			total++
			if st.Completed {
				done++
			}
		}
	}
	if !found {
		return SetResult{}, ErrNotFound
	}
	res.ProgressPercent = Progress(done, total)
	return res, nil
}
func (m *memRepo) FinishSession(_ context.Context, u, id string, notes *string) (Session, error) {
	if m.sessUser[id] != u {
		return Session{}, ErrNotFound
	}
	s := m.sessions[id]
	s.Status, s.Notes = StatusDone, notes
	return *s, nil
}
func (m *memRepo) Records(context.Context, string) ([]PersonalRecord, error) { return nil, nil }

// ---- helpers

func router(repo Repository) http.Handler {
	h := NewHandler(NewService(repo))
	r := chi.NewRouter()
	r.Get("/plans", h.ListPlans)
	r.Post("/plans", h.CreatePlan)
	r.Get("/plans/{id}", h.GetPlan)
	r.Put("/plans/{id}", h.ReplacePlan)
	r.Delete("/plans/{id}", h.DeletePlan)
	r.Get("/sessions", h.ListSessions)
	r.Post("/sessions", h.StartSession)
	r.Get("/sessions/active", h.ActiveSession)
	r.Get("/sessions/{id}", h.GetSession)
	r.Patch("/sessions/{id}/sets/{setId}", h.UpdateSet)
	r.Post("/sessions/{id}/finish", h.FinishSession)
	r.Get("/records", h.Records)
	return r
}

func req(h http.Handler, user, method, target, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r = r.WithContext(auth.WithUserID(r.Context(), user))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

const planBody = `{"plan_date":"2026-11-20","title":"Push day","category":"push","target_muscle_group_ids":[1,1,2],
 "exercises":[{"exercise_id":"` + exID + `","target_sets":3,"target_reps":8},{"exercise_id":"` + exID + `","target_sets":2,"hold_seconds":30}]}`

func decodeInto(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
}

func createPlan(t *testing.T, h http.Handler, user string) Plan {
	t.Helper()
	rec := req(h, user, "POST", "/plans", planBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create plan = %d %s", rec.Code, rec.Body)
	}
	var p Plan
	decodeInto(t, rec, &p)
	return p
}

// ---- tests

func TestProgress(t *testing.T) {
	for _, c := range []struct {
		done, total int
		want        float64
	}{{0, 0, 0}, {0, 5, 0}, {1, 3, 33.33}, {2, 3, 66.67}, {5, 5, 100}} {
		if got := Progress(c.done, c.total); got != c.want {
			t.Errorf("Progress(%d,%d) = %v, want %v", c.done, c.total, got, c.want)
		}
	}
}

func TestCreatePlanNormalizesAndReturns201(t *testing.T) {
	repo := newMem()
	h := router(repo)
	p := createPlan(t, h, userA)
	if len(p.Exercises) != 2 || p.Exercises[1].Position != 1 || p.Status != "planned" {
		t.Errorf("plan = %+v", p)
	}
	if got := repo.gotPlan.TargetMuscleGroupIDs; len(got) != 2 {
		t.Errorf("muscle group ids not de-duplicated: %v", got)
	}
}

func TestPlanValidation(t *testing.T) {
	h := router(newMem())
	ex := `"exercises":[{"exercise_id":"` + exID + `","target_sets":1}]`
	bad := []string{
		``, `{`, `[]`, `{"unknown":1}`,
		`{"title":"t","category":"push","plan_date":"2026-13-01",` + ex + `}`,
		`{"title":"t","category":"push","plan_date":"20-11-2026",` + ex + `}`,
		`{"title":"","category":"push","plan_date":"2026-11-20",` + ex + `}`,
		`{"title":"  ","category":"push","plan_date":"2026-11-20",` + ex + `}`,
		`{"title":"` + strings.Repeat("a", 121) + `","category":"push","plan_date":"2026-11-20",` + ex + `}`,
		`{"title":"t\u0000","category":"push","plan_date":"2026-11-20",` + ex + `}`,
		`{"title":"t","category":"yoga","plan_date":"2026-11-20",` + ex + `}`,
		`{"title":"t","category":"push","plan_date":"2026-11-20"}`,
		`{"title":"t","category":"push","plan_date":"2026-11-20","exercises":[{"exercise_id":"x","target_sets":1}]}`,
		`{"title":"t","category":"push","plan_date":"2026-11-20","exercises":[{"exercise_id":"` + exID + `","target_sets":0}]}`,
		`{"title":"t","category":"push","plan_date":"2026-11-20","exercises":[{"exercise_id":"` + exID + `"}]}`,
		`{"title":"t","category":"push","plan_date":"2026-11-20","exercises":[{"exercise_id":"` + exID + `","target_sets":1,"target_reps":-1}]}`,
		`{"title":"t","category":"push","plan_date":"2026-11-20","exercises":[{"exercise_id":"` + exID + `","target_sets":1,"rest_seconds":-1}]}`,
		`{"title":"t","category":"push","plan_date":"2026-11-20","target_muscle_group_ids":[0],` + ex + `}`,
	}
	owned := createPlan(t, h, userA)
	for i, b := range bad {
		for _, m := range []string{"POST /plans", "PUT /plans/" + owned.ID} {
			parts := strings.SplitN(m, " ", 2)
			rec := req(h, userA, parts[0], parts[1], b)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"bad_request"`) {
				t.Errorf("case %d %s = %d %s", i, m, rec.Code, rec.Body)
			}
		}
	}
}

// Ownership (404) must be decided before body validation (400) on PUT /plans/{id}.
func TestReplacePlanChecksOwnershipBeforeValidation(t *testing.T) {
	h := router(newMem())
	p := createPlan(t, h, userA)
	for name, path := range map[string]string{"foreign": "/plans/" + p.ID, "missing": "/plans/" + uid(99)} {
		for _, body := range []string{`{"title":""}`, `not json`, `{}`} {
			rec := req(h, userB, "PUT", path, body)
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s plan, body %q = %d %s, want 404", name, body, rec.Code, rec.Body)
			}
		}
	}
}

func TestPlanOwnership(t *testing.T) {
	h := router(newMem())
	p := createPlan(t, h, userA)
	path := "/plans/" + p.ID
	for _, c := range []struct{ method, body string }{{"GET", ""}, {"PUT", planBody}, {"DELETE", ""}} {
		rec := req(h, userB, c.method, path, c.body)
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"not_found"`) {
			t.Errorf("user B %s foreign plan = %d %s", c.method, rec.Code, rec.Body)
		}
	}
	// B's list must not include A's plan; A still sees it, and it survived B's DELETE.
	var list struct{ Items []Plan }
	decodeInto(t, req(h, userB, "GET", "/plans", ""), &list)
	if len(list.Items) != 0 {
		t.Errorf("user B listed %d foreign plans", len(list.Items))
	}
	if rec := req(h, userA, "GET", path, ""); rec.Code != http.StatusOK {
		t.Errorf("owner GET = %d", rec.Code)
	}
	if rec := req(h, userA, "DELETE", path, ""); rec.Code != http.StatusNoContent {
		t.Errorf("owner DELETE = %d", rec.Code)
	}
	if rec := req(h, userA, "GET", "/plans/"+badUID, ""); rec.Code != http.StatusNotFound {
		t.Errorf("malformed id = %d, want 404", rec.Code)
	}
}

func TestListPlansDateRange(t *testing.T) {
	h := router(newMem())
	createPlan(t, h, userA)
	for target, want := range map[string]int{
		"/plans?from=2026-11-20&to=2026-11-20": 1, "/plans?from=2026-11-21": 0, "/plans?to=2026-11-19": 0, "/plans": 1,
	} {
		var l struct{ Items []Plan }
		rec := req(h, userA, "GET", target, "")
		decodeInto(t, rec, &l)
		if rec.Code != 200 || len(l.Items) != want {
			t.Errorf("%s = %d, %d items, want %d", target, rec.Code, len(l.Items), want)
		}
	}
	for _, target := range []string{"/plans?from=nope", "/plans?to=2026-02-30", "/plans?from=2026-11-21&to=2026-11-20"} {
		if rec := req(h, userA, "GET", target, ""); rec.Code != 400 {
			t.Errorf("%s = %d, want 400", target, rec.Code)
		}
	}
}

func TestSessionSnapshotProgressAndOwnership(t *testing.T) {
	repo := newMem()
	h := router(repo)
	p := createPlan(t, h, userA)

	// user B cannot start a session from A's plan
	if rec := req(h, userB, "POST", "/sessions", `{"plan_id":"`+p.ID+`"}`); rec.Code != 404 {
		t.Fatalf("B starts from A's plan = %d", rec.Code)
	}
	if rec := req(h, userA, "POST", "/sessions", `{"plan_id":"zzz"}`); rec.Code != 400 {
		t.Fatalf("bad plan_id = %d", rec.Code)
	}
	rec := req(h, userA, "POST", "/sessions", `{"plan_id":"`+p.ID+`"}`)
	if rec.Code != 201 {
		t.Fatalf("start = %d %s", rec.Code, rec.Body)
	}
	var s Session
	decodeInto(t, rec, &s)
	if len(s.Exercises) != 2 || len(s.Exercises[0].Sets) != 3 || len(s.Exercises[1].Sets) != 2 {
		t.Fatalf("snapshot = %+v", s.Exercises)
	}

	// B cannot read, patch, or finish A's session; A's data is untouched
	set := s.Exercises[0].Sets[0].ID
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/sessions/" + s.ID, ""},
		{"PATCH", "/sessions/" + s.ID + "/sets/" + set, `{"completed":true}`},
		{"POST", "/sessions/" + s.ID + "/finish", `{}`},
	} {
		if rec := req(h, userB, c.method, c.path, c.body); rec.Code != 404 {
			t.Errorf("B %s %s = %d %s", c.method, c.path, rec.Code, rec.Body)
		}
	}
	if rec := req(h, userB, "GET", "/sessions/active", ""); rec.Code != 404 {
		t.Errorf("B active = %d", rec.Code)
	}
	if repo.sessions[s.ID].Status != StatusActive {
		t.Error("foreign finish changed the session")
	}

	// progress = completed / total (1 of 5 sets = 20)
	rec = req(h, userA, "PATCH", "/sessions/"+s.ID+"/sets/"+set, `{"completed":true,"actual_reps":8,"notes":null}`)
	var res SetResult
	decodeInto(t, rec, &res)
	if rec.Code != 200 || res.ProgressPercent != 20 || !res.Set.Completed || *res.Set.ActualReps != 8 {
		t.Fatalf("patch = %d %s", rec.Code, rec.Body)
	}
	if rec := req(h, userA, "GET", "/sessions/active", ""); rec.Code != 200 {
		t.Errorf("A active = %d", rec.Code)
	}

	// finish, then sets are read-only
	if rec := req(h, userA, "POST", "/sessions/"+s.ID+"/finish", `{"notes":"good"}`); rec.Code != 200 {
		t.Fatalf("finish = %d %s", rec.Code, rec.Body)
	}
	if rec := req(h, userA, "PATCH", "/sessions/"+s.ID+"/sets/"+set, `{"completed":false}`); rec.Code != 400 {
		t.Errorf("patch after finish = %d, want 400", rec.Code)
	}
	if rec := req(h, userA, "POST", "/sessions", `{}`); rec.Code != 201 { // ad-hoc session without a plan
		t.Errorf("start without plan = %d", rec.Code)
	}
	if rec := req(h, userA, "POST", "/sessions", ``); rec.Code != 201 { // empty body allowed
		t.Errorf("start with empty body = %d", rec.Code)
	}
}

func TestPatchSetValidation(t *testing.T) {
	h := router(newMem())
	path := "/sessions/" + uid(1) + "/sets/" + uid(2)
	for _, b := range []string{
		``, `{`, `[]`, `{}`, `{"actual_reps":-1}`, `{"actual_reps":1.5}`, `{"actual_reps":"3"}`, `{"actual_reps":null}`,
		`{"duration_seconds":-5}`, `{"completed":"yes"}`, `{"completed":null}`, `{"id":"x"}`, `{"notes":5}`,
		`{"notes":"a\u0000b"}`, `{"assist_band":"` + strings.Repeat("a", 51) + `"}`,
	} {
		if rec := req(h, userA, "PATCH", path, b); rec.Code != 400 {
			t.Errorf("PATCH %q = %d %s", b, rec.Code, rec.Body)
		}
	}
	// explicit null on nullable fields is valid syntax (reaches the repo -> 404 for the unknown session)
	if rec := req(h, userA, "PATCH", path, `{"notes":null,"assist_band":null}`); rec.Code != 404 {
		t.Errorf("null notes = %d", rec.Code)
	}
	if rec := req(h, userA, "PATCH", "/sessions/"+badUID+"/sets/"+uid(2), `{"completed":true}`); rec.Code != 404 {
		t.Errorf("bad session id = %d", rec.Code)
	}
}

func TestListSessionsParamsAndCursor(t *testing.T) {
	repo := newMem()
	svc := NewService(repo)
	ctx := context.Background()
	for _, p := range []SessionParams{{Status: "x"}, {Limit: "0"}, {Limit: "101"}, {Limit: "a"}, {Cursor: "!!"}, {Cursor: "bm9wZQ"}, {From: "x"}} {
		if _, err := svc.ListSessions(ctx, userA, p); !errors.Is(err, ErrValidation) {
			t.Errorf("%+v: err = %v", p, err)
		}
	}
	// 3 rows with limit 2 -> next_cursor round-trips into the filter
	for i := 0; i < 3; i++ {
		_, _ = repo.StartSession(ctx, userA, nil)
	}
	page, err := svc.ListSessions(ctx, userA, SessionParams{Limit: "2", Status: "in_progress"})
	if err != nil || len(page.Items) != 2 || page.NextCursor == nil {
		t.Fatalf("page = %+v err=%v", page, err)
	}
	if repo.gotF.Limit != 2 || repo.gotF.Status != "in_progress" {
		t.Errorf("filter = %+v", repo.gotF)
	}
	c := encodeCursor(time.Date(2026, 11, 17, 1, 2, 3, 456789000, time.UTC), uid(7))
	if _, err := svc.ListSessions(ctx, userA, SessionParams{Cursor: c}); err != nil {
		t.Fatal(err)
	}
	if repo.gotF.AfterStarted == nil || repo.gotF.AfterStarted.Nanosecond() != 456789000 || repo.gotF.AfterID != uid(7) {
		t.Errorf("cursor not decoded: %+v", repo.gotF)
	}
}

func TestRecordsEmptyIsArray(t *testing.T) {
	rec := req(router(newMem()), userA, "GET", "/records", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"items":[]}` {
		t.Errorf("records = %d %s", rec.Code, rec.Body)
	}
}

func TestInternalErrorsAre500(t *testing.T) {
	h := router(failRepo{})
	rec := req(h, userA, "GET", "/plans/"+uid(1), "")
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("= %d %s", rec.Code, rec.Body)
	}
}

type failRepo struct{ Repository }

func (failRepo) GetPlan(context.Context, string, string) (Plan, error) {
	return Plan{}, errors.New("boom")
}
