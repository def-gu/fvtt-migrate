import { useState } from "react";
import type { Inventory, Plan } from "../contract";
import type { Destination, State } from "../api";
import { WorldsTab } from "../tabs/Worlds";
import { PackagesTab } from "../tabs/Packages";
import { PlanScreen } from "./Plan";
import { count } from "../format";

type Tab = "worlds" | "systems" | "modules" | "assets";

type Props = {
  inventory: Inventory;
  state: State;
  plan: Plan;
  dest: Destination;
  onDest: (d: Destination) => void;
  onPlan: (p: Plan) => void;
  onStart: () => void;
};

export function TabsScreen({ inventory, state, plan, dest, onDest, onPlan, onStart }: Props) {
  const [tab, setTab] = useState<Tab>("worlds");

  const setWorld = (id: string, include: boolean) =>
    onPlan({ ...plan, worlds: plan.worlds.map((w) => (w.id === id ? { ...w, include } : w)) });

  const tabs: [Tab, string, number][] = [
    ["worlds", "Миры", inventory.worlds.length],
    ["systems", "Игровые системы", inventory.systems.length],
    ["modules", "Модули", inventory.modules.length],
    ["assets", "Файлы и назначение", plan.assets.user_directories.length],
  ];

  return (
    <div className="screen">
      <div className="nav-tabs nav-tabs-wide">
        {tabs.map(([id, name, n]) => (
          <button
            key={id}
            className={"nav-tab" + (tab === id ? " nav-tab-on" : "")}
            onClick={() => setTab(id)}
          >
            {name} <span className="group-count">{count(n)}</span>
          </button>
        ))}
      </div>

      {tab === "worlds" && (
        <WorldsTab inventory={inventory} plan={plan} onInclude={setWorld} />
      )}

      {tab === "systems" && (
        <PackagesTab
          list={inventory.systems}
          kind="system"
          lead="Игровая система нужна каждому миру, который на ней собран. Мир без своей системы на новом месте не откроется."
        />
      )}

      {tab === "modules" && (
        <PackagesTab
          list={inventory.modules}
          kind="module"
          lead="Модули, которые стоят в этой установке. Отмечено, какие миры их включают и откуда каждый берётся."
        />
      )}

      {tab === "assets" && (
        <PlanScreen
          state={state}
          plan={plan}
          dest={dest}
          onDest={onDest}
          onPlan={onPlan}
          onStart={onStart}
        />
      )}
    </div>
  );
}
