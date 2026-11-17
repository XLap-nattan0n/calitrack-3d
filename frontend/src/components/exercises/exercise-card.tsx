import Link from "next/link";
import { messages } from "@/constants/messages";
import { Card, CardContent } from "@/components/ui/card";
import type { ExerciseSummary } from "@/lib/api/types";

export function ExerciseCard({ ex, muscleNames }: { ex: ExerciseSummary; muscleNames: Record<string, string> }) {
  const l = messages.labels;
  const primary = ex.muscles.filter((m) => m.role === "primary").map((m) => muscleNames[m.muscle_code] ?? m.muscle_code);
  return (
    <Card className="h-full">
      <CardContent className="flex h-full flex-col gap-2 p-4">
        <Link href={`/exercises/${ex.id}`} className="font-semibold underline-offset-2 hover:underline focus-visible:underline">
          {ex.name_en}
        </Link>
        <div className="flex flex-wrap gap-1 text-xs">
          <Chip>{l.category[ex.category]}</Chip>
          <Chip>{l.difficulty(ex.difficulty)}</Chip>
          <Chip>{l.type[ex.exercise_type]}</Chip>
        </div>
        {primary.length > 0 && <p className="text-sm text-muted-foreground">{primary.join(", ")}</p>}
        <p className="mt-auto text-xs text-muted-foreground">{ex.equipment_codes.map((c) => l.equipment[c]).join(" · ")}</p>
      </CardContent>
    </Card>
  );
}

export function Chip({ children }: { children: React.ReactNode }) {
  return <span className="rounded-full bg-muted px-2 py-0.5">{children}</span>;
}
