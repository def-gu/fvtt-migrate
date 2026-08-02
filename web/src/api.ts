import type {
  Plan, StreamEvent, ApplyResult, VerifyResult,
  Inventory, ScanEvent, Found, Listing, Chosen,
} from "./contract";

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

export function detect(): Promise<Found[]> {
  return get("/api/detect");
}

export function browse(path: string): Promise<Listing> {
  return get("/api/browse?path=" + encodeURIComponent(path));
}

export function open(root: string): Promise<Chosen> {
  return post("/api/open", { root });
}

export function inventory(): Promise<Inventory> {
  return get("/api/inventory");
}

export function scan(onEvent: (e: ScanEvent) => void, signal?: AbortSignal): Promise<void> {
  return stream("/api/scan", {}, onEvent, signal);
}

export function run(
  plan: Plan,
  dest: Destination,
  onEvent: (e: RunEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const body = { to: dest.to, token: dest.token, dry_run: dest.dryRun, plan };
  return stream("/api/run", body, onEvent, signal);
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(path);
  if (!res.ok) throw new Error(await message(res));
  return res.json() as Promise<T>;
}

// One line of JSON per event. Reading them as they arrive is what lets a long
// operation show where it is instead of looking stalled.
async function stream<E>(
  path: string,
  body: unknown,
  onEvent: (e: E) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
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
