// Mirrors docs/openapi.yaml schemas.
export type EquipmentCode = "pullup_bar" | "parallel_bars" | "resistance_bands" | "bodyweight";
export type Category = "push" | "pull" | "legs" | "core" | "full_body" | "skills";
export type PlanCategory = Category | "custom";

export const CATEGORIES: Category[] = ["push", "pull", "legs", "core", "full_body", "skills"];
export const PLAN_CATEGORIES: PlanCategory[] = [...CATEGORIES, "custom"];

export interface Equipment { id: number; code: EquipmentCode; name_en: string; name_th?: string | null }
export interface MuscleGroup { id: number; code: string; name_en: string; name_th?: string | null }
export interface Muscle { id: number; group_id: number; code: string; name_en: string; mesh_name: string }
export interface ExerciseMuscle { muscle_code: string; role: "primary" | "secondary" }

export interface ExerciseSummary {
  id: string;
  slug: string;
  name_en: string;
  name_th?: string | null;
  category: Category;
  difficulty: number;
  exercise_type: "reps" | "hold";
  equipment_codes: EquipmentCode[];
  muscles: ExerciseMuscle[];
}

export interface ExerciseDetail extends ExerciseSummary {
  description: string;
  instructions: string;
  safety_notes: string;
  rec_sets?: number | null;
  rec_reps_min?: number | null;
  rec_reps_max?: number | null;
  rec_hold_seconds?: number | null;
  progressions: { exercise_id: string; relation: "easier" | "harder" }[];
}

export interface PlanExerciseInput {
  exercise_id: string;
  target_sets: number;
  target_reps?: number | null;
  hold_seconds?: number | null;
  rest_seconds?: number | null;
  assist_band?: string | null;
}

export interface PlanInput {
  plan_date: string;
  title: string;
  category: PlanCategory;
  target_muscle_group_ids?: number[];
  exercises: PlanExerciseInput[];
}

export interface Plan extends PlanInput { id: string; status: string }

export interface ExerciseQuery {
  q?: string;
  category?: Category | "";
  difficulty?: number | "";
  /** undefined = API default (user's equipment); [] = explicitly none (sent as "equipment=") */
  equipment?: EquipmentCode[];
  muscle?: string;
  cursor?: string;
  limit?: number;
}
