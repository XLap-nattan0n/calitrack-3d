"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api/client";
import type { Plan, Session, SessionSet, SetPatch } from "@/lib/api/types";
import { messages } from "@/constants/messages";
import { Button } from "@/components/ui/button";
import { HoldTimer, RestTimer } from "./timer";

const m = messages.workout;
const DEFAULT_REST_SECONDS = 60;
const BANDS = messages.planner.assistOptions;

const isoDate = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
const errMsg = (e: unknown, fallback: string) => (e instanceof Error ? e.message : fallback);

function ProgressBar({ percent }: { percent: number }) {
  const v = Math.min(Math.max(percent, 0), 100);
  return (
    <div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-muted" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(v)} aria-label={m.progress(v)}>
        <div className="h-full bg-primary transition-all" style={{ width: `${v}%` }} />
      </div>
      <p className="mt-1 text-sm text-muted-foreground">{m.progress(v)}</p>
    </div>
  );
}

function SetRow({
  set, onSave, onCompleted,
}: {
  set: SessionSet;
  onSave: (patch: SetPatch) => Promise<boolean>;
  onCompleted: () => void;
}) {
  const hold = set.target_reps == null;
  const [reps, setReps] = useState(set.actual_reps?.toString() ?? "");
  const [secs, setSecs] = useState(set.duration_seconds?.toString() ?? "");
  const [notes, setNotes] = useState(set.notes ?? "");
  const [done, setDone] = useState(set.completed);
  const id = `set-${set.id}`;

  const numPatch = (): SetPatch => {
    const p: SetPatch = {};
    if (!hold && reps !== "") p.actual_reps = Math.max(0, Math.trunc(Number(reps)));
    if (hold && secs !== "") p.duration_seconds = Math.max(0, Math.trunc(Number(secs)));
    return p;
  };

  async function toggle(next: boolean) {
    setDone(next);
    const ok = await onSave({ ...numPatch(), completed: next });
    if (!ok) setDone(!next);
    else if (next) onCompleted();
  }

  return (
    <li className="grid gap-2 rounded-md border p-3 sm:grid-cols-[auto_1fr_1fr_1fr_auto] sm:items-end">
      <div className="flex items-center gap-2 sm:pb-2">
        <input id={`${id}-done`} type="checkbox" className="h-5 w-5" checked={done} onChange={(e) => toggle(e.target.checked)} aria-label={`${m.done} - ${m.set(set.set_number)}`} />
        <span className="text-sm font-medium">{m.set(set.set_number)}</span>
      </div>
      {hold ? (
        <label className="text-sm" htmlFor={`${id}-secs`}>
          {m.seconds}
          <input id={`${id}-secs`} type="number" inputMode="numeric" min={0} className="mt-1 h-10 w-full rounded-md border bg-background px-2"
            value={secs} onChange={(e) => setSecs(e.target.value)} onBlur={() => secs !== "" && onSave(numPatch())} />
        </label>
      ) : (
        <label className="text-sm" htmlFor={`${id}-reps`}>
          {m.reps} <span className="text-muted-foreground">({m.target(set.target_reps ?? 0)})</span>
          <input id={`${id}-reps`} type="number" inputMode="numeric" min={0} className="mt-1 h-10 w-full rounded-md border bg-background px-2"
            value={reps} onChange={(e) => setReps(e.target.value)} onBlur={() => reps !== "" && onSave(numPatch())} />
        </label>
      )}
      <label className="text-sm" htmlFor={`${id}-band`}>
        {m.band}
        <select id={`${id}-band`} className="mt-1 h-10 w-full rounded-md border bg-background px-2" defaultValue={set.assist_band ?? ""}
          onChange={(e) => onSave({ assist_band: e.target.value || null })}>
          <option value="">{messages.planner.assistNone}</option>
          {BANDS.map((b) => <option key={b} value={b}>{b}</option>)}
        </select>
      </label>
      <label className="text-sm" htmlFor={`${id}-notes`}>
        {m.notes}
        <input id={`${id}-notes`} type="text" className="mt-1 h-10 w-full rounded-md border bg-background px-2" value={notes}
          onChange={(e) => setNotes(e.target.value)} onBlur={() => notes !== (set.notes ?? "") && onSave({ notes: notes || null })} />
      </label>
      {hold && <div className="sm:col-span-5"><HoldTimer onUse={(s) => { setSecs(String(s)); onSave({ duration_seconds: s }); }} /></div>}
    </li>
  );
}

