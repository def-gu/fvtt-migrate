import type { VerifyResult } from "../contract";
import { count } from "../format";
import { Section } from "../ui";

type Props = { result: VerifyResult; onBack: () => void };

export function VerifyScreen({ result, onBack }: Props) {
  const ok =
    result.missing.length === 0 &&
    result.mismatch.length === 0 &&
    result.worlds.every((w) => w.source_documents === w.target_documents);

  return (
    <div className="screen">
      <header className="screen-head">
        <div>
          <div className="kicker">Проверка</div>
          <h1>{ok ? "Копия совпадает с источником" : "Найдены расхождения"}</h1>
          <p className="lead">
            Файлы могут скопироваться полностью, а мир при этом остаться пустым. Счёт документов
            находит такой случай.
          </p>
        </div>
      </header>

      <Section title="Миры" hint="число документов в источнике и на приёмнике">
        <div className="rows">
          {result.worlds.map((w) => {
            const same = w.source_documents === w.target_documents;
            return (
              <div key={w.id} className={"row" + (same ? "" : " row-bad")}>
                <div className="row-main">
                  <div className="row-title">{w.id}</div>
                  <div className="row-sub">
                    источник {count(w.source_documents)}, приёмник {count(w.target_documents)}
                  </div>
                  {w.failure && <div className="row-blocker">{w.failure}</div>}
                  {!same && w.differing_namespaces && (
                    <div className="row-blocker">
                      На приёмнике отсутствуют {w.differing_namespaces.join(", ")}
                    </div>
                  )}
                </div>
                <span className={same ? "verdict-ok" : "verdict-bad"}>
                  {same ? "совпадает" : "расхождение"}
                </span>
              </div>
            );
          })}
        </div>
      </Section>

      <footer className="footer">
        <div className="footer-text">
          {ok
            ? "Каждый переехавший мир читается с тем же числом документов."
            : "Перенос стоит повторить, он дошлёт недостающее."}
        </div>
        <button className="btn btn-primary" onClick={onBack}>К плану</button>
      </footer>
    </div>
  );
}
