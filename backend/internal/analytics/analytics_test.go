package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/auth"
)

const (
	userA = "aaaaaaaa-0000-0000-0000-000000000001"
	userB = "bbbbbbbb-0000-0000-0000-000000000002"
	exID  = "11111111-1111-1111-1111-111111111111"
)

// memRepo returns data only for userA, so any leak to userB would show as non-empty output.
type memRepo struct {
	since    *time.Time
	gotUser  string
	existing map[string]bool
}

func (m *memRepo) Summary(_ context.Context, u string, since *time.Time) (int, int, []WeekCount, error) {
	m.gotUser, m.since = u, since
	if u != userA {
		return 0, 0, nil, nil
	}
	return 2, 9, []WeekCount{{"2026-11-02", 1}, {"2026-11-16", 1}}, nil
}
func (m *memRepo) ExerciseExists(_ context.Context, id string) (bool, error) {
	return m.existing[id], nil
}
func (m *memRepo) Progress(_ context.Context, u, _ string, since *time.Time) ([]ProgressPoint, error) {
	m.gotUser, m.since = u, since
	if u != userA {
		return nil, nil
	}
	r := 12
	return []ProgressPoint{{Date: "2026-11-02", BestReps: &r, TotalSets: 3}}, nil
}
func (m *memRepo) MuscleFrequency(_ context.Context, u string, since *time.Time) ([]MuscleFrequency, error) {
	m.gotUser, m.since = u, since
	if u != userA {
		return nil, nil
	}
	return []MuscleFrequency{{"pectoralis_major", "chest", 2}}, nil
}

var fixedNow = time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC) // Thursday

func setup() (http.Handler, *memRepo) {
	m := &memRepo{existing: map[string]bool{exID: true}}
	svc := NewService(m)
	svc.now = func() time.Time { return fixedNow }
	h := NewHandler(svc)
	r := chi.NewRouter()
	r.Get("/analytics/summary", h.Summary)
	r.Get("/analytics/exercises/{id}/progress", h.Progress)
	r.Get("/analytics/muscle-frequency", h.MuscleFrequency)
	return r, m
}

func get(h http.Handler, user, path string) *httptest.ResponseRecorder {
	rq := httptest.NewRequest(http.MethodGet, path, nil)
	rq = rq.WithContext(auth.WithUserID(rq.Context(), user))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, rq)
	return rec
}

func TestRangeValidation(t *testing.T) {
	h, _ := setup()
	for _, p := range []string{"/analytics/summary", "/analytics/muscle-frequency", "/analytics/exercises/" + exID + "/progress"} {
		for _, bad := range []string{"1d", "30D", "abc", "-7d", "7d,30d", "%20"} {
			rec := get(h, userA, p+"?range="+bad)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"bad_request"`) {
				t.Errorf("%s?range=%s = %d %s", p, bad, rec.Code, rec.Body)
			}
		}
		for _, ok := range []string{"", "7d", "30d", "90d", "1y", "all"} {
			q := ""
			if ok != "" {
				q = "?range=" + ok
			}
			if rec := get(h, userA, p+q); rec.Code != http.StatusOK {
				t.Errorf("%s%s = %d %s", p, q, rec.Code, rec.Body)
			}
		}
	}
}

func TestRangeBounds(t *testing.T) {
	h, m := setup()
	for rng, want := range map[string]string{"7d": "2026-11-12", "": "2026-10-20", "90d": "2026-08-21", "1y": "2025-11-19"} {
		get(h, userA, "/analytics/muscle-frequency?range="+rng)
		if m.since == nil || m.since.Format(dateLayout) != want {
			t.Errorf("range %q since = %v, want %s", rng, m.since, want)
		}
	}
	get(h, userA, "/analytics/muscle-frequency?range=all")
	if m.since != nil {
		t.Errorf("range all since = %v, want nil", m.since)
	}
}

func TestSummaryShapeAndWeekFill(t *testing.T) {
	h, _ := setup()
	rec := get(h, userA, "/analytics/summary?range=30d")
	var out Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.TotalWorkouts != 2 || out.CompletedSets != 9 {
		t.Errorf("totals = %+v", out)
	}
	// 30d before Thu 2026-11-19 = 2026-10-20 -> week of Mon 2026-10-19 .. Mon 2026-11-16.
	want := []string{"2026-10-19", "2026-10-26", "2026-11-02", "2026-11-09", "2026-11-16"}
	if len(out.WeeklyFrequency) != len(want) {
		t.Fatalf("weeks = %+v", out.WeeklyFrequency)
	}
	for i, w := range out.WeeklyFrequency {
		if w.WeekStart != want[i] {
			t.Errorf("week %d = %s, want %s", i, w.WeekStart, want[i])
		}
	}
	if out.WeeklyFrequency[2].Workouts != 1 || out.WeeklyFrequency[3].Workouts != 0 {
		t.Errorf("counts = %+v", out.WeeklyFrequency)
	}
}

func TestEmptyResultsAreArrays(t *testing.T) {
	h, _ := setup()
	for path, key := range map[string]string{
		"/analytics/summary":                         `"weekly_frequency":[]`,
		"/analytics/muscle-frequency":                `"items":[]`,
		"/analytics/exercises/" + exID + "/progress": `"series":[]`,
	} {
		rec := get(h, userB, path+"?range=all")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), key) {
			t.Errorf("%s = %d %s, want %s", path, rec.Code, rec.Body, key)
		}
	}
}

func TestOwnershipScoping(t *testing.T) {
	h, m := setup()
	for _, p := range []string{"/analytics/summary", "/analytics/muscle-frequency", "/analytics/exercises/" + exID + "/progress"} {
		a := get(h, userA, p)
		b := get(h, userB, p)
		if m.gotUser != userB {
			t.Errorf("%s: repo called with %q, want the caller's id", p, m.gotUser)
		}
		if a.Body.String() == b.Body.String() {
			t.Errorf("%s: user B received user A's data: %s", p, b.Body)
		}
		if strings.Contains(b.Body.String(), "pectoralis") || strings.Contains(b.Body.String(), `"best_reps":12`) || strings.Contains(b.Body.String(), `"total_workouts":2`) {
			t.Errorf("%s leaked: %s", p, b.Body)
		}
	}
}

func TestProgressNotFound(t *testing.T) {
	h, _ := setup()
	for _, id := range []string{"not-a-uuid", "22222222-2222-2222-2222-222222222222"} {
		rec := get(h, userA, "/analytics/exercises/"+id+"/progress")
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"not_found"`) {
			t.Errorf("%s = %d %s", id, rec.Code, rec.Body)
		}
	}
}
