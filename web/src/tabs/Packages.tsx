import { useMemo, useState } from "react";
import type { Delivery, InventoryPackage } from "../contract";
import { bytes, count, plural } from "../format";
import { Disclosure } from "../ui";

type Order = "size" | "title" | "author" | "system";

const delivery: Record<Delivery, string> = {
  open: "скачивается по ссылке",
  store: "куплен в магазине Foundry",
  carry: "едет файлами, скачать неоткуда",
};

type Props = {
  list: InventoryPackage[];
  kind: "system" | "module";
  lead: string;
};

export function PackagesTab({ list, kind, lead }: Props) {
  const [order, setOrder] = useState<Order>("size");
  const [group, setGroup] = useState(false);

  const sorted = useMemo(() => [...list].sort(compare(order)), [list, order]);
  const groups = useMemo(() => (group ? byAuthor(sorted) : [["", sorted] as const]), [sorted, group]);

  const paid = list.filter((p) => p.delivery !== "open").length;

  return (
    <div className="tab">
      <div className="tab-head">
        <div>
          <p className="lead">{lead}</p>
          <p className="row-sub">
            {count(list.length)} {plural(list.length, "пакет", "пакета", "пакетов")}, из них{" "}
            {count(paid)} без открытой ссылки на скачивание
          </p>
        </div>
        <div className="tab-controls">
          <Sort value={order} onChange={setOrder} kind={kind} />
          <label className="check">
            <input type="checkbox" checked={group} onChange={(e) => setGroup(e.target.checked)} />
            <span>группировать по авторам</span>
          </label>
        </div>
      </div>

      {groups.map(([author, items]) => (
        <section key={author || "all"}>
          {author && (
            <div className="group-head">
              <h3>
                {author} <span className="group-count">{items.length}</span>
              </h3>
            </div>
          )}
          <div className="rows">
            {items.map((p) => (
              <PackageRow key={p.id} pkg={p} kind={kind} />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}

function PackageRow({ pkg, kind }: { pkg: InventoryPackage; kind: "system" | "module" }) {
  const used = pkg.used_by_worlds.length;

  return (
    <div className="row">
      <div className="row-main">
        <div className="row-title">{pkg.title}</div>
        <div className="row-sub">
          {pkg.id} {pkg.version && "версия " + pkg.version}
          {pkg.authors.length > 0 && " " + pkg.authors.join(", ")}
        </div>

        <Disclosure label="Подробности">
          <div className="pairs">
            <div className="pair-key">Папка</div>
            <div className="pair-value">{pkg.path}</div>
            <div className="pair-key">Откуда берётся</div>
            <div className="pair-value">{delivery[pkg.delivery]}</div>
            {pkg.verified_core && (
              <>
                <div className="pair-key">Проверен на Foundry</div>
                <div className="pair-value">{pkg.verified_core}</div>
              </>
            )}
            {kind === "module" && pkg.target_systems.length > 0 && (
              <>
                <div className="pair-key">Для систем</div>
                <div className="pair-value">{pkg.target_systems.join(", ")}</div>
              </>
            )}
            {kind === "system" && pkg.module_count ? (
              <>
                <div className="pair-key">Модулей рассчитано на неё</div>
                <div className="pair-value">{count(pkg.module_count)}</div>
              </>
            ) : null}
            {pkg.requires.length > 0 && (
              <>
                <div className="pair-key">Требует</div>
                <div className="pair-value">{pkg.requires.join(", ")}</div>
              </>
            )}
            {pkg.missing_requirements.length > 0 && (
              <>
                <div className="pair-key">Не установлено из требуемого</div>
                <div className="pair-value">{pkg.missing_requirements.join(", ")}</div>
              </>
            )}
          </div>
          {used > 0 && (
            <div className="chips">
              {pkg.used_by_worlds.map((id) => (
                <span className="chip" key={id}>
                  {id}
                </span>
              ))}
            </div>
          )}
        </Disclosure>
      </div>

      <div className="row-sub">
        {used > 0
          ? `${count(used)} ${plural(used, "мир", "мира", "миров")}`
          : "ни одним миром не включён"}
      </div>
      <div className="row-sub">{bytes(pkg.size.bytes)}</div>
      {pkg.delivery !== "open" && <span className="tag tag-neutral">{delivery[pkg.delivery]}</span>}
    </div>
  );
}

function Sort({
  value,
  onChange,
  kind,
}: {
  value: Order;
  onChange: (v: Order) => void;
  kind: "system" | "module";
}) {
  const options: [Order, string][] = [
    ["size", "по размеру"],
    ["title", "по названию"],
    ["author", "по автору"],
  ];
  if (kind === "module") options.push(["system", "по системе"]);

  return (
    <div className="seg">
      {options.map(([id, name]) => (
        <label className="seg-opt" key={id}>
          <input
            type="radio"
            name={"order-" + kind}
            checked={value === id}
            onChange={() => onChange(id)}
          />
          <span>{name}</span>
        </label>
      ))}
    </div>
  );
}

// Sorting and grouping work on the list already in memory, so changing them
// costs nothing and never reads the disk again.
function compare(order: Order) {
  return (a: InventoryPackage, b: InventoryPackage) => {
    if (order === "size") return b.size.bytes - a.size.bytes;
    if (order === "author") return author(a).localeCompare(author(b), "ru");
    if (order === "system") return system(a).localeCompare(system(b), "ru");
    return a.title.localeCompare(b.title, "ru");
  };
}

function author(p: InventoryPackage): string {
  return p.authors[0] ?? "";
}

function system(p: InventoryPackage): string {
  return p.target_systems[0] ?? "";
}

function byAuthor(list: InventoryPackage[]): (readonly [string, InventoryPackage[]])[] {
  const groups = new Map<string, InventoryPackage[]>();
  for (const p of list) {
    const key = author(p) || "автор не указан";
    const held = groups.get(key);
    if (held) held.push(p);
    else groups.set(key, [p]);
  }
  return [...groups.entries()]
    .sort((a, b) => b[1].length - a[1].length)
    .map(([k, v]) => [k, v] as const);
}
