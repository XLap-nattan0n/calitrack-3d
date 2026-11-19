"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { api, ApiError } from "@/lib/api/client";
import type { Session } from "@/lib/api/types";
import { messages } from "@/constants/messages";
import { fmtDateTime } from "./history";

const h = messages.history;

export function SessionDetail({ id }: { id: string }) {
  const [session, setSession] = useState<Session | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    api.getSession(id).then(
      (s) => alive && setSession(s),
      (e) => alive && setError(e instanceof ApiError && e.status === 404 ? h.notFound : messages.errors.load),
    );
    return () => { alive = false; };
  }, [id]);

  const back = <Link href="/history" className="text-sm underline">{h.back}</Link>;
  if (error) return <div className="space-y-2">{back}<p role="alert">{error}</p></div>;
  if (!session) return <p>{messages.library.loading}</p>;

  const all = session.exercises.flatMap((e) => e.sets);
  const done = all.filter((s) => s.completed).length;
  const mins = session.finished_at ? Math.max(1, Math.round((+new Date(session.finished_at) - +new Date(session.started_at)) / 60000)) : null;

  return (
    <div className="space-y-6">
      {back}
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold">{fmtDateTime(session.started_at)}</h1>
        <p className="text-sm text-muted-foreground">
          {h.status[session.status]} · {h.setsOf(done, all.length)} · {messages.workout.progress(session.progress_percent)}
          {mins != null && ` · ${h.duration} ${mins} min`}
        </p>
        {session.notes && <p className="text-sm"><span className="font-medium">{h.notes}:</span> {session.notes}</p>}
      </header>
      {session.exercises.map((ex) => (
        <section key={ex.id} aria-labelledby={`ex-${ex.id}`} className="space-y-2">
          <h2 id={`ex-${ex.id}`} className="text-lg font-semibold">{ex.exercise_name || messages.workout.exerciseFallback}</h2>
          <ul className="space-y-1 text-sm">
            {ex.sets.map((s) => (
              <li key={s.id} className="flex flex-wrap gap-x-3 rounded-md border p-2">
                <span className="font-medium">{messages.workout.set(s.set_number)}</span>
                <span>{s.completed ? "✓" : "–"}<span className="sr-only">{s.completed ? messages.workout.done : ""}</span></span>
                {s.target_reps != null && <span>{s.actual_reps ?? 0} / {s.target_reps} {messages.detail.reps}</span>}
                {s.duration_seconds != null && <span>{s.duration_seconds}s</span>}
                {s.assist_band && <span>{messages.workout.band}: {s.assist_band}</span>}
                {s.notes && <span className="text-muted-foreground">{s.notes}</span>}
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}
