"use client";
import dynamic from "next/dynamic";
import Link from "next/link";
import { Component, useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { messages } from "@/constants/messages";
import { HIGHLIGHT_COLORS, type Highlight, type ViewPreset } from "@/constants/anatomy";
import { api } from "@/lib/api/client";
import { useCatalog } from "@/lib/api/hooks";
import type { ExerciseSummary } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";

const m = messages.anatomy;

const MuscleCanvas = dynamic(() => import("./muscle-canvas"), { ssr: false });

class CanvasBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  render() {
    return this.state.failed
      ? <p role="alert" className="flex h-full items-center justify-center p-4 text-center text-sm text-destructive">{m.modelError}</p>
      : this.props.children;
  }
}

function Swatch({ color, label }: { color: string; label: string }) {
  return (
    <li className="flex items-center gap-2">
      <span aria-hidden className="inline-block h-3 w-3 rounded-full border" style={{ backgroundColor: color }} />
      {label}
    </li>
  );
}

export function AnatomyViewer() {
  const catalog = useCatalog();
  const [meshes, setMeshes] = useState<Set<string> | null>(null);
  const [view, setView] = useState<ViewPreset>("front");
  const [viewNonce, setViewNonce] = useState(0);
  const [muscle, setMuscle] = useState<string | null>(null);
  const [exercise, setExercise] = useState<ExerciseSummary | null>(null);
  const [items, setItems] = useState<ExerciseSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const muscles = catalog.data?.muscles;
  const names = useMemo(() => Object.fromEntries((muscles ?? []).map((x) => [x.code, x.name_en])), [muscles]);
  const meshOf = useMemo(() => Object.fromEntries((muscles ?? []).map((x) => [x.mesh_name, x.code])), [muscles]);
  const hasMesh = useCallback((code: string) => meshes?.has(code) ?? false, [meshes]);
  const allEquipment = catalog.data?.equipment;

  // muscles the DB knows but the GLB has no mesh for (never faked with a placeholder)
  const noModel = useMemo(() => (meshes && muscles ? muscles.filter((x) => !meshes.has(x.mesh_name)) : []), [meshes, muscles]);
  const withModel = useMemo(() => (meshes && muscles ? muscles.filter((x) => meshes.has(x.mesh_name)) : []), [meshes, muscles]);

  useEffect(() => {
    if (!muscle || !allEquipment) { setItems([]); return; }
    let live = true;
    setLoading(true);
    setError(null);
    // browse everything: pass all equipment so the user's saved-equipment default doesn't hide exercises
    api.listExercises({ muscle, equipment: allEquipment.map((e) => e.code), limit: 50 })
      .then((r) => live && setItems(r.items))
      .catch((e: Error) => live && setError(e.message))
      .finally(() => live && setLoading(false));
    return () => { live = false; };
  }, [muscle, allEquipment]);

  const highlights = useMemo(() => {
    const h: Record<string, Highlight> = {};
    if (exercise) {
      // secondary first so a muscle listed as both ends up primary
      for (const role of ["secondary", "primary"] as const)
        for (const em of exercise.muscles) if (em.role === role) h[em.muscle_code] = role;
    } else if (muscle) {
      h[muscle] = "selected";
    }
    return h;
  }, [exercise, muscle]);

  const onSelectMesh = useCallback((mesh: string | null) => {
    setExercise(null);
    setMuscle(mesh ? (meshOf[mesh] ?? mesh) : null);
  }, [meshOf]);

  const onMeshes = useCallback((n: string[]) => setMeshes(new Set(n)), []);
  const pickView = (v: ViewPreset) => { setView(v); setViewNonce((n) => n + 1); };
  const hiddenForExercise = exercise
    ? exercise.muscles.filter((x) => meshes && !meshes.has(x.muscle_code)).map((x) => names[x.muscle_code] ?? x.muscle_code)
    : [];

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{m.title}</h1>
        <p className="text-sm text-muted-foreground">{m.description}</p>
      </div>
      {catalog.error && (
        <p role="alert" className="text-sm text-destructive">
          {catalog.error} <Button variant="outline" size="sm" onClick={catalog.reload}>{messages.errors.retry}</Button>
        </p>
      )}
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="space-y-2">
          <div className="relative h-[55vh] min-h-[320px] overflow-hidden rounded-lg border bg-muted/30" role="img" aria-label={m.canvasLabel}>
            <CanvasBoundary>
              <MuscleCanvas
                highlights={highlights}
                view={view}
                viewNonce={viewNonce}
                onSelect={onSelectMesh}
                onMeshes={onMeshes}
              />
            </CanvasBoundary>
            {!meshes && <p className="pointer-events-none absolute inset-0 flex items-center justify-center text-sm text-muted-foreground" aria-live="polite">{m.loadingModel}</p>}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button size="sm" variant={view === "front" ? "default" : "outline"} aria-pressed={view === "front"} onClick={() => pickView("front")}>{m.views.front}</Button>
            <Button size="sm" variant={view === "back" ? "default" : "outline"} aria-pressed={view === "back"} onClick={() => pickView("back")}>{m.views.back}</Button>
            <Button size="sm" variant="ghost" onClick={() => pickView(view)}>{m.views.reset}</Button>
          </div>
          <section aria-label={m.legend} className="text-sm">
            <ul className="flex flex-wrap gap-x-4 gap-y-1">
              <Swatch color={HIGHLIGHT_COLORS.primary} label={m.legendPrimary} />
              <Swatch color={HIGHLIGHT_COLORS.secondary} label={m.legendSecondary} />
              <Swatch color={HIGHLIGHT_COLORS.selected} label={m.legendSelected} />
              <Swatch color={HIGHLIGHT_COLORS.idle} label={m.legendIdle} />
            </ul>
            {hiddenForExercise.length > 0 && <p className="mt-1 text-xs text-muted-foreground">{m.exerciseMusclesNoModel(hiddenForExercise.join(", "))}</p>}
          </section>
        </div>

        <div className="space-y-4">
          <Card>
            <CardContent className="space-y-3 p-4">
              <h2 className="font-semibold">{m.selectedMuscle}</h2>
              {muscle ? (
                <>
                  <p className="text-sm">
                    <span className="font-medium">{names[muscle] ?? muscle}</span>
                    {meshes && !hasMesh(muscle) && <span className="ml-2 rounded-full bg-muted px-2 py-0.5 text-xs">{m.noModelBadge}</span>}
                  </p>
                  {meshes && !hasMesh(muscle) && <p className="text-xs text-muted-foreground">{m.noModelForSelected}</p>}
                  <Button size="sm" variant="outline" onClick={() => { setMuscle(null); setExercise(null); }}>{m.clearMuscle}</Button>
                  <h3 className="text-sm font-medium">{m.exercisesFor(names[muscle] ?? muscle)}</h3>
                  {loading && <p className="text-sm text-muted-foreground">{messages.library.loading}</p>}
                  {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
                  {!loading && !error && items.length === 0 && <p className="text-sm text-muted-foreground">{m.noExercises}</p>}
                  <ul className="max-h-64 space-y-1 overflow-y-auto">
                    {items.map((ex) => {
                      const role = ex.muscles.find((x) => x.muscle_code === muscle)?.role;
                      const active = exercise?.id === ex.id;
                      return (
                        <li key={ex.id}>
                          <button
                            type="button"
                            aria-pressed={active}
                            onClick={() => setExercise(active ? null : ex)}
                            className={`flex min-h-11 w-full items-center justify-between gap-2 rounded-md border px-3 py-2 text-left text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${active ? "border-primary bg-muted" : "hover:bg-muted"}`}
                          >
                            <span>{ex.name_en}</span>
                            {role && <span className="shrink-0 text-xs text-muted-foreground">{messages.labels.role[role]}</span>}
                          </button>
                        </li>
                      );
                    })}
                  </ul>
                </>
              ) : (
                <p className="text-sm text-muted-foreground">{m.noMuscleSelected}</p>
              )}
              {exercise && (
                <div className="space-y-2 border-t pt-3">
                  <p className="text-sm"><span className="text-muted-foreground">{m.selectedExercise}: </span><span className="font-medium">{exercise.name_en}</span></p>
                  <div className="flex flex-wrap gap-2">
                    <Button size="sm" variant="outline" onClick={() => setExercise(null)}>{m.clearExercise}</Button>
                    <Button asChild size="sm" variant="ghost"><Link href={`/exercises/${exercise.id}`}>{m.openExercise}</Link></Button>
                  </div>
                  <ul className="space-y-1 text-xs">
                    {exercise.muscles.map((em) => (
                      <li key={em.muscle_code}>
                        <span className="font-medium">{messages.labels.role[em.role]}:</span> {names[em.muscle_code] ?? em.muscle_code}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardContent className="space-y-2 p-4">
              <h2 className="font-semibold">{m.musclesList}</h2>
              <div className="flex flex-wrap gap-1">
                {withModel.map((x) => (
                  <button
                    key={x.code}
                    type="button"
                    aria-pressed={muscle === x.code}
                    onClick={() => { setExercise(null); setMuscle(x.code); }}
                    className={`min-h-9 rounded-full border px-3 text-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${muscle === x.code ? "border-primary bg-muted" : "hover:bg-muted"}`}
                  >
                    {x.name_en}
                  </button>
                ))}
              </div>
              {noModel.length > 0 && (
                <div className="space-y-2 border-t pt-3">
                  <h3 className="text-sm font-medium">{m.noModelTitle}</h3>
                  <p className="text-xs text-muted-foreground">{m.noModelDescription}</p>
                  <div className="flex flex-wrap gap-1">
                    {noModel.map((x) => (
                      <button
                        key={x.code}
                        type="button"
                        aria-pressed={muscle === x.code}
                        onClick={() => { setExercise(null); setMuscle(x.code); }}
                        className={`min-h-9 rounded-full border border-dashed px-3 text-xs text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${muscle === x.code ? "border-primary bg-muted" : "hover:bg-muted"}`}
                      >
                        {x.name_en} · {m.noModelBadge}
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
      <p className="text-xs text-muted-foreground">
        {m.attribution}{" "}
        <a className="underline" href="https://dbarchive.biosciencedbc.jp/en/bodyparts3d/" target="_blank" rel="noreferrer">{m.attributionLink}</a>
      </p>
    </div>
  );
}
