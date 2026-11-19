"use client";

import { useCallback, useEffect, useState } from "react";
import {
  Bar, BarChart, CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis,
} from "recharts";
import { api } from "@/lib/api/client";
import { useCatalog } from "@/lib/api/hooks";
import type {
  AnalyticsRange, AnalyticsSummary, ExerciseProgressPoint, MuscleFrequencyItem, PersonalRecord,
} from "@/lib/api/types";
import { messages } from "@/constants/messages";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";

const a = messages.analytics;
const RANGES = Object.keys(a.ranges) as AnalyticsRange[];
const C1 = "#2563eb";
const C2 = "#ea580c";
const shortDate = (d: string) => new Date(`${d}T00:00:00`).toLocaleDateString(undefined, { month: "short", day: "numeric" });

function Panel({ title, desc, children }: { title: string; desc?: string; children: React.ReactNode }) {
  return (
    <section aria-label={title} className="space-y-2 rounded-md border p-4">
      <div>
        <h2 className="text-lg font-semibold">{title}</h2>
        {desc && <p className="text-sm text-muted-foreground">{desc}</p>}
      </div>
      {children}
    </section>
  );
}

const Empty = ({ text }: { text: string }) => <p className="py-8 text-center text-sm text-muted-foreground">{text}</p>;

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-md border p-4">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="text-3xl font-semibold">{value}</p>
    </div>
  );
}

interface Data {
  summary: AnalyticsSummary;
  muscles: MuscleFrequencyItem[];
}

