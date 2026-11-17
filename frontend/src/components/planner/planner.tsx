"use client";
import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { messages } from "@/constants/messages";
import { api, MOCK_PLANS } from "@/lib/api/client";
import { useCatalog } from "@/lib/api/hooks";
import {
  PLAN_CATEGORIES, type ExerciseSummary, type PlanCategory, type PlanInput,
} from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";

const p = messages.planner;

interface Row {
  key: string;
  ex: ExerciseSummary;
  sets: string;
  reps: string;
  hold: string;
  rest: string;
  band: string;
}

const today = () => {
  const d = new Date(); // local date, not UTC
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};

function newRow(ex: ExerciseSummary): Row {
  return {
    key: `${ex.id}-${Math.random().toString(36).slice(2)}`,
    ex,
    sets: "3",
    reps: ex.exercise_type === "reps" ? "8" : "",
    hold: ex.exercise_type === "hold" ? "20" : "",
    rest: "60",
    band: "",
  };
}

/** "" -> null, otherwise integer; NaN signals invalid. */
function toInt(s: string): number | null {
  if (s.trim() === "") return null;
  const n = Number(s);
  return Number.isInteger(n) ? n : NaN;
}

export function buildPlanInput(
  date: string, title: string, category: PlanCategory, groupIds: number[], rows: Row[],
): PlanInput | string {
  if (!title.trim()) return p.errors.titleRequired;
  if (rows.length === 0) return p.errors.noExercises;
  const exercises = [];
  for (const r of rows) {
    const sets = toInt(r.sets), reps = toInt(r.reps), hold = toInt(r.hold), rest = toInt(r.rest);
    if (sets === null || Number.isNaN(sets) || sets < 1) return p.errors.invalidNumber;
    if ([reps, hold, rest].some((v) => v !== null && (Number.isNaN(v) || v < 0))) return p.errors.invalidNumber;
    exercises.push({
      exercise_id: r.ex.id, target_sets: sets, target_reps: reps, hold_seconds: hold, rest_seconds: rest,
      assist_band: r.band || null,
    });
  }
  return { plan_date: date, title: title.trim(), category, target_muscle_group_ids: groupIds, exercises };
}

