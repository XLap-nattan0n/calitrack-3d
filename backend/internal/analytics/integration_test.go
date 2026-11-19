package analytics

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/db"
)

// Runs against a migrated + seeded Postgres (CI job supabase-db sets TEST_DATABASE_URL).
// Fixtures are created with the owner connection and removed again at the end.
func TestPostgresAnalytics(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	const a, b = "aaaaaaaa-0000-0000-0000-0000000000a1", "bbbbbbbb-0000-0000-0000-0000000000b2"
	cleanup := func() { _, _ = conn.Exec(ctx, `delete from auth.users where id in ($1::uuid, $2::uuid)`, a, b) }
	cleanup()
	t.Cleanup(cleanup)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec(`insert into auth.users (id, email) values ($1::uuid, 'int-a@example.com'), ($2::uuid, 'int-b@example.com')`, a, b)

	var exercise string
	if err := conn.QueryRow(ctx, `select e.id::text from public.exercises e
	  where exists (select 1 from public.exercise_muscles em where em.exercise_id = e.id) order by e.slug limit 1`).Scan(&exercise); err != nil {
		t.Fatalf("need a seeded exercise with muscles: %v", err)
	}
	var muscles int
	_ = conn.QueryRow(ctx, `select count(*) from public.exercise_muscles where exercise_id = $1::uuid`, exercise).Scan(&muscles)

	// session adds one finished/other session with the given started_at and (reps, completed) sets.
	type set struct {
		reps, hold int
		done       bool
	}
	session := func(user, status string, ago time.Duration, sets ...set) {
		t.Helper()
		var sid, seid string
		started := time.Now().UTC().Add(-ago)
		if err := conn.QueryRow(ctx, `insert into public.workout_sessions (user_id, status, started_at) values ($1::uuid, $2, $3) returning id::text`,
			user, status, started).Scan(&sid); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `insert into public.session_exercises (session_id, user_id, exercise_id, position) values ($1::uuid, $2::uuid, $3::uuid, 0) returning id::text`,
			sid, user, exercise).Scan(&seid); err != nil {
			t.Fatal(err)
		}
		for i, s := range sets {
			exec(`insert into public.session_sets (session_exercise_id, user_id, set_number, actual_reps, duration_seconds, completed)
			      values ($1::uuid, $2::uuid, $3, $4, $5, $6)`, seid, user, i+1, s.reps, s.hold, s.done)
		}
	}
	day := 24 * time.Hour
	session(a, "completed", 2*day, set{10, 0, true}, set{12, 30, true}, set{99, 99, false})
	session(a, "completed", 3*day, set{8, 0, true})
	session(a, "completed", 200*day, set{20, 0, true}) // outside 30d
	session(a, "abandoned", 1*day, set{50, 0, true})   // not counted
	session(b, "completed", 2*day, set{30, 0, true})

	svc := NewService(NewPGRepository(db.New(url)))

	sum, err := svc.Summary(ctx, a, "30d")
	if err != nil {
		t.Fatal(err)
	}
	if sum.TotalWorkouts != 2 || sum.CompletedSets != 3 {
		t.Errorf("A 30d summary = %+v, want 2 workouts / 3 sets", sum)
	}
	weekly := 0
	for _, w := range sum.WeeklyFrequency {
		weekly += w.Workouts
	}
	if weekly != 2 {
		t.Errorf("weekly sum = %d, want 2 (%+v)", weekly, sum.WeeklyFrequency)
	}
	if all, _ := svc.Summary(ctx, a, "all"); all.TotalWorkouts != 3 || all.CompletedSets != 4 {
		t.Errorf("A all summary = %+v", all)
	}
	if bs, _ := svc.Summary(ctx, b, "all"); bs.TotalWorkouts != 1 || bs.CompletedSets != 1 {
		t.Errorf("B summary = %+v", bs)
	}

	prog, err := svc.Progress(ctx, a, exercise, "30d")
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Series) != 2 {
		t.Fatalf("A progress = %+v, want 2 days", prog.Series)
	}
	for _, p := range prog.Series {
		if p.BestReps == nil || (*p.BestReps != 12 && *p.BestReps != 8) {
			t.Errorf("point %+v: incomplete sets / other users must not count", p)
		}
	}
	if pb, _ := svc.Progress(ctx, b, exercise, "30d"); len(pb.Series) != 1 || *pb.Series[0].BestReps != 30 {
		t.Errorf("B progress = %+v", pb.Series)
	}

	freq, err := svc.MuscleFrequency(ctx, a, "30d")
	if err != nil {
		t.Fatal(err)
	}
	if len(freq) != muscles {
		t.Errorf("muscle rows = %d, want %d", len(freq), muscles)
	}
	for _, f := range freq {
		if f.Sessions != 2 || f.GroupCode == "" {
			t.Errorf("muscle row %+v, want 2 sessions", f)
		}
	}
	if fb, _ := svc.MuscleFrequency(ctx, "cccccccc-0000-0000-0000-0000000000c3", "all"); len(fb) != 0 {
		t.Errorf("unknown user sees %+v", fb)
	}
}
