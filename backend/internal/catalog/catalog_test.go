package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/auth"
)

type fakeRepo struct {
	mine     []Equipment
	rows     []ExerciseSummary
	gotF     Filter
	replaced []string
	getErr   error
}

func (f *fakeRepo) ListEquipment(context.Context) ([]Equipment, error)         { return f.mine, nil }
func (f *fakeRepo) ListMuscleGroups(context.Context) ([]MuscleGroup, error)    { return nil, nil }
func (f *fakeRepo) ListMuscles(context.Context) ([]Muscle, error)              { return nil, nil }
func (f *fakeRepo) UserEquipment(context.Context, string) ([]Equipment, error) { return f.mine, nil }
func (f *fakeRepo) ReplaceUserEquipment(_ context.Context, _ string, c []string) error {
	f.replaced = c
	return nil
}
func (f *fakeRepo) SearchExercises(_ context.Context, fl Filter) ([]ExerciseSummary, error) {
	f.gotF = fl
	return f.rows, nil
}
func (f *fakeRepo) GetExercise(context.Context, string) (ExerciseDetail, error) {
	return ExerciseDetail{}, f.getErr
}

func rowsN(n int) []ExerciseSummary {
	out := make([]ExerciseSummary, n)
	for i := range out {
		out[i] = ExerciseSummary{Slug: "s" + string(rune('a'+i)), Difficulty: 2}
	}
	return out
}

func TestSearchDefaultsToUserEquipment(t *testing.T) {
	repo := &fakeRepo{mine: []Equipment{{Code: "pullup_bar"}}}
	if _, err := NewService(repo).SearchExercises(context.Background(), "u", SearchParams{}); err != nil {
		t.Fatal(err)
	}
	if len(repo.gotF.Equipment) != 1 || repo.gotF.Equipment[0] != "pullup_bar" || repo.gotF.Limit != DefaultLimit {
		t.Errorf("filter = %+v", repo.gotF)
	}
}

func TestSearchExplicitEquipmentOverridesDefault(t *testing.T) {
	repo := &fakeRepo{mine: []Equipment{{Code: "pullup_bar"}}}
	_, err := NewService(repo).SearchExercises(context.Background(), "u", SearchParams{Equipment: []string{"parallel_bars", "parallel_bars"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.gotF.Equipment) != 1 || repo.gotF.Equipment[0] != "parallel_bars" {
		t.Errorf("equipment = %v", repo.gotF.Equipment)
	}
}

func TestSearchPaginationCursor(t *testing.T) {
	repo := &fakeRepo{rows: rowsN(3)} // limit 2 -> 3 rows means more pages
	svc := NewService(repo)
	page, err := svc.SearchExercises(context.Background(), "u", SearchParams{Limit: "2", Equipment: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.NextCursor == nil {
		t.Fatalf("page = %+v", page)
	}
	if repo.gotF.Limit != 2 {
		t.Errorf("limit = %d", repo.gotF.Limit)
	}
	if _, err := svc.SearchExercises(context.Background(), "u", SearchParams{Cursor: *page.NextCursor, Equipment: []string{}}); err != nil {
		t.Fatal(err)
	}
	if repo.gotF.AfterDiff != 2 || repo.gotF.AfterSlug != "sb" {
		t.Errorf("after = %d %q", repo.gotF.AfterDiff, repo.gotF.AfterSlug)
	}
	last, _ := svc.SearchExercises(context.Background(), "u", SearchParams{Limit: "5", Equipment: []string{}})
	if last.NextCursor != nil {
		t.Error("expected no next cursor on last page")
	}
}

func TestSearchValidation(t *testing.T) {
	svc := NewService(&fakeRepo{})
	bad := []SearchParams{
		{Category: "nope"}, {Difficulty: "0"}, {Difficulty: "6"}, {Difficulty: "x"},
		{Limit: "0"}, {Limit: "101"}, {Limit: "abc"}, {Role: "tertiary"},
		{Equipment: []string{"jetpack"}}, {Cursor: "!!!"}, {Cursor: "bm9jb2xvbg"}, {Q: strings.Repeat("a", 101)},
	}
	for i, p := range bad {
		_, err := svc.SearchExercises(context.Background(), "u", p)
		if !errors.Is(err, ErrValidation) {
			t.Errorf("case %d (%+v): err = %v, want validation", i, p, err)
		}
	}
}

func TestSetMyEquipment(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	if _, err := svc.SetMyEquipment(context.Background(), "u", []string{"pullup_bar", "pullup_bar", "bodyweight"}); err != nil {
		t.Fatal(err)
	}
	if len(repo.replaced) != 2 {
		t.Errorf("replaced = %v", repo.replaced)
	}
	if _, err := svc.SetMyEquipment(context.Background(), "u", []string{}); err != nil {
		t.Errorf("empty list should clear: %v", err)
	}
	for _, c := range [][]string{nil, {"rope"}} {
		if _, err := svc.SetMyEquipment(context.Background(), "u", c); !errors.Is(err, ErrValidation) {
			t.Errorf("%v: err = %v", c, err)
		}
	}
}

func call(h http.HandlerFunc, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req = req.WithContext(auth.WithUserID(req.Context(), "u"))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestHandlerMalformedInputIs400(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{}))
	for _, target := range []string{
		"/exercises?difficulty=abc", "/exercises?limit=-1", "/exercises?category=x",
		"/exercises?equipment=laser", "/exercises?cursor=zzz", "/exercises?role=boss",
	} {
		rec := call(h.ListExercises, http.MethodGet, target, "")
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"bad_request"`) {
			t.Errorf("%s = %d %s", target, rec.Code, rec.Body.String())
		}
	}
	for _, body := range []string{``, `{`, `[]`, `{"equipment_codes":"x"}`, `{"equipment_codes":null}`, `{"equipment_codes":["x"]}`, `{"other":1}`} {
		rec := call(h.SetMyEquipment, http.MethodPut, "/me/equipment", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("PUT %q = %d", body, rec.Code)
		}
	}
}

func TestHandlerEquipmentQueryForms(t *testing.T) {
	repo := &fakeRepo{}
	h := NewHandler(NewService(repo))
	rec := call(h.ListExercises, http.MethodGet, "/exercises?equipment=pullup_bar&equipment=resistance_bands,bodyweight", "")
	if rec.Code != http.StatusOK || len(repo.gotF.Equipment) != 3 {
		t.Errorf("code=%d equipment=%v", rec.Code, repo.gotF.Equipment)
	}
	var page ExercisePage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || page.Items == nil {
		t.Errorf("body = %s", rec.Body.String())
	}
	// "equipment=" explicitly means no extra equipment, not "use my default"
	repo.mine = []Equipment{{Code: "pullup_bar"}}
	call(h.ListExercises, http.MethodGet, "/exercises?equipment=", "")
	if len(repo.gotF.Equipment) != 0 {
		t.Errorf("empty equipment param fell back to default: %v", repo.gotF.Equipment)
	}
}

func TestHandlerGetExerciseNotFoundAndError(t *testing.T) {
	repo := &fakeRepo{getErr: ErrNotFound}
	h := NewHandler(NewService(repo))
	get := func() int {
		r := chi.NewRouter()
		r.Get("/exercises/{id}", h.GetExercise)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/exercises/abc", nil))
		return rec.Code
	}
	if c := get(); c != http.StatusNotFound {
		t.Errorf("not found = %d", c)
	}
	repo.getErr = errors.New("db down")
	if c := get(); c != http.StatusInternalServerError {
		t.Errorf("internal = %d", c)
	}
}
