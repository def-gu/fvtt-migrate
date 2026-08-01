import type { ReactNode } from "react";

export function Section({ title, hint, children }: { title: string; hint?: string; children: ReactNode }) {
  return (
    <section className="section">
      <div className="section-head">
        <h2>{title}</h2>
        {hint && <span className="section-hint">{hint}</span>}
      </div>
      {children}
    </section>
  );
}

export function GroupHead({ title, n, hint }: { title: string; n: number; hint?: string }) {
  return (
    <div className="group-head">
      <h3>
        {title} <span className="group-count">{n}</span>
      </h3>
      {hint && <p className="group-hint">{hint}</p>}
    </div>
  );
}

export function Check({
  checked, onChange, label, disabled,
}: { checked: boolean; onChange: (v: boolean) => void; label: string; disabled?: boolean }) {
  return (
    <label className={"check" + (disabled ? " check-off" : "")}>
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span>{label}</span>
    </label>
  );
}

export function Disclosure({ label, children }: { label: string; children: ReactNode }) {
  return (
    <details className="disclosure">
      <summary>{label}</summary>
      <div className="disclosure-body">{children}</div>
    </details>
  );
}

export function Callout({ kicker, title, children, aside }: {
  kicker: string; title: string; children: ReactNode; aside?: ReactNode;
}) {
  return (
    <div className="callout">
      <div className="callout-main">
        <div className="callout-kicker">{kicker}</div>
        <div className="callout-title">{title}</div>
        <p className="callout-body">{children}</p>
      </div>
      {aside && <div className="callout-aside">{aside}</div>}
    </div>
  );
}
