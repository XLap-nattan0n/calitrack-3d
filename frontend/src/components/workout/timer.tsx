"use client";

import { useEffect, useRef, useState } from "react";
import { messages } from "@/constants/messages";
import { Button } from "@/components/ui/button";

const t = messages.timer;

export const fmt = (s: number) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;

/** Wall-clock based ticking so background tabs do not drift. Returns elapsed whole seconds. */
function useStopwatch(running: boolean, baseSeconds: number) {
  const [elapsed, setElapsed] = useState(baseSeconds);
  const baseRef = useRef(baseSeconds);
  useEffect(() => {
    baseRef.current = elapsed;
  }, [running]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!running) return;
    const startedAt = Date.now();
    const base = baseRef.current;
    const id = setInterval(() => setElapsed(base + Math.floor((Date.now() - startedAt) / 1000)), 250);
    return () => clearInterval(id);
  }, [running]);
  return [elapsed, setElapsed] as const;
}

/** Count-down rest timer; `startKey` change restarts it. */
export function RestTimer({ seconds, startKey, onDone }: { seconds: number; startKey: number; onDone?: () => void }) {
  const [total, setTotal] = useState(seconds);
  const [running, setRunning] = useState(startKey > 0);
  const [elapsed, setElapsed] = useStopwatch(running, 0);
  const remaining = Math.max(total - elapsed, 0);
  const finished = useRef(false);

  useEffect(() => {
    if (startKey > 0) {
      setTotal(seconds);
      setElapsed(0);
      finished.current = false;
      setRunning(true);
    }
  }, [startKey]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (running && remaining === 0 && !finished.current) {
      finished.current = true;
      setRunning(false);
      onDone?.();
    }
  }, [running, remaining, onDone]);

  return (
    <div className="flex flex-wrap items-center gap-2 rounded-md border p-3" role="timer" aria-label={t.rest}>
      <span className="text-sm font-medium">{t.rest}</span>
      <span className="text-2xl font-semibold tabular-nums" aria-live="off">{fmt(remaining)}</span>
      <Button size="sm" variant="outline" onClick={() => setRunning((r) => !r)} disabled={remaining === 0}>
        {running ? t.pause : t.start}
      </Button>
      <Button size="sm" variant="ghost" onClick={() => { setTotal((x) => x + 15); finished.current = false; }}>{t.add15}</Button>
      <Button size="sm" variant="ghost" onClick={() => { setRunning(false); setElapsed(total); }}>{t.restSkip}</Button>
      {remaining === 0 && startKey > 0 && <span role="status" className="text-sm text-muted-foreground">{t.restDone}</span>}
    </div>
  );
}

/** Count-up hold stopwatch; "use" copies the time into the set's duration. */
export function HoldTimer({ onUse }: { onUse: (seconds: number) => void }) {
  const [running, setRunning] = useState(false);
  const [elapsed, setElapsed] = useStopwatch(running, 0);
  return (
    <div className="flex flex-wrap items-center gap-2" role="timer" aria-label={t.hold}>
      <span className="text-lg font-semibold tabular-nums">{fmt(elapsed)}</span>
      <Button size="sm" variant="outline" onClick={() => setRunning((r) => !r)}>{running ? t.pause : t.start}</Button>
      <Button size="sm" variant="ghost" onClick={() => { setRunning(false); setElapsed(0); }}>{t.reset}</Button>
      <Button size="sm" variant="ghost" disabled={elapsed === 0} onClick={() => { setRunning(false); onUse(elapsed); }}>{t.useTime}</Button>
    </div>
  );
}
