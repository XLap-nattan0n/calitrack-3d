"use client";
import { useCallback, useEffect, useState } from "react";
import { api } from "./client";
import type { Equipment, MuscleGroup, Muscle } from "./types";

export interface Catalog { equipment: Equipment[]; groups: MuscleGroup[]; muscles: Muscle[] }

/** Loads static catalog data once. */
export function useCatalog() {
  const [data, setData] = useState<Catalog | null>(null);
  const [error, setError] = useState<string | null>(null);
  const load = useCallback(() => {
    setError(null);
    Promise.all([api.listEquipment(), api.listMuscleGroups(), api.listMuscles()])
      .then(([e, g, m]) => setData({ equipment: e.equipment, groups: g.items, muscles: m.items }))
      .catch((e: Error) => setError(e.message));
  }, []);
  useEffect(load, [load]);
  return { data, error, reload: load };
}
