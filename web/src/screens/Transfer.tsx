import { useEffect, useRef, useState } from "react";
import type { ApplyResult, Plan, ProgressPhase } from "../contract";
import type { Destination } from "../api";
import * as api from "../api";
import { bytes, count, files } from "../format";
import { Section } from "../ui";

const PHASES: { id: ProgressPhase; name: string; hint: string }[] = [
  { id: "hashing", name: "Подсчёт отпечатков", hint: "чтение файлов источника" },
  { id: "negotiating", name: "Сверка отпечатков", hint: "сопоставление с приёмником" },
  { id: "transferring", name: "Передача", hint: "отправка недостающих файлов" },
  { id: "placing", name: "Размещение", hint: "раскладка файлов по местам" },
];

type Props = { plan: Plan; dest: Destination; onBack: () => void; onDone: () => void };

export function TransferScreen({ plan, dest, onBack, onDone }: Props) {
  const [phase, setPhase] = useState<ProgressPhase>("hashing");
  const [done, setDone] = useState(0);
  const [total, setTotal] = useState(0);
  const [sent, setSent] = useState(0);
  const [notices, setNotices] = useState<string[]>([]);
  const [result, setResult] = useState<ApplyResult | null>(null);
  const [failure, setFailure] = useState("");
  const abort = useRef<AbortController>();

  useEffect(() => {
    const controller = new AbortController();
    abort.current = controller;

    api
      .run(plan, dest, (e) => {
        if (e.type === "progress") {
          setPhase(e.phase);
          setDone(e.done);
          setTotal(e.total);
          if (e.bytes) setSent(e.bytes);
        } else if (e.type === "notice") {
          setNotices((n) => (n.includes(e.message) ? n : [...n, e.message]));
        } else if (e.type === "result") {
          setResult(e.result);
        } else if (e.type === "failed") {
          setFailure(e.message);
        }
      }, controller.signal)
      .catch((e) => {
        if (!controller.signal.aborted) setFailure(String((e as Error).message));
      });

    return () => controller.abort();
  }, [plan, dest]);

  if (failure) {
    return (
      <div className="screen">
        <header className="screen-head">
          <div>
            <div className="kicker">Перенос остановлен</div>
            <h1>Не получилось</h1>
            <p className="lead">{failure}</p>
          </div>
        </header>
        <footer className="footer">
          <div className="footer-text">Источник не изменялся.</div>
          <button className="btn btn-primary" onClick={onBack}>Вернуться к плану</button>
        </footer>
      </div>
    );
  }

  if (result) {
    const preview = dest.dryRun;
    return (
      <div className="screen">
        <header className="screen-head">
          <div>
            <div className="kicker">{preview ? "Предварительный расчёт" : "Перенос завершён"}</div>
            <h1>
              {preview
                ? `Поедет ${bytes(result.would_send_bytes ?? 0)}`
                : `Передано ${bytes(result.transferred_bytes)}`}
            </h1>
          </div>
        </header>

        <Section title="Итог">
          <div className="stats">
            <div>
              <b>{count(preview ? (result.would_send_blobs ?? 0) : result.transferred_blobs)}</b>
              <span>отпечатков</span>
              <em>{preview ? "будет отправлено" : "отправлено"}</em>
            </div>
            <div className="stat-quiet">
              <b>{bytes(result.already_present_bytes)}</b>
              <span>{count(result.already_present_blobs)} отпечатков</span>
              <em>пропущено без пересылки</em>
            </div>
            <div>
              <b>{files(result.placed_files)}</b>
              <span>из {files(result.selected_files)}</span>
              <em>размещено</em>
            </div>
          </div>
        </Section>

        <footer className="footer">
          <div className="footer-text">Источник не изменялся.</div>
          <button className="btn btn-secondary" onClick={onBack}>К плану</button>
          {!preview && (
            <button className="btn btn-primary" onClick={onDone}>Проверить результат</button>
          )}
        </footer>
      </div>
    );
  }

  const current = PHASES.find((p) => p.id === phase) ?? PHASES[0];
  const share = total > 0 ? Math.round((done / total) * 100) : 0;

  return (
    <div className="screen">
      <header className="screen-head">
        <div>
          <div className="kicker">Перенос</div>
          <h1>{current.name}</h1>
          <p className="lead">{current.hint}</p>
        </div>
      </header>

      <Section title="Ход работы">
        <ol className="phases">
          {PHASES.map((p) => {
            const at = PHASES.findIndex((x) => x.id === phase);
            const i = PHASES.findIndex((x) => x.id === p.id);
            return (
              <li key={p.id} className={i < at ? "phase-done" : i === at ? "phase-now" : ""}>
                <span className="phase-name">{p.name}</span>
                <span className="phase-hint">{p.hint}</span>
              </li>
            );
          })}
        </ol>

        <div className="bar">
          <div className="bar-fill" style={{ width: `${share}%` }} />
        </div>

        <div className="stats">
          <div>
            <b>{bytes(sent)}</b>
            <span>отправлено</span>
            <em>по проводу</em>
          </div>
          <div>
            <b>{count(done)}</b>
            <span>из {count(total)}</span>
            <em>обработано</em>
          </div>
        </div>

        {notices.map((n) => (
          <p key={n} className="section-foot">{n}</p>
        ))}
      </Section>

      <footer className="footer">
        <div className="footer-text">
          Остановка безопасна. Теряется не больше одного незавершённого файла, следующий запуск
          продолжит с этого места.
        </div>
        <button className="btn btn-secondary" onClick={() => abort.current?.abort()}>
          Остановить
        </button>
      </footer>
    </div>
  );
}
