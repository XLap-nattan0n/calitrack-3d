"use client";
import Link from "next/link";
import { useEffect, useState } from "react";
import { messages } from "@/constants/messages";
import { api, ApiError } from "@/lib/api/client";
import { useCatalog } from "@/lib/api/hooks";
import type { ExerciseDetail as Detail, ExerciseSummary } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { Chip } from "./exercise-card";

const d = messages.detail;
const l = messages.labels;

export function ExerciseDetail({ id }: { id: string }) {
  const catalog = useCatalog();
  const [ex, setEx] = useState<Detail | null>(null);
  const [related, setRelated] = useState<Record<string, ExerciseSummary | Detail>>({});
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.getExercise(id)
      .then(async (e) => {
        setEx(e);
        const rel = await Promise.all(e.progressions.map((p) => api.getExercise(p.exercise_id).catch(() => null)));
        setRelated(Object.fromEntries(rel.filter((r): r is Detail => !!r).map((r) => [r.id, r])));
      })
      .catch((e: Error) => setError(e instanceof ApiError && e.status === 404 ? d.notFound : e.message));
  }, [id]);

  const names = Object.fromEntries((catalog.data?.muscles ?? []).map((x) => [x.code, x.name_en]));

  if (error) return <p role="alert" className="text-destructive">{error}</p>;
  if (!ex) return <p className="text-muted-foreground">{messages.library.loading}</p>;

  const rec = [
    ex.rec_sets ? `${ex.rec_sets} ${d.sets}` : null,
    ex.rec_reps_min != null ? `${ex.rec_reps_min}${ex.rec_reps_max && ex.rec_reps_max !== ex.rec_reps_min ? `-${ex.rec_reps_max}` : ""} ${d.reps}` : null,
    ex.rec_hold_seconds ? `${ex.rec_hold_seconds}${d.seconds}` : null,
  ].filter(Boolean).join(" · ");

  return (
    <article className="max-w-3xl space-y-6">
      <Link href="/exercises" className="text-sm underline-offset-2 hover:underline">← {d.back}</Link>
      <header className="space-y-2">
        <h1 className="text-2xl font-semibold">{ex.name_en}</h1>
        <div className="flex flex-wrap gap-1 text-xs">
          <Chip>{l.category[ex.category]}</Chip>
          <Chip>{l.difficulty(ex.difficulty)}</Chip>
          <Chip>{l.type[ex.exercise_type]}</Chip>
        </div>
        <Button asChild size="sm"><Link href={`/planner?add=${ex.id}`}>{d.addToPlanner}</Link></Button>
      </header>
      <Section title={d.description}><p>{ex.description}</p></Section>
      <Section title={d.instructions}><p className="whitespace-pre-line">{ex.instructions}</p></Section>
      <Section title={d.safety}><p className="whitespace-pre-line">{ex.safety_notes}</p></Section>
      {rec && <Section title={d.recommended}><p>{rec}</p></Section>}
      <Section title={d.muscles}>
        <ul className="flex flex-wrap gap-2 text-sm">
          {ex.muscles.map((x) => (
            <li key={x.muscle_code}><Chip>{names[x.muscle_code] ?? x.muscle_code} ({l.role[x.role]})</Chip></li>
          ))}
        </ul>
      </Section>
      <Section title={d.equipment}><p>{ex.equipment_codes.map((c) => l.equipment[c]).join(", ")}</p></Section>
      {ex.progressions.length > 0 && (
        <Section title={d.progressions}>
          <ul className="space-y-1 text-sm">
            {ex.progressions.map((p) => (
              <li key={p.exercise_id + p.relation}>
                <span className="text-muted-foreground">{p.relation === "easier" ? d.easier : d.harder}: </span>
                <Link className="underline-offset-2 hover:underline" href={`/exercises/${p.exercise_id}`}>
                  {related[p.exercise_id]?.name_en ?? p.exercise_id}
                </Link>
              </li>
            ))}
          </ul>
        </Section>
      )}
    </article>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-1">
      <h2 className="text-lg font-medium">{title}</h2>
      {children}
    </section>
  );
}
