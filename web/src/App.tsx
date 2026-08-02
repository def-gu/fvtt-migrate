import { useCallback, useEffect, useState } from "react";
import type { Inventory, Plan, VerifyResult } from "./contract";
import * as api from "./api";
import { WelcomeScreen } from "./screens/Welcome";
import { ScanningScreen } from "./screens/Scanning";
import { TabsScreen } from "./screens/Tabs";
import { TransferScreen } from "./screens/Transfer";
import { VerifyScreen } from "./screens/Verify";

type Stage = "welcome" | "scanning" | "plan" | "transfer" | "verify";

export function App() {
  const [stage, setStage] = useState<Stage>("welcome");
  const [root, setRoot] = useState("");
  const [inventory, setInventory] = useState<Inventory | null>(null);
  const [state, setState] = useState<api.State | null>(null);
  const [plan, setPlan] = useState<Plan | null>(null);
  const [dest, setDest] = useState<api.Destination>({ to: "", token: "", dryRun: false });
  const [result, setResult] = useState<VerifyResult | null>(null);
  const [error, setError] = useState("");

  const scanned = useCallback(async (inv: Inventory) => {
    setInventory(inv);
    setRoot(inv.root);
    try {
      const s = await api.loadState();
      setState(s);
      setPlan(await api.buildPlan(s.targets?.[0] ?? "", false));
      setStage("plan");
    } catch (e) {
      setError(String((e as Error).message));
    }
  }, []);

  // A panel started with an installation already read skips the picker.
  useEffect(() => {
    if (stage !== "welcome") return;
    fetch("/api/inventory")
      .then((res) => (res.ok ? res.json() : null))
      .then((inv: Inventory | null) => {
        if (inv) scanned(inv);
      })
      .catch(() => undefined);
  }, [stage, scanned]);

  if (error) {
    return (
      <div className="screen">
        <div className="kicker">{root}</div>
        <h1>Не удалось прочитать установку</h1>
        <p className="lead verdict-bad">{error}</p>
        <button
          className="btn btn-primary"
          onClick={() => {
            setError("");
            setStage("welcome");
          }}
        >
          Выбрать другую папку
        </button>
      </div>
    );
  }

  if (stage === "welcome") {
    return (
      <WelcomeScreen
        onOpen={(chosen) => {
          setRoot(chosen);
          setStage("scanning");
        }}
      />
    );
  }

  if (stage === "scanning") {
    return <ScanningScreen root={root} onDone={scanned} />;
  }

  if (!state || !plan || !inventory) {
    return (
      <div className="screen">
        <p className="lead">Чтение установки</p>
      </div>
    );
  }

  if (inventory.worlds.length === 0) {
    return (
      <div className="screen">
        <div className="kicker">{inventory.root}</div>
        <h1>В этой установке нет миров</h1>
        <p className="lead">
          Программа прочитала {inventory.root} и не нашла там ни одного мира. Настоящие
          данные Foundry могут лежать на другом диске.
        </p>
        <button className="btn btn-primary" onClick={() => setStage("welcome")}>
          Выбрать другую папку
        </button>
      </div>
    );
  }

  return (
    <div className="app">
      <nav className="nav">
        <span className="nav-brand">Перенос установки Foundry</span>
        <span className="nav-root">{inventory.root}</span>
        <button className="btn btn-secondary" onClick={() => setStage("welcome")}>
          Другая установка
        </button>
      </nav>

      <main>
        {stage === "plan" && (
          <TabsScreen
            inventory={inventory}
            state={state}
            plan={plan}
            dest={dest}
            onDest={setDest}
            onPlan={setPlan}
            onStart={() => setStage("transfer")}
          />
        )}
        {stage === "transfer" && (
          <TransferScreen
            plan={plan}
            dest={dest}
            onBack={() => setStage("plan")}
            onDone={async () => {
              try {
                setResult(await api.verify(dest));
              } catch (e) {
                setError(String((e as Error).message));
                return;
              }
              setStage("verify");
            }}
          />
        )}
        {stage === "verify" && result && (
          <VerifyScreen result={result} onBack={() => setStage("plan")} />
        )}
      </main>
    </div>
  );
}
