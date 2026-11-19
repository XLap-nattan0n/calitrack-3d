"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api/client";
import type { PersonalRecord, SessionSummary } from "@/lib/api/types";
import { messages } from "@/constants/messages";
import { Button } from "@/components/ui/button";

const h = messages.history;
export const fmtDateTime = (iso: string) => new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });

export function History() {
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [records, setRecords] = useState<PersonalRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (after?: string) => {
    setLoading(true);
    setError(null);
    try {
      const [s, r] = await Promise.all([api.listSessions(after), after ? Promise.resolve(null) : api.listRecords()]);
      setSessions((prev) => (after ? [...prev, ...s.items] : s.items));
      setCursor(s.next_cursor ?? null);
      if (r) setRecords(r.items);
    } catch (e) {
      setError(e instanceof Error ? e.message : messages.errors.load);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  return (
    <div className="space-y-8">
      <h1 className="text-2xl font-semibold">{h.title}</h1>
      {error && (
        <div role="alert" className="rounded-md bg-red-50 p-2 text-sm text-red-700">
          {error} <Button size="sm" variant="outline" onClick={() => load()}>{messages.errors.retry}</Button>
        </div>
      )}
      <section aria-labelledby="records-h" className="space-y-2">
        <h2 id="records-h" className="text-lg font-semibold">{h.records}</h2>
        {!loading && records.length === 0 ? <p className="text-muted-foreground">{h.noRecords}</p> : (
          <ul className="grid gap-2 sm:grid-cols-2">
            {records.map((r) => (
              <li key={r.exercise_id} className="rounded-md border p-3">
                <p className="font-medium">{r.exercise_name}</p>
                <p className="text-sm">
                  {[r.best_reps != null && h.bestReps(r.best_reps), r.best_hold_seconds != null && h.bestHold(r.best_hold_seconds)].filter(Boolean).join(" · ")}
                </p>
                <p className="text-xs text-muted-foreground">{h.achieved} {fmtDateTime(r.achieved_at)}</p>
              </li>
            ))}
          </ul>
        )}
      </section>
      <section aria-labelledby="sessions-h" className="space-y-2">
        <h2 id="sessions-h" className="text-lg font-semibold">{h.sessions}</h2>
        {!loading && sessions.length === 0 && !error && <p className="text-muted-foreground">{h.noSessions}</p>}
        <ul className="space-y-2">
          {sessions.map((s) => (
            <li key={s.id}>
              <Link href={s.status === "in_progress" ? "/workout" : `/history/${s.id}`} className="flex flex-wrap items-center justify-between gap-2 rounded-md border p-3 hover:bg-muted">
                <span>
                  <span className="block font-medium">{fmtDateTime(s.started_at)}</span>
                  <span className="text-sm text-muted-foreground">{h.status[s.status]} · {messages.workout.progress(s.progress_percent)}</span>
                </span>
                <span className="text-sm underline">{s.status === "in_progress" ? h.resume : messages.library.view}</span>
              </Link>
            </li>
          ))}
        </ul>
        {loading && <p>{messages.library.loading}</p>}
        {cursor && !loading && <Button variant="outline" onClick={() => load(cursor)}>{h.loadMore}</Button>}
      </section>
    </div>
  );
}
