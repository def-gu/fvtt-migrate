import { useMemo, useState } from "react";
import type { Package, Plan } from "../contract";
import type { Destination, State } from "../api";
import * as api from "../api";
import { bytes, count, files, packages, plural } from "../format";
import { Callout, Check, Disclosure, GroupHead, Section } from "../ui";

type Props = {
  state: State;
  plan: Plan;
  dest: Destination;
  onDest: (d: Destination) => void;
  onPlan: (p: Plan) => void;
  onStart: () => void;
};

export function PlanScreen({ state, plan, dest, onDest, onPlan, onStart }: Props) {
  const [busy, setBusy] = useState(false);
  const [checkUpdates, setCheckUpdates] = useState(false);
  const groups = useMemo(() => split(plan), [plan]);

  const rebuild = async (target: string, updates: boolean) => {
    setBusy(true);
    try {
      onPlan(await api.buildPlan(target, updates));
    } finally {
      setBusy(false);
    }
  };

  const setWorld = (id: string, include: boolean) =>
    onPlan({ ...plan, worlds: plan.worlds.map((w) => (w.id === id ? { ...w, include } : w)) });

  const setDirectory = (path: string, action: "include" | "skip") =>
    onPlan({
      ...plan,
      assets: {
        ...plan.assets,
        user_directories: plan.assets.user_directories.map((d) =>
          d.path === path ? { ...d, action } : d,
        ),
      },
    });

  const setPolicy = (id: string, take: boolean) =>
    onPlan({
      ...plan,
      packages: plan.packages.map((p) => (p.id === id ? { ...p, policy: take ? "latest" : "pin" } : p)),
    });

  const included = plan.worlds.filter((w) => !w.blocker && w.include);
  const blocked = plan.worlds.filter((w) => w.blocker);
  const ready = dest.to.trim() !== "" && !busy;

  return (
    <div className="screen">
      <header className="screen-head">
        <div>
          <div className="kicker">План переноса</div>
          <h1>{state.scan.counts.worlds} {plural(state.scan.counts.worlds, "мир", "мира", "миров")}, {packages(plan.packages.length)}</h1>
        </div>
        <label className="target">
          <span className="target-label">Целевая версия Foundry</span>
          <select
            value={plan.source.target_core_version}
            disabled={busy}
            onChange={(e) => rebuild(e.target.value, checkUpdates)}
          >
            {state.targets.map((t) => (
              <option key={t} value={t}>{t}</option>
            ))}
          </select>
          <span className="target-note">Выбор версии меняет все решения ниже</span>
          <Check
            checked={checkUpdates}
            disabled={busy}
            label="узнать доступные версии у источников"
            onChange={(v) => {
              setCheckUpdates(v);
              rebuild(plan.source.target_core_version, v);
            }}
          />
        </label>
      </header>

      <Section title="Миры" hint={`${included.length} из ${plan.worlds.length} войдут в перенос`}>
        <div className="rows">
          {plan.worlds.map((w) => (
            <div key={w.id} className={"row" + (w.blocker ? " row-blocked" : "")}>
              <div className="row-main">
                <div className="row-title">{w.id}</div>
                <div className="row-sub">{w.system} {w.system_version}, ядро {w.core_version}</div>
                {w.blocker && <div className="row-blocker">{w.blocker}</div>}
              </div>
              <Check
                checked={!w.blocker && w.include}
                disabled={Boolean(w.blocker)}
                onChange={(v) => setWorld(w.id, v)}
                label="переносить"
              />
            </div>
          ))}
        </div>
        {blocked.length > 0 && (
          <p className="section-foot">
            {plural(blocked.length, "Заблокирован", "Заблокированы", "Заблокированы")} {blocked.length}{" "}
            {plural(blocked.length, "мир", "мира", "миров")}. Это обычное состояние, а не сбой.
          </p>
        )}
      </Section>

      <Section title="Пакеты" hint={`${packages(plan.packages.length)} всего`}>
        <Group
          title="Требуют обновления"
          hint="Установленные сборки на выбранной версии не запускаются"
          list={groups.required}
          plan={plan}
          onTake={setPolicy}
        />
        <Group
          title="Стоит взять"
          hint="Работают на выбранной версии и подготовлены к следующему поколению"
          list={groups.widening}
          plan={plan}
          onTake={setPolicy}
        />

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

        {plan.assets.user_directories
          .filter((d) => d.looks_stale)
          .map((d) => (
            <Callout
              key={d.path}
              kicker="пути устарели, файлы не лишние"
              title={`Папка «${d.path}», ${files(d.files)}, ${bytes(d.bytes)}`}
              aside={
                <Check
                  checked={d.action === "include"}
                  onChange={(v) => setDirectory(d.path, v ? "include" : "skip")}
                  label="переносить"
                />
              }
            >
              Живых ссылок на эти файлы нет, но {count(d.broken_references_into_it ?? 0)}{" "}
              {plural(d.broken_references_into_it ?? 0, "битая ссылка ведёт", "битые ссылки ведут", "битых ссылок ведут")}{" "}
              внутрь этой же папки. Похоже, у файлов сместились пути. Папка включена в перенос,
              чтобы ссылки можно было починить на новом месте.
            </Callout>
          ))}

        <Disclosure label="Показать остальные папки без ссылок">
          <div className="rows">
            {plan.assets.user_directories
              .filter((d) => !d.looks_stale)
              .map((d) => (
                <div key={d.path} className="row">
                  <div className="row-main">
                    <div className="row-title">{d.path}</div>
                    <div className="row-sub">{files(d.files)}, {bytes(d.bytes)}</div>
                  </div>
                  <Check
                    checked={d.action === "include"}
                    onChange={(v) => setDirectory(d.path, v ? "include" : "skip")}
                    label="переносить"
                  />
                </div>
              ))}
          </div>
        </Disclosure>
      </Section>

      <Section title="Куда переносить">
        <div className="dest">
          <label className="field">
            <span>Адрес приёмника</span>
            <input
              className="input"
              placeholder="https://ваш.домен или /путь/до/папки"
              value={dest.to}
              onChange={(e) => onDest({ ...dest, to: e.target.value })}
            />
          </label>
          <label className="field">
            <span>Ключ доступа</span>
            <input
              className="input"
              type="password"
              placeholder="для адреса с https"
              value={dest.token}
              onChange={(e) => onDest({ ...dest, token: e.target.value })}
            />
          </label>
        </div>
        <p className="section-foot">
          Ключ печатает принимающая сторона при запуске. Для переноса в папку на этой же машине
          он не нужен.
        </p>
      </Section>

      <footer className="footer">
        <div className="footer-text">
          Будет скопировано {files(plan.assets.referenced.files + plan.assets.in_packages.files)},{" "}
          {bytes(plan.assets.referenced.bytes + plan.assets.in_packages.bytes)}. Установка не изменяется.
        </div>
        <button
          className="btn btn-secondary"
          disabled={!ready}
          onClick={() => {
            onDest({ ...dest, dryRun: true });
            onStart();
          }}
        >
          Предварительный расчёт
        </button>
        <button
          className="btn btn-primary"
          disabled={!ready}
          onClick={() => {
            onDest({ ...dest, dryRun: false });
            onStart();
          }}
        >
          Начать перенос
        </button>
      </footer>
    </div>
  );
}

function Group({
  title, hint, list, plan, onTake,
}: {
  title: string; hint: string; list: Package[]; plan: Plan;
  onTake: (id: string, take: boolean) => void;
}) {
  if (list.length === 0) return null;
  const policy = new Map(plan.packages.map((p) => [p.id, p.policy]));

  return (
    <>
      <GroupHead title={title} n={list.length} hint={hint} />
      <div className="rows">
        {list.map((p) => (
          <div key={p.id} className="row">
            <div className="row-main">
              <div className="row-title">{p.id}</div>
              <div className="row-sub">{p.version} до {p.available}</div>
            </div>
            <Check
              checked={policy.get(p.id) !== "pin"}
              onChange={(v) => onTake(p.id, v)}
              label="обновить"
            />
          </div>
        ))}
      </div>
    </>
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
