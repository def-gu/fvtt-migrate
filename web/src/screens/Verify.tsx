import { count } from "../format";
import { Section } from "../ui";
import { VERIFY } from "../sample";

export function VerifyScreen() {
  const ok = VERIFY.missing.length === 0 && VERIFY.mismatch.length === 0 &&
    VERIFY.worlds.every((w) => w.source_documents === w.target_documents);

  return (
    <div className="screen">
      <header className="screen-head">
        <div>
          <div className="kicker">Проверка</div>
          <h1>{ok ? "Перенос совпадает с источником" : "Найдены расхождения"}</h1>
          <p className="lead">
            Проверка запускается сама после переноса. Файлы могут скопироваться полностью, а мир
            при этом остаться пустым. Это единственная проверка, которая находит такой случай.
          </p>
        </div>
      </header>

      <Section title="Миры" hint="число документов в источнике и на приёмнике">
        <div className="rows">
          {VERIFY.worlds.map((w) => {
            const same = w.source_documents === w.target_documents;
            return (
              <div key={w.id} className={"row" + (same ? "" : " row-bad")}>
                <div className="row-main">
                  <div className="row-title">{w.id}</div>
                  <div className="row-sub">
                    источник {count(w.source_documents)}, приёмник {count(w.target_documents)}
                  </div>
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

      <Section title="Файлы">
        <div className="stats">
          <div>
            <b>{count(VERIFY.missing.length)}</b>
            <span>путей</span>
            <em>отсутствуют на приёмнике</em>
          </div>
          <div>
            <b>{count(VERIFY.mismatch.length)}</b>
            <span>путей</span>
            <em>содержимое различается</em>
          </div>
        </div>
      </Section>

      <footer className="footer">
        <div className="footer-text">
          Всё, что было выбрано в плане, присутствует и читается так же, как в источнике.
        </div>
      </footer>
    </div>
  );
}
