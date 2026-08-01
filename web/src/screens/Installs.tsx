import type { InstallStatus } from "../contract";
import { bytes, count, plural } from "../format";
import { Section } from "../ui";
import { INSTALLATIONS } from "../sample";

const STATUS: Record<InstallStatus, { label: string; tone: string; hint?: string }> = {
  idle: { label: "свободна", tone: "calm" },
  running: { label: "Foundry запущен", tone: "calm" },
  "world-loaded": {
    label: "мир открыт",
    tone: "busy",
    hint: "копирование открытой базы повредит мир",
  },
  migrating: {
    label: "идёт миграция",
    tone: "busy",
    hint: "Foundry не отвечает, пока переносит базу. Перезапуск в этот момент повредит мир",
  },
  unreachable: { label: "нет связи", tone: "bad" },
};

type Props = { onReview: () => void };

export function InstallsScreen({ onReview }: Props) {
  return (
    <div className="screen">
      <header className="screen-head">
        <div>
          <div className="kicker">Установки</div>
          <h1>Известные каталоги Foundry</h1>
        </div>
      </header>

      <Section title="Список">
        <div className="rows">
          {INSTALLATIONS.map((i) => {
            const s = STATUS[i.status];
            return (
              <div key={i.id} className="row">
                <div className="row-main">
                  <div className="row-title">{i.title}</div>
                  <div className="row-sub">
                    {i.machine}, ядро {i.core_version}, {count(i.worlds)}{" "}
                    {plural(i.worlds, "мир", "мира", "миров")}
                    {i.bytes > 0 && `, ${bytes(i.bytes)}`}
                  </div>
                  {s.hint && <div className="row-blocker">{s.hint}</div>}
                </div>
                <span className={"status status-" + s.tone}>{s.label}</span>
                <button className="btn btn-secondary" onClick={onReview}>
                  Собрать план
                </button>
              </div>
            );
          })}
        </div>
      </Section>
    </div>
  );
}
