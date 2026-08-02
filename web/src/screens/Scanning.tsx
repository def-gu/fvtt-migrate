import { useEffect, useRef, useState } from "react";
import type { Inventory, ScanEvent, ScanPhase } from "../contract";
import * as api from "../api";
import { Section } from "../ui";
import { bytes, count, shortPath } from "../format";

const PHASES: { id: ScanPhase; name: string; hint: string }[] = [
  { id: "packages", name: "Пакеты", hint: "манифесты миров, систем и модулей" },
  { id: "indexing", name: "Файлы", hint: "обход папки с данными" },
  { id: "worlds", name: "Миры", hint: "чтение баз данных" },
  { id: "classifying", name: "Ссылки", hint: "какие файлы кем используются" },
];

type Step = { done: number; total: number; bytes?: number; detail?: string };

export function ScanningScreen({
  root,
  onDone,
}: {
  root: string;
  onDone: (inv: Inventory) => void;
}) {
  const [steps, setSteps] = useState<Partial<Record<ScanPhase, Step>>>({});
  const [phase, setPhase] = useState<ScanPhase>("packages");
  const [error, setError] = useState("");
  const started = useRef(false);

  useEffect(() => {
    if (started.current) return;
    started.current = true;

    api
      .scan((e: ScanEvent) => {
        if (e.type === "progress") {
          setPhase(e.phase);
          setSteps((s) => ({
            ...s,
            [e.phase]: { done: e.done, total: e.total, bytes: e.bytes, detail: e.detail },
          }));
        } else if (e.type === "result") {
          onDone(e.result);
        } else if (e.type === "failed") {
          setError(e.message);
        }
      })
      .catch((e) => setError(String((e as Error).message)));
  }, [onDone]);

  if (error) {
    return (
      <div className="screen">
        <header className="screen-head">
          <div>
            <div className="kicker">{root}</div>
            <h1>Не удалось прочитать установку</h1>
            <p className="lead verdict-bad">{error}</p>
          </div>
        </header>
      </div>
    );
  }

  const at = PHASES.findIndex((p) => p.id === phase);
  const step = steps[phase];
  const share = step && step.total > 0 ? Math.round((step.done / step.total) * 100) : 0;

  return (
    <div className="screen">
      <header className="screen-head">
        <div>
          <div className="kicker">{root}</div>
          <h1>Чтение установки</h1>
          <p className="lead">
            Программа только читает. Она ничего не меняет и никуда не обращается.
          </p>
        </div>
      </header>

      <Section title="Ход работы">
        <ol className="phases">
          {PHASES.map((p, i) => {
            const s = steps[p.id];
            return (
              <li key={p.id} className={i < at ? "phase-done" : i === at ? "phase-now" : ""}>
                <span className="phase-name">{p.name}</span>
                <span className="phase-hint">
                  {s ? (s.total > 0 ? `${count(s.done)} из ${count(s.total)}` : count(s.done)) : p.hint}
                </span>
              </li>
            );
          })}
        </ol>

        <div className="bar">
          <div className="bar-fill" style={{ width: (step && step.total > 0 ? share : 100) + "%" }} />
        </div>

        <p className="row-sub browser-path">
          {step?.detail ? shortPath(step.detail, 64) : ""}
          {step?.bytes ? " " + bytes(step.bytes) : ""}
        </p>
      </Section>
    </div>
  );
}
