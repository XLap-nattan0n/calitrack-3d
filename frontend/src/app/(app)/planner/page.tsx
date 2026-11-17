import { Suspense } from "react";
import { Planner } from "@/components/planner/planner";

export default function PlannerPage() {
  return (
    <Suspense fallback={null}>
      <Planner />
    </Suspense>
  );
}
