import { useEffect, useRef, useState } from "react";
import type { ProgressPhase } from "../contract";
import { bytes, count, files } from "../format";
import { Section } from "../ui";
import { TOTAL } from "../sample";

const PHASES: { id: ProgressPhase; name: string; hint: string }[] = [
  { id: "hashing", name: "Подсчёт отпечатков", hint: "чтение файлов источника" },
  { id: "negotiating", name: "Сверка отпечатков", hint: "сопоставление с приёмником" },
  { id: "transferring", name: "Передача", hint: "отправка недостающих файлов" },
  { id: "placing", name: "Размещение", hint: "раскладка файлов по местам" },
  { id: "verifying", name: "Проверка", hint: "повторное чтение результата" },
];

const TOTAL_BLOBS = 27211;
const SKIPPED_BYTES = 1284000000;

type Props = { onDone: () => void };

export function TransferScreen({ onDone }: Props) {
  const [phase, setPhase] = useState(0);
  const [done, setDone] = useState(0);
  const [sent, setSent] = useState(0);
  const [stopped, setStopped] = useState(false);
  const tick = useRef<number>();

  useEffect(() => {
    if (stopped) return;
    tick.current = window.setInterval(() => {
      setDone((d) => {
        const next = d + Math.round(TOTAL.files / 60);
        if (next >= TOTAL.files) {
          setPhase((p) => Math.min(p + 1, PHASES.length - 1));
          return 0;
        }
        return next;
      });
      setSent((b) => Math.min(b + TOTAL.bytes / 60, TOTAL.bytes));
    }, 120);
    return () => window.clearInterval(tick.current);
  }, [stopped]);

  const current = PHASES[phase];

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
          {PHASES.map((p, i) => (
            <li key={p.id} className={i < phase ? "phase-done" : i === phase ? "phase-now" : ""}>
              <span className="phase-name">{p.name}</span>
              <span className="phase-hint">{p.hint}</span>
            </li>
          ))}
        </ol>

        <div className="bar">
          <div className="bar-fill" style={{ width: `${Math.round((done / TOTAL.files) * 100)}%` }} />
        </div>

        <div className="stats">
          <div>
            <b>{bytes(sent)}</b>
            <span>из {bytes(TOTAL.bytes)}</span>
            <em>передано</em>
          </div>
          <div className="stat-quiet">
            <b>{bytes(SKIPPED_BYTES)}</b>
            <span>{count(TOTAL_BLOBS)} отпечатков</span>
            <em>пропущено без пересылки</em>
          </div>
          <div>
            <b>{files(done)}</b>
            <span>из {files(TOTAL.files)}</span>
            <em>обработано</em>
          </div>
        </div>
      </Section>

      {stopped ? (
        <Section title="Остановлено">
          <p className="lead">
            Скопированное осталось на приёмнике. Следующий запуск продолжит с этого места.
            Источник не изменялся.
          </p>
          <button className="btn btn-primary" onClick={() => setStopped(false)}>
            Продолжить
          </button>
        </Section>
      ) : (
        <footer className="footer">
          <div className="footer-text">
            Остановка безопасна. Теряется не больше одного незавершённого файла, следующий
            запуск продолжит с этого места.
          </div>
          <button className="btn btn-secondary" onClick={() => setStopped(true)}>
            Остановить
          </button>
          <button className="btn btn-primary" onClick={onDone}>
            К проверке
          </button>
        </footer>
      )}
    </div>
  );
}
