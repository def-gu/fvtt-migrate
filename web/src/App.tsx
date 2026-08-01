import { useEffect, useState } from "react";
import type { Plan, VerifyResult } from "./contract";
import * as api from "./api";
import { PlanScreen } from "./screens/Plan";
import { TransferScreen } from "./screens/Transfer";
import { VerifyScreen } from "./screens/Verify";

type Stage = "loading" | "plan" | "transfer" | "verify";

export function App() {
  const [stage, setStage] = useState<Stage>("loading");
  const [state, setState] = useState<api.State | null>(null);
  const [plan, setPlan] = useState<Plan | null>(null);
  const [dest, setDest] = useState<api.Destination>({ to: "", token: "", dryRun: false });
  const [result, setResult] = useState<VerifyResult | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .loadState()
      .then(async (s) => {
        setState(s);
        setPlan(await api.buildPlan(s.targets[0] ?? "", false));
        setStage("plan");
      })
      .catch((e) => setError(String(e.message ?? e)));
  }, []);

  if (error) {
    return (
      <div className="screen">
        <h1>Не удалось прочитать установку</h1>
        <p className="lead">{error}</p>
      </div>
    );
  }

  if (stage === "loading" || !state || !plan) {
    return (
      <div className="screen">
        <p className="lead">Чтение установки</p>
      </div>
    );
  }

  return (
    <div className="app">
      <nav className="nav">
        <span className="nav-brand">Перенос установки Foundry</span>
        <span className="nav-root">{state.root}</span>
      </nav>

      <main>
        {stage === "plan" && (
          <PlanScreen
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
