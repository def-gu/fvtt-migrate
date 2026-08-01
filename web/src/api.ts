import type { Plan, StreamEvent, ApplyResult, VerifyResult } from "./contract";

export type State = {
  root: string;
  targets: string[];
  scan: {
    root: string;
    counts: { worlds: number; systems: number; modules: number; files: number };
    assets: {
      referenced: { files: number; bytes: number };
      in_packages: { files: number; bytes: number };
      orphaned: { files: number; bytes: number };
      built_in_references: number;
      orphan_directories: {
        path: string; files: number; bytes: number;
        broken_references_into_it: number; looks_stale: boolean;
      }[];
    };
    broken_references: { path: string; references: number; first_seen_at: string }[];
  };
};

export type RunEvent =
  | StreamEvent
  | { type: "result"; result: ApplyResult }
  | { type: "failed"; message: string };

export type Destination = { to: string; token: string; dryRun: boolean };

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(await message(res));
  return res.json() as Promise<T>;
}

async function message(res: Response): Promise<string> {
  try {
    const body = await res.json();
    if (body && typeof body.message === "string") return body.message;
  } catch {
    // falls through to the status line
  }
  return `Сервер ответил ${res.status}`;
}

export async function loadState(): Promise<State> {
  const res = await fetch("/api/state");
  if (!res.ok) throw new Error(await message(res));
  return res.json();
}

export function buildPlan(target: string, checkUpdates: boolean): Promise<Plan> {
  return post("/api/plan", { target_core: target, check_updates: checkUpdates });
}

export function verify(dest: Destination): Promise<VerifyResult> {
  return post("/api/verify", { to: dest.to, token: dest.token });
}

export async function run(
  plan: Plan,
  dest: Destination,
  onEvent: (e: RunEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch("/api/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ to: dest.to, token: dest.token, dry_run: dest.dryRun, plan }),
    signal,
  });
  if (!res.ok) throw new Error(await message(res));
  if (!res.body) throw new Error("Ответ пришёл без содержимого");

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    let cut: number;
    while ((cut = buffer.indexOf("\n")) >= 0) {
      const line = buffer.slice(0, cut).trim();
      buffer = buffer.slice(cut + 1);
      if (line) onEvent(JSON.parse(line));
    }
  }
}
