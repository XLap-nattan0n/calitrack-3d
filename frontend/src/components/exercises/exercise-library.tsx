"use client";
import { useCallback, useEffect, useMemo, useState } from "react";
import { messages } from "@/constants/messages";
import { api } from "@/lib/api/client";
import { useCatalog } from "@/lib/api/hooks";
import { CATEGORIES, type Category, type Equipment, type EquipmentCode, type ExerciseSummary } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { ExerciseCard } from "./exercise-card";

const m = messages.library;

export function ExerciseLibrary() {
  const catalog = useCatalog();
  const [mine, setMine] = useState<Equipment[] | null>(null);
  const [custom, setCustom] = useState<EquipmentCode[] | null>(null); // null = use saved equipment
  const [search, setSearch] = useState("");
  const [q, setQ] = useState("");
  const [category, setCategory] = useState<Category | "">("");
  const [difficulty, setDifficulty] = useState<number | "">("");
  const [muscle, setMuscle] = useState("");
  const [items, setItems] = useState<ExerciseSummary[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    api.getMyEquipment().then((r) => setMine(r.equipment)).catch((e: Error) => setError(e.message));
  }, []);
  useEffect(() => {
    const t = setTimeout(() => setQ(search), 300);
    return () => clearTimeout(t);
  }, [search]);

  const myCodes = useMemo(() => (mine ?? []).map((e) => e.code), [mine]);
  const checked = custom ?? myCodes;

  const run = useCallback(async (after?: string) => {
    setLoading(true);
    setError(null);
    try {
      const r = await api.listExercises({ q, category, difficulty, muscle, equipment: custom ?? undefined, cursor: after });
      setItems((prev) => (after ? [...prev, ...r.items] : r.items));
      setCursor(r.next_cursor ?? null);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [q, category, difficulty, muscle, custom]);

  useEffect(() => {
    if (mine === null && !error) return; // wait for saved equipment so the default filter is meaningful
    void run();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [run, mine]);

  const muscleNames = useMemo(
    () => Object.fromEntries((catalog.data?.muscles ?? []).map((x) => [x.code, x.name_en])),
    [catalog.data],
  );

  function toggle(code: EquipmentCode) {
    const base = checked;
    const next = base.includes(code) ? base.filter((c) => c !== code) : [...base, code];
    setCustom(next);
    setNote(null);
  }

  async function saveEquipment() {
    setSaving(true);
    try {
      const r = await api.setMyEquipment(checked);
      setMine(r.equipment);
      setCustom(null);
      setNote(m.equipmentSaved);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }

  function clear() {
    setSearch(""); setQ(""); setCategory(""); setDifficulty(""); setMuscle(""); setCustom(null);
  }

  const equipmentOptions = catalog.data?.equipment ?? [];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">{m.title}</h1>
        <p className="text-sm text-muted-foreground">{m.description}</p>
      </div>

      <form role="search" onSubmit={(e) => e.preventDefault()} className="grid gap-4 rounded-lg border p-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="space-y-1 sm:col-span-2 lg:col-span-4">
          <Label htmlFor="lib-search">{m.search}</Label>
          <Input id="lib-search" type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder={m.searchPlaceholder} />
        </div>
        <div className="space-y-1">
          <Label htmlFor="lib-category">{m.category}</Label>
          <Select id="lib-category" value={category} onChange={(e) => setCategory(e.target.value as Category | "")}>
            <option value="">{m.any}</option>
            {CATEGORIES.map((c) => <option key={c} value={c}>{messages.labels.category[c]}</option>)}
          </Select>
        </div>
        <div className="space-y-1">
          <Label htmlFor="lib-difficulty">{m.difficulty}</Label>
          <Select id="lib-difficulty" value={difficulty} onChange={(e) => setDifficulty(e.target.value ? Number(e.target.value) : "")}>
            <option value="">{m.any}</option>
            {[1, 2, 3, 4, 5].map((n) => <option key={n} value={n}>{messages.labels.difficulty(n)}</option>)}
          </Select>
        </div>
        <div className="space-y-1 sm:col-span-2">
          <Label htmlFor="lib-muscle">{m.muscle}</Label>
          <Select id="lib-muscle" value={muscle} onChange={(e) => setMuscle(e.target.value)}>
            <option value="">{m.any}</option>
            {(catalog.data?.muscles ?? []).map((x) => <option key={x.code} value={x.code}>{x.name_en}</option>)}
          </Select>
        </div>
        <fieldset className="sm:col-span-2 lg:col-span-4">
          <legend className="mb-2 text-sm font-medium">{m.equipment}</legend>
          <div className="flex flex-wrap gap-x-6 gap-y-2">
            {equipmentOptions.map((e) => (
              <label key={e.code} className="flex min-h-10 items-center gap-2 text-sm">
                <input type="checkbox" className="h-4 w-4" checked={checked.includes(e.code)} onChange={() => toggle(e.code)} />
                {e.name_en}
              </label>
            ))}
          </div>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            {custom === null ? (
              <p className="text-sm text-muted-foreground">{myCodes.length ? m.usingMyEquipment : m.noEquipmentSaved}</p>
            ) : (
              <>
                <Button type="button" size="sm" variant="outline" onClick={() => setCustom(null)}>{m.useMyEquipment}</Button>
                <Button type="button" size="sm" onClick={saveEquipment} disabled={saving}>{saving ? m.savingEquipment : m.saveMyEquipment}</Button>
              </>
            )}
            {note && <p role="status" className="text-sm text-muted-foreground">{note}</p>}
          </div>
        </fieldset>
        <div className="sm:col-span-2 lg:col-span-4">
          <Button type="button" variant="ghost" size="sm" onClick={clear}>{m.clear}</Button>
        </div>
      </form>

      {error && (
        <div role="alert" className="flex items-center gap-3 rounded-md border border-destructive p-3 text-sm text-destructive">
          {error}
          <Button size="sm" variant="outline" onClick={() => void run()}>{messages.errors.retry}</Button>
        </div>
      )}

      <p aria-live="polite" className="text-sm text-muted-foreground">
        {loading && items.length === 0 ? m.loading : m.results(items.length) + (cursor ? "+" : "")}
      </p>
      {!loading && !error && items.length === 0 && <p>{m.empty}</p>}

      <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {items.map((ex) => <li key={ex.id}><ExerciseCard ex={ex} muscleNames={muscleNames} /></li>)}
      </ul>
      {cursor && (
        <Button variant="outline" onClick={() => void run(cursor)} disabled={loading}>{loading ? m.loading : m.loadMore}</Button>
      )}
    </div>
  );
}
