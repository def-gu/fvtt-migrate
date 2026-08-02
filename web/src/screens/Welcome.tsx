import { useEffect, useState } from "react";
import type { Found, Listing } from "../contract";
import * as api from "../api";
import { Section } from "../ui";
import { count } from "../format";

export function WelcomeScreen({ onOpen }: { onOpen: (root: string) => void }) {
  const [found, setFound] = useState<Found[] | null>(null);
  const [listing, setListing] = useState<Listing | null>(null);
  const [typed, setTyped] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.detect().then(setFound).catch((e) => setError(String(e.message ?? e)));
  }, []);

  async function choose(root: string) {
    setBusy(true);
    setError("");
    try {
      await api.open(root);
      onOpen(root);
    } catch (e) {
      setError(String((e as Error).message));
      setBusy(false);
    }
  }

  async function look(path: string) {
    setError("");
    try {
      setListing(await api.browse(path));
    } catch (e) {
      setError(String((e as Error).message));
    }
  }

  return (
    <div className="screen">
      <header className="screen-head">
        <div>
          <div className="kicker">Перенос установки Foundry</div>
          <h1>Выберите установку</h1>
          <p className="lead">
            Нужна папка, внутри которой лежат Data и Config. Если Foundry держит данные
            на другом диске, программа перейдёт туда сама.
          </p>
        </div>
      </header>

      <Section title="Найденные установки" hint="поиск по обычным местам">
        {found === null && <p className="lead">Идёт поиск</p>}
        {found !== null && found.length === 0 && (
          <p className="lead">
            В обычных местах ничего не нашлось. Укажите папку вручную ниже.
          </p>
        )}
        <div className="rows">
          {found?.map((f) => (
            <div className="row" key={f.root}>
              <div className="row-main">
                <div className="row-title">{f.root}</div>
                <div className="row-sub">
                  миров {count(f.worlds)}, систем {count(f.systems)}, модулей{" "}
                  {count(f.modules)}
                </div>
              </div>
              <button className="btn btn-primary" disabled={busy} onClick={() => choose(f.root)}>
                Выбрать
              </button>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Указать вручную" hint="если установка лежит в необычном месте">
        <div className="dest">
          <label className="field">
            <span>Путь до папки с Data и Config</span>
            <input
              className="input"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && typed && choose(typed)}
            />
          </label>
          <button className="btn btn-primary" disabled={busy || !typed} onClick={() => choose(typed)}>
            Открыть
          </button>
          <button className="btn btn-secondary" onClick={() => look(typed)}>
            Обзор
          </button>
        </div>

        {listing && (
          <div className="browser">
            <div className="browser-path">{listing.path}</div>
            <div className="browser-list">
              {listing.roots.map((r) => (
                <button className="browser-item" key={"root" + r} onClick={() => look(r)}>
                  {r}
                </button>
              ))}
              {listing.parent && (
                <button className="browser-item" onClick={() => look(listing.parent)}>
                  Вверх
                </button>
              )}
              {listing.entries.map((e) => (
                <button
                  className={"browser-item" + (e.foundry ? " browser-item-found" : "")}
                  key={e.path}
                  onClick={() => (e.foundry ? choose(e.path) : look(e.path))}
                >
                  <span>{e.name}</span>
                  {e.foundry && <span className="tag tag-accent">установка Foundry</span>}
                </button>
              ))}
            </div>
          </div>
        )}
      </Section>

      {error && (
        <p className="lead verdict-bad" role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