export function Planner() {
  const catalog = useCatalog();
  const params = useSearchParams();
  const preset = params.get("add");
  const [date, setDate] = useState(today());
  const [title, setTitle] = useState("");
  const [category, setCategory] = useState<PlanCategory>("custom");
  const [groupIds, setGroupIds] = useState<number[]>([]);
  const [rows, setRows] = useState<Row[]>([]);
  const [search, setSearch] = useState("");
  const [results, setResults] = useState<ExerciseSummary[]>([]);
  const [searching, setSearching] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);

  // Preselect an exercise coming from the detail page (?add=<id>).
  useEffect(() => {
    if (!preset) return;
    api.getExercise(preset).then((e) => setRows((r) => (r.length ? r : [newRow(e)]))).catch(() => undefined);
  }, [preset]);

  useEffect(() => {
    const t = setTimeout(() => {
      setSearching(true);
      api.listExercises({ q: search, limit: 10 })
        .then((r) => setResults(r.items))
        .catch((e: Error) => setError(e.message))
        .finally(() => setSearching(false));
    }, 300);
    return () => clearTimeout(t);
  }, [search]);

  const addedIds = useMemo(() => new Set(rows.map((r) => r.ex.id)), [rows]);

  function update(key: string, patch: Partial<Row>) {
    setRows((rs) => rs.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  }
  function move(i: number, dir: -1 | 1) {
    setRows((rs) => {
      const j = i + dir;
      if (j < 0 || j >= rs.length) return rs;
      const next = [...rs];
      [next[i], next[j]] = [next[j], next[i]];
      return next;
    });
  }

  async function save() {
    setError(null);
    setStatus(null);
    const input = buildPlanInput(date, title, category, groupIds, rows);
    if (typeof input === "string") return setError(input);
    setSaving(true);
    try {
      if (MOCK_PLANS) {
        const key = "calitrack.mockPlans";
        const all = JSON.parse(localStorage.getItem(key) ?? "[]") as PlanInput[];
        localStorage.setItem(key, JSON.stringify([...all, input]));
        setStatus(p.savedMock);
      } else {
        await api.createPlan(input);
        setStatus(p.saved);
      }
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">{p.title}</h1>
        <p className="text-sm text-muted-foreground">{p.description}</p>
        {MOCK_PLANS && <p className="mt-2 rounded-md bg-muted p-2 text-sm">{p.mockBanner}</p>}
      </div>

      <section className="grid gap-4 rounded-lg border p-4 sm:grid-cols-2">
        <div className="space-y-1">
          <Label htmlFor="plan-date">{p.date}</Label>
          <Input id="plan-date" type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        </div>
        <div className="space-y-1">
          <Label htmlFor="plan-category">{p.category}</Label>
          <Select id="plan-category" value={category} onChange={(e) => setCategory(e.target.value as PlanCategory)}>
            {PLAN_CATEGORIES.map((c) => <option key={c} value={c}>{messages.labels.category[c]}</option>)}
          </Select>
        </div>
        <div className="space-y-1 sm:col-span-2">
          <Label htmlFor="plan-title">{p.planTitle}</Label>
          <Input id="plan-title" maxLength={120} value={title} onChange={(e) => setTitle(e.target.value)} placeholder={p.planTitlePlaceholder} />
        </div>
        <fieldset className="sm:col-span-2">
          <legend className="mb-2 text-sm font-medium">{p.muscleGroups}</legend>
          <div className="flex flex-wrap gap-x-6 gap-y-2">
            {(catalog.data?.groups ?? []).map((g) => (
              <label key={g.id} className="flex min-h-10 items-center gap-2 text-sm">
                <input
                  type="checkbox" className="h-4 w-4" checked={groupIds.includes(g.id)}
                  onChange={() => setGroupIds((ids) => (ids.includes(g.id) ? ids.filter((x) => x !== g.id) : [...ids, g.id]))}
                />
                {g.name_en}
              </label>
            ))}
          </div>
        </fieldset>
      </section>

      <section className="space-y-3 rounded-lg border p-4">
        <h2 className="text-lg font-medium">{p.addExercise}</h2>
        <Label htmlFor="plan-search" className="sr-only">{p.addExercise}</Label>
        <Input id="plan-search" type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder={p.addPlaceholder} />
        <ul className="max-h-60 divide-y overflow-y-auto rounded-md border" aria-busy={searching}>
          {results.map((ex) => (
            <li key={ex.id} className="flex items-center justify-between gap-2 p-2 text-sm">
              <span>{ex.name_en} <span className="text-muted-foreground">· {messages.labels.category[ex.category]}</span></span>
              <Button size="sm" variant="outline" disabled={addedIds.has(ex.id)} onClick={() => setRows((r) => [...r, newRow(ex)])}>
                {addedIds.has(ex.id) ? p.alreadyAdded : p.addExercise}
              </Button>
            </li>
          ))}
          {!searching && results.length === 0 && <li className="p-2 text-sm text-muted-foreground">{p.noResults}</li>}
        </ul>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-medium">{p.exercises} ({rows.length})</h2>
        {rows.length === 0 && <p className="text-sm text-muted-foreground">{p.emptyPlan}</p>}
        <ol className="space-y-3">
          {rows.map((r, i) => (
            <li key={r.key} className="rounded-lg border p-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="font-medium">{i + 1}. {r.ex.name_en}</p>
                <div className="flex gap-1">
                  <Button size="sm" variant="outline" onClick={() => move(i, -1)} disabled={i === 0} aria-label={`${p.moveUp}: ${r.ex.name_en}`}>↑</Button>
                  <Button size="sm" variant="outline" onClick={() => move(i, 1)} disabled={i === rows.length - 1} aria-label={`${p.moveDown}: ${r.ex.name_en}`}>↓</Button>
                  <Button size="sm" variant="ghost" onClick={() => setRows((rs) => rs.filter((x) => x.key !== r.key))} aria-label={`${p.remove}: ${r.ex.name_en}`}>{p.remove}</Button>
                </div>
              </div>
              <div className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-5">
                <Num id={`${r.key}-sets`} label={p.sets} value={r.sets} min={1} onChange={(v) => update(r.key, { sets: v })} />
                <Num id={`${r.key}-reps`} label={p.reps} value={r.reps} onChange={(v) => update(r.key, { reps: v })} />
                <Num id={`${r.key}-hold`} label={p.hold} value={r.hold} onChange={(v) => update(r.key, { hold: v })} />
                <Num id={`${r.key}-rest`} label={p.rest} value={r.rest} onChange={(v) => update(r.key, { rest: v })} />
                <div className="space-y-1">
                  <Label htmlFor={`${r.key}-band`}>{p.assistBand}</Label>
                  <Select id={`${r.key}-band`} value={r.band} onChange={(e) => update(r.key, { band: e.target.value })}>
                    <option value="">{p.assistNone}</option>
                    {p.assistOptions.map((o) => <option key={o} value={o.toLowerCase()}>{o}</option>)}
                  </Select>
                </div>
              </div>
            </li>
          ))}
        </ol>
      </section>

      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      {status && <p role="status" className="text-sm">{status}</p>}
      <Button onClick={save} disabled={saving}>{saving ? p.saving : p.save}</Button>
    </div>
  );
}

function Num({ id, label, value, min = 0, onChange }: { id: string; label: string; value: string; min?: number; onChange: (v: string) => void }) {
  return (
    <div className="space-y-1">
      <Label htmlFor={id}>{label}</Label>
      <Input id={id} type="number" inputMode="numeric" min={min} step={1} value={value} onChange={(e) => onChange(e.target.value)} />
    </div>
  );
}