function SessionView({ initial, onFinished }: { initial: Session; onFinished: (s: Session) => void }) {
  const [session] = useState(initial);
  const [progress, setProgress] = useState(initial.progress_percent);
  const [restKey, setRestKey] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [finishing, setFinishing] = useState(false);
  const [notes, setNotes] = useState("");
  const [completedIds, setCompletedIds] = useState(
    () => new Set(initial.exercises.flatMap((e) => e.sets.filter((s) => s.completed).map((s) => s.id))),
  );
  const total = session.exercises.reduce((n, e) => n + e.sets.length, 0);

  const save = useCallback(async (setId: string, patch: SetPatch) => {
    setError(null);
    try {
      const r = await api.updateSet(session.id, setId, patch);
      setProgress(r.progress_percent); // authoritative value from backend
      setCompletedIds((prev) => {
        const next = new Set(prev);
        if (r.set.completed) next.add(setId); else next.delete(setId);
        return next;
      });
      return true;
    } catch (e) {
      setError(`${m.saveFailed} ${errMsg(e, "")}`.trim());
      return false;
    }
  }, [session.id]);

  async function finish() {
    const remaining = total - completedIds.size;
    if (remaining > 0 && !window.confirm(m.finishUnfinished(remaining))) return;
    setFinishing(true);
    setError(null);
    try {
      onFinished(await api.finishSession(session.id, notes.trim() || undefined));
    } catch (e) {
      setError(errMsg(e, messages.errors.generic));
      setFinishing(false);
    }
  }

  return (
    <div className="space-y-6">
      <div className="sticky top-0 z-10 space-y-2 bg-background/95 py-2 backdrop-blur">
        <ProgressBar percent={progress} />
        {restKey > 0 && <RestTimer seconds={DEFAULT_REST_SECONDS} startKey={restKey} />}
      </div>
      {error && <p role="alert" className="rounded-md bg-red-50 p-2 text-sm text-red-700">{error}</p>}
      {session.exercises.map((ex) => (
        <section key={ex.id} aria-labelledby={`ex-${ex.id}`} className="space-y-2">
          <h2 id={`ex-${ex.id}`} className="text-lg font-semibold">{ex.exercise_name || m.exerciseFallback}</h2>
          <ul className="space-y-2">
            {ex.sets.map((s) => (
              <SetRow key={s.id} set={s} onSave={(p) => save(s.id, p)} onCompleted={() => setRestKey((k) => k + 1)} />
            ))}
          </ul>
        </section>
      ))}
      <div className="space-y-2">
        <label className="block text-sm" htmlFor="session-notes">
          {m.sessionNotes}
          <textarea id="session-notes" className="mt-1 w-full rounded-md border bg-background p-2" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} />
        </label>
        <Button onClick={finish} disabled={finishing}>{finishing ? m.finishing : m.finish}</Button>
      </div>
    </div>
  );
}

export function WorkoutSession() {
  const [session, setSession] = useState<Session | null>(null);
  const [finished, setFinished] = useState<Session | null>(null);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [resumed, setResumed] = useState(false);
  const [loading, setLoading] = useState(true);
  const [starting, setStarting] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const active = await api.getActiveSession();
      if (active) {
        setSession(active);
        setResumed(true);
      } else {
        // Include recent past plans (last 30 days) so missed, unstarted plans can still be started.
        const from = new Date();
        from.setDate(from.getDate() - 30);
        const to = new Date();
        to.setDate(to.getDate() + 14);
        const items = (await api.listPlans(isoDate(from), isoDate(to))).items;
        setPlans(items.filter((p) => p.status === "planned"));
      }
    } catch (e) {
      setError(errMsg(e, messages.errors.load));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  async function start(planId: string) {
    setStarting(planId);
    setError(null);
    try {
      setSession(await api.startSession(planId));
    } catch (e) {
      setError(errMsg(e, messages.errors.generic));
    } finally {
      setStarting(null);
    }
  }

  if (loading) return <p>{messages.library.loading}</p>;

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">{m.title}</h1>
      {error && (
        <div role="alert" className="rounded-md bg-red-50 p-2 text-sm text-red-700">
          {error} <Button size="sm" variant="outline" onClick={load}>{messages.errors.retry}</Button>
        </div>
      )}
      {finished ? (
        <div className="space-y-3" role="status">
          <p>{m.finished}</p>
          <Button asChild><Link href={`/history/${finished.id}`}>{m.viewHistory}</Link></Button>
        </div>
      ) : session ? (
        <>
          {resumed && <p className="rounded-md bg-muted p-2 text-sm" role="status">{m.resumed}</p>}
          <SessionView initial={session} onFinished={setFinished} />
        </>
      ) : (
        <section className="space-y-3">
          <h2 className="text-lg font-semibold">{m.pickPlan}</h2>
          {plans.length === 0 && !error ? (
            <div className="space-y-2">
              <p className="text-muted-foreground">{m.noPlans}</p>
              <Button asChild variant="outline"><Link href="/planner">{m.goPlanner}</Link></Button>
            </div>
          ) : (
            <ul className="space-y-2">
              {plans.map((p) => (
                <li key={p.id} className="flex flex-wrap items-center justify-between gap-2 rounded-md border p-3">
                  <div>
                    <p className="font-medium">{p.title}</p>
                    <p className="text-sm text-muted-foreground">{p.plan_date} · {messages.labels.category[p.category]} · {messages.library.results(p.exercises.length)}</p>
                  </div>
                  <Button size="sm" onClick={() => start(p.id)} disabled={starting !== null}>{starting === p.id ? m.starting : m.start}</Button>
                </li>
              ))}
            </ul>
          )}
        </section>
      )}
    </div>
  );
}
