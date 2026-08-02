import { useState } from "react";
import type { Inventory, InventoryWorld, Plan } from "../contract";
import { bytes, count, files, plain, plural, shortPath } from "../format";
import { Check, Disclosure } from "../ui";

type Props = {
  inventory: Inventory;
  plan: Plan;
  onInclude: (id: string, include: boolean) => void;
};

export function WorldsTab({ inventory, plan, onInclude }: Props) {
  const [order, setOrder] = useState<"played" | "size" | "title">("played");
  const decided = new Map(plan.worlds.map((w) => [w.id, w]));
  const sorted = [...inventory.worlds].sort(compare(order));

  return (
    <div className="tab">
      <div className="tab-head">
        <p className="lead">
          Названия взяты из самих миров. Выключенный мир остаётся на этой машине и никуда
          не едет.
        </p>
        <Sort value={order} onChange={setOrder} />
      </div>

      <div className="cards">
        {sorted.map((w) => (
          <WorldCard
            key={w.id}
            world={w}
            include={decided.get(w.id)?.include ?? true}
            blocker={decided.get(w.id)?.blocker}
            onInclude={(v) => onInclude(w.id, v)}
          />
        ))}
      </div>
    </div>
  );
}

function WorldCard({
  world,
  include,
  blocker,
  onInclude,
}: {
  world: InventoryWorld;
  include: boolean;
  blocker?: string;
  onInclude: (v: boolean) => void;
}) {
  const missing = world.missing_modules.length;

  return (
    <article className={"card elev-sm" + (blocker ? " card-blocked" : "")}>
      <div className="card-head">
        <div>
          <div className="card-title">{world.title}</div>
          <div className="card-meta">
            <span>{world.system_title || world.system}</span>
            <span>версия {world.system_version || "неизвестна"}</span>
            <span>Foundry {world.core_version}</span>
          </div>
        </div>
        <Check
          checked={include && !blocker}
          disabled={Boolean(blocker)}
          label="переносить"
          onChange={onInclude}
        />
      </div>

      {blocker && <p className="row-blocker">{blocker}</p>}

      {world.description && <p className="card-body">{plain(world.description)}</p>}

      <div className="card-meta">
        <span>{files(world.size.files)}</span>
        <span>{bytes(world.size.bytes)}</span>
        <span>
          {count(world.active_modules.length)}{" "}
          {plural(world.active_modules.length, "включённый модуль", "включённых модуля", "включённых модулей")}
        </span>
        {missing > 0 && (
          <span className="tag tag-accent-2">
            {count(missing)} {plural(missing, "не установлен", "не установлены", "не установлены")}
          </span>
        )}
      </div>

      <Disclosure label="Что входит в этот мир">
        <div className="pairs">
          <div className="pair-key">Папка</div>
          <div className="pair-value">{shortPath(world.path, 70)}</div>
          {world.last_played && (
            <>
              <div className="pair-key">Последняя игра</div>
              <div className="pair-value">{world.last_played}</div>
            </>
          )}
          <div className="pair-key">Игровая система</div>
          <div className="pair-value">
            {world.system} {world.system_version}
            {!world.system_installed && " (не установлена в этой папке)"}
          </div>
        </div>

        <GroupList title="Включённые модули" ids={world.active_modules} missing={world.missing_modules} />
      </Disclosure>
    </article>
  );
}

function GroupList({ title, ids, missing }: { title: string; ids: string[]; missing: string[] }) {
  if (ids.length === 0) return null;
  const absent = new Set(missing);

  return (
    <div className="idlist">
      <div className="group-hint">{title}</div>
      <div className="chips">
        {ids.map((id) => (
          <span className={"chip" + (absent.has(id) ? " chip-missing" : "")} key={id}>
            {id}
          </span>
        ))}
      </div>
    </div>
  );
}

function Sort({
  value,
  onChange,
}: {
  value: "played" | "size" | "title";
  onChange: (v: "played" | "size" | "title") => void;
}) {
  return (
    <div className="seg">
      {(
        [
          ["played", "по последней игре"],
          ["size", "по размеру"],
          ["title", "по названию"],
        ] as const
      ).map(([id, name]) => (
        <label className="seg-opt" key={id}>
          <input
            type="radio"
            name="world-order"
            checked={value === id}
            onChange={() => onChange(id)}
          />
          <span>{name}</span>
        </label>
      ))}
    </div>
  );
}

// Sorting reorders what is already in memory. Nothing is read again, which is
// the point of holding one reading.
function compare(order: "played" | "size" | "title") {
  return (a: InventoryWorld, b: InventoryWorld) => {
    if (order === "size") return b.size.bytes - a.size.bytes;
    if (order === "title") return a.title.localeCompare(b.title, "ru");
    const at = Date.parse(a.last_played) || 0;
    const bt = Date.parse(b.last_played) || 0;
    return bt - at;
  };
}