export function Analytics() {
  const [range, setRange] = useState<AnalyticsRange>("30d");
  const [data, setData] = useState<Data | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [records, setRecords] = useState<PersonalRecord[]>([]);
  const [exerciseId, setExerciseId] = useState("");
  const [series, setSeries] = useState<ExerciseProgressPoint[] | null>(null);
  const [seriesError, setSeriesError] = useState<string | null>(null);
  const catalog = useCatalog();

  const load = useCallback(() => {
    let alive = true;
    setData(null);
    setError(null);
    Promise.all([api.getAnalyticsSummary(range), api.getMuscleFrequency(range)]).then(
      ([summary, m]) => alive && setData({ summary, muscles: m.items }),
      (e) => alive && setError(e instanceof Error ? e.message : messages.errors.load),
    );
    return () => { alive = false; };
  }, [range]);
  useEffect(load, [load]);

  // Exercises the user has records for populate the selector.
  useEffect(() => {
    api.listRecords().then(
      (r) => {
        setRecords(r.items);
        setExerciseId((cur) => cur || r.items[0]?.exercise_id || "");
      },
      () => setRecords([]),
    );
  }, []);

  useEffect(() => {
    if (!exerciseId) return;
    let alive = true;
    setSeries(null);
    setSeriesError(null);
    api.getExerciseProgress(exerciseId, range).then(
      (r) => alive && setSeries(r.series),
      (e) => alive && setSeriesError(e instanceof Error ? e.message : messages.errors.load),
    );
    return () => { alive = false; };
  }, [exerciseId, range]);

  const muscleName = (code: string) => catalog.data?.muscles.find((m) => m.code === code)?.name_en ?? code;
  const muscleRows = (data?.muscles ?? [])
    .map((m) => ({ name: muscleName(m.muscle_code), sessions: m.sessions }))
    .sort((x, y) => y.sessions - x.sessions)
    .slice(0, 12);
  const weekly = (data?.summary.weekly_frequency ?? []).map((w) => ({ week: shortDate(w.week_start), workouts: w.workouts }));
  const points = (series ?? []).map((p) => ({ ...p, label: shortDate(p.date) }));
  const hasReps = points.some((p) => p.best_reps != null);
  const hasHold = points.some((p) => p.best_hold_seconds != null);

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold">{a.title}</h1>
          <p className="text-muted-foreground">{a.description}</p>
        </div>
        <label className="flex items-center gap-2 text-sm">
          {a.range}
          <Select value={range} onChange={(e) => setRange(e.target.value as AnalyticsRange)} className="w-40">
            {RANGES.map((r) => <option key={r} value={r}>{a.ranges[r]}</option>)}
          </Select>
        </label>
      </header>

      {error && (
        <div role="alert" className="rounded-md bg-red-50 p-2 text-sm text-red-700">
          {error} <Button size="sm" variant="outline" onClick={load}>{messages.errors.retry}</Button>
        </div>
      )}
      {!data && !error && <p role="status">{a.loading}</p>}

      {data && (
        <>
          <div className="grid grid-cols-2 gap-3">
            <Stat label={a.totalWorkouts} value={data.summary.total_workouts} />
            <Stat label={a.completedSets} value={data.summary.completed_sets} />
          </div>

          <Panel title={a.weeklyFrequency} desc={a.weeklyFrequencyDesc}>
            {weekly.length === 0 ? <Empty text={a.emptySummary} /> : (
              <div role="img" aria-label={a.chartLabel(a.weeklyFrequency)} className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={weekly} margin={{ left: -20, right: 8, top: 8 }}>
                    <CartesianGrid strokeDasharray="3 3" vertical={false} />
                    <XAxis dataKey="week" fontSize={12} />
                    <YAxis allowDecimals={false} fontSize={12} />
                    <Tooltip />
                    <Bar dataKey="workouts" name={a.workouts} fill={C1} radius={[4, 4, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            )}
          </Panel>

          <Panel title={a.progress} desc={a.progressDesc}>
            {records.length === 0 ? <Empty text={a.emptyNoExercises} /> : (
              <>
                <label className="flex items-center gap-2 text-sm">
                  {a.exercise}
                  <Select value={exerciseId} onChange={(e) => setExerciseId(e.target.value)} className="max-w-xs">
                    {records.map((r) => <option key={r.exercise_id} value={r.exercise_id}>{r.exercise_name}</option>)}
                  </Select>
                </label>
                {seriesError && <p role="alert" className="text-sm text-red-700">{seriesError}</p>}
                {!series && !seriesError && <p role="status">{a.loading}</p>}
                {series && points.length === 0 && <Empty text={a.emptyProgress} />}
                {series && points.length > 0 && (
                  <div role="img" aria-label={a.chartLabel(a.progress)} className="h-64">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={points} margin={{ left: -20, right: 8, top: 8 }}>
                        <CartesianGrid strokeDasharray="3 3" vertical={false} />
                        <XAxis dataKey="label" fontSize={12} />
                        <YAxis allowDecimals={false} fontSize={12} />
                        <Tooltip />
                        <Legend />
                        {hasReps && <Line type="monotone" dataKey="best_reps" name={a.bestReps} stroke={C1} strokeWidth={2} connectNulls />}
                        {hasHold && <Line type="monotone" dataKey="best_hold_seconds" name={a.bestHold} stroke={C2} strokeWidth={2} connectNulls />}
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </>
            )}
          </Panel>

          <Panel title={a.muscleFrequency} desc={a.muscleFrequencyDesc}>
            {muscleRows.length === 0 ? <Empty text={a.emptyMuscle} /> : (
              <div role="img" aria-label={a.chartLabel(a.muscleFrequency)} style={{ height: 40 + muscleRows.length * 28 }}>
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={muscleRows} layout="vertical" margin={{ left: 8, right: 8 }}>
                    <CartesianGrid strokeDasharray="3 3" horizontal={false} />
                    <XAxis type="number" allowDecimals={false} fontSize={12} />
                    <YAxis type="category" dataKey="name" width={96} fontSize={12} />
                    <Tooltip />
                    <Bar dataKey="sessions" name={a.sessions} fill={C1} radius={[0, 4, 4, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            )}
          </Panel>
        </>
      )}
    </div>
  );
}
