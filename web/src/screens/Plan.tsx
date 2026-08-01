import { useMemo, useState } from "react";
import type { Package, Plan } from "../contract";
import { bytes, count, files, packages, plural } from "../format";
import { Callout, Check, Disclosure, GroupHead, Section } from "../ui";
import { TARGETS, TOTAL, planFor } from "../sample";

type Props = { onStart: () => void };

export function PlanScreen({ onStart }: Props) {
  const [target, setTarget] = useState(TARGETS[0].version);
  const [worldsOff, setWorldsOff] = useState<Set<string>>(new Set());
  const [taken, setTaken] = useState<Set<string>>(new Set());
  const [dirsOff, setDirsOff] = useState<Set<string>>(new Set());

  const plan = useMemo(() => planFor(target), [target]);
  const groups = useMemo(() => split(plan), [plan]);

  const included = plan.worlds.filter((w) => !w.blocker && !worldsOff.has(w.id));
  const blocked = plan.worlds.filter((w) => w.blocker);

  const toggle = (set: Set<string>, apply: (s: Set<string>) => void) => (id: string) => {
    const next = new Set(set);
    next.has(id) ? next.delete(id) : next.add(id);
    apply(next);
  };

  return (
    <div className="screen">
      <header className="screen-head">
        <div>
          <div className="kicker">Источник</div>
          <h1>{plan.source.root}</h1>
        </div>
        <label className="target">
          <span className="target-label">Целевая версия Foundry</span>
          <select value={target} onChange={(e) => setTarget(e.target.value)}>
            {TARGETS.map((t) => (
              <option key={t.version} value={t.version}>
                {t.version}, {t.hint}
              </option>
            ))}
          </select>
          <span className="target-note">Выбор версии меняет все решения ниже</span>
        </label>
      </header>

      <Section title="Миры" hint={`${included.length} из ${plan.worlds.length} войдут в перенос`}>
        <div className="rows">
          {plan.worlds.map((w) => (
            <div key={w.id} className={"row" + (w.blocker ? " row-blocked" : "")}>
              <div className="row-main">
                <div className="row-title">{w.id}</div>
                <div className="row-sub">
                  {w.system} {w.system_version}, ядро {w.core_version}
                </div>
                {w.blocker && <div className="row-blocker">{w.blocker}</div>}
              </div>
              <Check
                checked={!w.blocker && !worldsOff.has(w.id)}
                disabled={Boolean(w.blocker)}
                onChange={() => toggle(worldsOff, setWorldsOff)(w.id)}
                label="переносить"
              />
            </div>
          ))}
        </div>
        {blocked.length > 0 && (
          <p className="section-foot">
            {plural(blocked.length, "Заблокирован", "Заблокированы", "Заблокированы")}{" "}
            {blocked.length} {plural(blocked.length, "мир", "мира", "миров")}. Это обычное
            состояние, а не сбой.
          </p>
        )}
      </Section>

      <Section title="Пакеты" hint={`${packages(plan.packages.length)} всего`}>
        {groups.required.length > 0 && (
          <>
            <GroupHead
              title="Требуют обновления"
              n={groups.required.length}
              hint="Установленные сборки на выбранной версии не запускаются"
            />
            <PackageRows list={groups.required} taken={taken} onToggle={toggle(taken, setTaken)} />
          </>
        )}

        {groups.widening.length > 0 && (
          <>
            <GroupHead
              title="Стоит взять"
              n={groups.widening.length}
              hint="Работают на выбранной версии и подготовлены к следующему поколению"
            />
            <PackageRows list={groups.widening} taken={taken} onToggle={toggle(taken, setTaken)} />
          </>
        )}

        {groups.keep.length > 0 && (
          <>
            <GroupHead
              title="Остаются как есть"
              n={groups.keep.length}
              hint="Обновления существуют, но на выбранной версии не работают, поэтому выбор недоступен"
            />
            <Disclosure label={`Показать ${packages(groups.keep.length)}`}>
              <p className="names">{groups.keep.map((p) => p.id).join(", ")}</p>
            </Disclosure>
          </>
        )}

        {groups.manual.length > 0 && (
          <>
            <GroupHead
              title="Требуют внимания"
              n={groups.manual.length}
              hint="Переносятся с диска. Причина указана для каждого"
            />
            <div className="rows">
              {groups.manual.map((p) => (
                <div key={p.id} className="row">
                  <div className="row-main">
                    <div className="row-title">{p.id}</div>
                    <div className="row-sub">версия {p.version}</div>
                  </div>
                  <div className="row-reason">{p.reason}</div>
                </div>
              ))}
            </div>
          </>
        )}

        {groups.premium.length > 0 && (
          <>
            <GroupHead
              title="Платный контент"
              n={groups.premium.length}
              hint="Копирование с диска вместо повторной загрузки. Для купленного это обычный порядок"
            />
            <Disclosure label={`Показать ${packages(groups.premium.length)}`}>
              <p className="names">{groups.premium.map((p) => p.id).join(", ")}</p>
            </Disclosure>
          </>
        )}
      </Section>

      <Section title="Файлы" hint="медиа и данные вне пакетов">
        <div className="stats">
          <div>
            <b>{files(plan.assets.referenced.files)}</b>
            <span>{bytes(plan.assets.referenced.bytes)}</span>
            <em>по ссылкам из миров</em>
          </div>
          <div>
            <b>{files(plan.assets.in_packages.files)}</b>
            <span>{bytes(plan.assets.in_packages.bytes)}</span>
            <em>внутри пакетов</em>
          </div>
          <div>
            <b>{count(plan.assets.broken_references)}</b>
            <span>ссылок</span>
            <em>не находят файла</em>
          </div>
        </div>

        {plan.assets.user_directories.map((d) =>
          d.looks_stale ? (
            <Callout
              key={d.path}
              kicker="пути устарели, файлы не лишние"
              title={`Папка «${d.path}», ${files(d.files)}, ${bytes(d.bytes)}`}
              aside={
                <Check
                  checked={!dirsOff.has(d.path)}
                  onChange={() => toggle(dirsOff, setDirsOff)(d.path)}
                  label="переносить"
                />
              }
            >
              Живых ссылок на эти файлы нет, но{" "}
              {count(d.broken_references_into_it ?? 0)}{" "}
              {plural(d.broken_references_into_it ?? 0, "битая ссылка ведёт", "битые ссылки ведут", "битых ссылок ведут")}{" "}
              внутрь этой же папки. Похоже, у файлов сместились пути. Папка включена в перенос,
              чтобы ссылки можно было починить на новом месте.
            </Callout>
          ) : null,
        )}

        <Disclosure label="Показать остальные папки без ссылок">
          <div className="rows">
            {plan.assets.user_directories
              .filter((d) => !d.looks_stale)
              .map((d) => (
                <div key={d.path} className="row">
                  <div className="row-main">
                    <div className="row-title">{d.path}</div>
                    <div className="row-sub">
                      {files(d.files)}, {bytes(d.bytes)}
                    </div>
                  </div>
                  <Check
                    checked={!dirsOff.has(d.path)}
                    onChange={() => toggle(dirsOff, setDirsOff)(d.path)}
                    label="переносить"
                  />
                </div>
              ))}
          </div>
        </Disclosure>
      </Section>

      <footer className="footer">
        <div className="footer-text">
          Будет скопировано {files(TOTAL.files)}, {bytes(TOTAL.bytes)}.
          {taken.size > 0 && ` Обновится ${packages(taken.size)}.`} Установка не изменяется.
        </div>
        <button className="btn btn-secondary">Предварительный расчёт</button>
        <button className="btn btn-primary" onClick={onStart}>
          Начать перенос
        </button>
      </footer>
    </div>
  );
}

function PackageRows({
  list, taken, onToggle,
}: { list: Package[]; taken: Set<string>; onToggle: (id: string) => void }) {
  return (
    <div className="rows">
      {list.map((p) => (
        <div key={p.id} className="row">
          <div className="row-main">
            <div className="row-title">{p.id}</div>
            <div className="row-sub">
              {p.version} до {p.available}
            </div>
          </div>
          <Check checked={taken.has(p.id)} onChange={() => onToggle(p.id)} label="обновить" />
        </div>
      ))}
    </div>
  );
}

function split(plan: Plan) {
  const required: Package[] = [];
  const widening: Package[] = [];
  const keep: Package[] = [];
  const manual: Package[] = [];
  const premium: Package[] = [];

  for (const p of plan.packages) {
    if (p.premium) premium.push(p);
    else if (p.source === "upload") manual.push(p);
    else if (p.recommend === "required") required.push(p);
    else if (p.recommend === "upgrade" && p.widens_support) widening.push(p);
    else if (p.recommend === "keep") keep.push(p);
  }
  return { required, widening, keep, manual, premium };
}
