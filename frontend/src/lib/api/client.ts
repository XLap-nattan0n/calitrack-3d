import { createClient } from "@/lib/supabase/client";
import { messages } from "@/constants/messages";
import type {
  Equipment, EquipmentCode, ExerciseDetail, ExerciseQuery, ExerciseSummary, MuscleGroup, Muscle, Plan, PlanInput,
} from "./types";

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message);
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const { data: { session } } = await createClient().auth.getSession();
  const res = await fetch(`/api/v1${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(session ? { Authorization: `Bearer ${session.access_token}` } : {}),
      ...init.headers,
    },
  });
  if (!res.ok) {
    let code = "unknown";
    let message: string = messages.errors.generic;
    try {
      const body = await res.json();
      code = body.error?.code ?? code;
      message = body.error?.message ?? message;
    } catch { /* non-JSON error body */ }
    throw new ApiError(res.status, code, message);
  }
  return res.status === 204 ? (undefined as T) : ((await res.json()) as T);
}

export function exerciseQueryString(q: ExerciseQuery): string {
  const p = new URLSearchParams();
  if (q.q?.trim()) p.set("q", q.q.trim());
  if (q.category) p.set("category", q.category);
  if (q.difficulty) p.set("difficulty", String(q.difficulty));
  if (q.muscle) p.set("muscle", q.muscle);
  // [] => explicit "equipment=" (no equipment); undefined => omit so the API uses saved equipment
  if (q.equipment?.length === 0) p.set("equipment", "");
  else q.equipment?.forEach((e) => p.append("equipment", e));
  if (q.cursor) p.set("cursor", q.cursor);
  p.set("limit", String(q.limit ?? 24));
  return p.toString();
}

export const api = {
  listExercises: (q: ExerciseQuery) =>
    request<{ items: ExerciseSummary[]; next_cursor?: string | null }>(`/exercises?${exerciseQueryString(q)}`),
  getExercise: (id: string) => request<ExerciseDetail>(`/exercises/${encodeURIComponent(id)}`),
  listEquipment: () => request<{ equipment: Equipment[] }>("/equipment"),
  listMuscleGroups: () => request<{ items: MuscleGroup[] }>("/muscle-groups"),
  listMuscles: () => request<{ items: Muscle[] }>("/muscles"),
  getMyEquipment: () => request<{ equipment: Equipment[] }>("/me/equipment"),
  setMyEquipment: (codes: EquipmentCode[]) =>
    request<{ equipment: Equipment[] }>("/me/equipment", { method: "PUT", body: JSON.stringify({ equipment_codes: codes }) }),
  createPlan: (plan: PlanInput) => request<Plan>("/plans", { method: "POST", body: JSON.stringify(plan) }),
};

/** Feature flag: mock plan saving until M3.2 (Plans API) is merged. */
export const MOCK_PLANS = process.env.NEXT_PUBLIC_MOCK_PLANS === "true";
