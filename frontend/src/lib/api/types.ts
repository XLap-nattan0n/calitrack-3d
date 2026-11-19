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

export type SessionStatus = "in_progress" | "completed" | "abandoned";

export interface SessionSet {
  id: string;
  set_number: number;
  target_reps?: number | null;
  actual_reps?: number | null;
  duration_seconds?: number | null;
  completed: boolean;
  assist_band?: string | null;
  notes?: string | null;
}

export interface SessionExercise {
  id: string;
  exercise_id: string;
  exercise_name?: string;
  position: number;
  sets: SessionSet[];
}

export interface SessionSummary {
  id: string;
  plan_id?: string | null;
  status: SessionStatus;
  started_at: string;
  finished_at?: string | null;
  progress_percent: number;
}

export interface Session extends SessionSummary {
  notes?: string | null;
  exercises: SessionExercise[];
}

export interface SetPatch {
  actual_reps?: number | null;
  duration_seconds?: number | null;
  completed?: boolean;
  assist_band?: string | null;
  notes?: string | null;
}

export interface PersonalRecord {
  exercise_id: string;
  exercise_name: string;
  best_reps?: number | null;
  best_hold_seconds?: number | null;
  achieved_at: string;
}

export type AnalyticsRange = "7d" | "30d" | "90d" | "1y" | "all";

export interface AnalyticsSummary {
  total_workouts: number;
  completed_sets: number;
  weekly_frequency: { week_start: string; workouts: number }[];
}

export interface ExerciseProgressPoint {
  date: string;
  best_reps?: number | null;
  best_hold_seconds?: number | null;
  total_sets: number;
}

export interface MuscleFrequencyItem { muscle_code: string; group_code: string; sessions: number }
