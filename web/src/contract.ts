// Shapes emitted by `fvtt-migrate --json`. Field names are the wire names and
// must not be renamed here.

export type Bucket = { files: number; bytes: number };

export type Recommendation = "none" | "keep" | "upgrade" | "required" | "blocked";
export type PackageSource = "server-cache" | "manifest" | "upload";
export type Policy = "pin" | "upgrade" | "latest";
export type Compat = "ok" | "untested" | "incompatible" | "unknown";

export type World = {
  id: string;
  system: string;
  system_version: string;
  core_version: string;
  system_installed: boolean;
  include: boolean;
  blocker?: string;
};

export type Package = {
  id: string;
  kind: "module" | "system";
  version: string;
  source: PackageSource;
  reason?: string;
  policy: Policy;
  premium?: boolean;
  compat_declared: Compat;
  observed_on?: string;
  available?: string;
  compat_available?: Compat;
  recommend: Recommendation;
  widens_support?: boolean;
  manifest_url?: string;
  entitled_by: string;
};

export type Directory = {
  path: string;
  files: number;
  bytes: number;
  action: "include" | "skip";
  note?: string;
  looks_stale?: boolean;
  broken_references_into_it?: number;
};

export type Plan = {
  format: { version: number; capabilities: string[] };
  identity: { tenant?: string; device?: string; installation?: string };
  source: { root: string; os: string; target_core_version: string; package_policy: Policy };
  worlds: World[];
  packages: Package[];
  assets: {
    referenced: Bucket;
    in_packages: Bucket;
    user_directories: Directory[];
    broken_references: number;
    case_only_matches: number;
  };
};

export type ProgressPhase =
  | "hashing"
  | "negotiating"
  | "transferring"
  | "placing"
  | "verifying";

export type StreamEvent =
  | { type: "progress"; phase: ProgressPhase; done: number; total: number; bytes?: number; detail?: string }
  | { type: "notice"; level: "note" | "warning"; code: string; message: string };

export type ApplyResult = {
  selected_files: number;
  unique_blobs: number;
  transferred_blobs: number;
  transferred_bytes: number;
  placed_files: number;
  already_present_blobs: number;
  already_present_bytes: number;
  would_send_blobs?: number;
  would_send_bytes?: number;
};

export type VerifyResult = {
  missing: string[];
  mismatch: string[];
  rehashed: number;
  worlds: {
    id: string;
    source_documents: number;
    target_documents: number;
    differing_namespaces?: string[];
    failure?: string;
  }[];
};

export type InstallStatus = "idle" | "running" | "world-loaded" | "migrating" | "unreachable";

export type Installation = {
  id: string;
  title: string;
  machine: string;
  core_version: string;
  worlds: number;
  bytes: number;
  status: InstallStatus;
};
