import { useState } from "react";
import { InstallsScreen } from "./screens/Installs";
import { PlanScreen } from "./screens/Plan";
import { TransferScreen } from "./screens/Transfer";
import { VerifyScreen } from "./screens/Verify";

type Screen = "installs" | "plan" | "transfer" | "verify";

const TABS: { id: Screen; label: string }[] = [
  { id: "installs", label: "Установки" },
  { id: "plan", label: "План" },
  { id: "transfer", label: "Перенос" },
  { id: "verify", label: "Проверка" },
];

export function App() {
  const [screen, setScreen] = useState<Screen>("plan");

  return (
    <div className="app">
      <nav className="nav">
        <span className="nav-brand">Перенос установки Foundry</span>
        <div className="nav-tabs">
          {TABS.map((t) => (
            <button
              key={t.id}
              className={"nav-tab" + (screen === t.id ? " nav-tab-on" : "")}
              onClick={() => setScreen(t.id)}
            >
              {t.label}
            </button>
          ))}
        </div>
      </nav>

      <main>
        {screen === "installs" && <InstallsScreen onReview={() => setScreen("plan")} />}
        {screen === "plan" && <PlanScreen onStart={() => setScreen("transfer")} />}
        {screen === "transfer" && <TransferScreen onDone={() => setScreen("verify")} />}
        {screen === "verify" && <VerifyScreen />}
      </main>
    </div>
  );
}
