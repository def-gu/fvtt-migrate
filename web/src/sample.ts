import type {
  Compat,
  Directory,
  Installation,
  Package,
  Plan,
  Recommendation,
  VerifyResult,
  World,
} from "./contract";

export const TARGETS = [
  { version: "13.351", hint: "как в источнике" },
  { version: "14.363", hint: "следующее поколение" },
];

type WorldSeed = World & { blockers: Record<string, string> };

const WORLDS: WorldSeed[] = [
  {
    id: "puti-azlanti", system: "pf2e", system_version: "7.7.4", core_version: "13.351",
    system_installed: true, include: true, blockers: {},
  },
  {
    id: "arka-i-olivkovaya-vetv", system: "pf2e", system_version: "7.12.2", core_version: "13.351",
    system_installed: true, include: true, blockers: {},
  },
  {
    id: "golarion-travel-guide", system: "pf2e", system_version: "7.12.2", core_version: "13.351",
    system_installed: true, include: true, blockers: {},
  },
  {
    id: "arka-i-zelenyj-skripach", system: "CoC7", system_version: "8.10", core_version: "13.351",
    system_installed: true, include: true,
    blockers: { "14.363": "система CoC7 не собрана под Foundry 14" },
  },
  {
    id: "dungeon-and-stone-reality-edition", system: "dungeon-stone", system_version: "1.1.1",
    core_version: "13.351", system_installed: false, include: false,
    blockers: { "13.351": "система dungeon-stone не установлена", "14.363": "система dungeon-stone не установлена" },
  },
  {
    id: "germes", system: "cyberpunk2020", system_version: "1.1.0", core_version: "13.351",
    system_installed: false, include: false,
    blockers: { "13.351": "система cyberpunk2020 не установлена", "14.363": "система cyberpunk2020 не установлена" },
  },
];

const names = (base: string[], total: number, prefix: string) => {
  const out = base.slice(0, total);
  for (let i = 1; out.length < total; i++) out.push(`${prefix}-${i}`);
  return out;
};

const WIDENING = names(
  ["Pf2eItemGenerator", "Pf2eNpcMaker", "annomicon", "coc7-qol", "item-piles", "sequencer",
   "pf2e-dorako-ui", "pf2e-leveler", "pf2e-relics", "quick-insert", "pf2e-ranged-combat",
   "pf2e-modifiers-matter", "pf2e-monster-parts", "pf2e-avoid-notice", "pf2e-afflictioner",
   "pf2e-item-activations", "pf2e-prepper", "pf2e-subsystems", "kctg-2e", "sundry"],
  33, "widening",
);

const PINNED = names(
  ["_chatcommands", "autoanimations", "betterroofs", "dice-so-nice", "dfreds-droppables",
   "filepicker-plus", "foundry-taskbar", "gatherer", "hexplorer", "hover-distance",
   "image-context", "journal-page-popout", "mastercrafted", "media-optimizer",
   "monks-active-tiles", "pf2e-dailies", "pf2e-hud", "pf2e-toolbelt", "portal-lib",
   "recycle-bin", "skill-tree", "smarttarget", "token-notes", "xdy-pf2e-workbench"],
  55, "pinned",
);

const MANUAL = names(
  ["action-container", "globe-forge-spike", "simple-quest", "sound-engine-master",
   "multi-tokenart", "quick-doors", "puzzle-locks", "tokenflip", "progress-tracker",
   "rich-info-tooltips", "quickdraw", "inactive-tokens-lmao"],
  20, "manual",
);

const PREMIUM = names(
  ["pf2e-kingmaker", "pf2e-ap196-199-season-of-ghosts", "pf2e-ap197-let-the-leaves-fall",
   "pf2e-decks-harrow", "pf2e-tokens-characters", "pf2e-tokens-monster-core",
   "pf2e-tokens-npc-core", "pf2e-tokens-myth-and-magic", "pf2e-mercenary-marketplace-vol1"],
  12, "premium",
);

const SETTLED = names(
  ["pf2e", "CoC7", "lib-wrapper", "socketlib", "babele", "ru-ru", "dice-calculator",
   "compact-scene-navigation", "dynamic-soundscapes", "chat-stickers"],
  42, "settled",
);

const MANUAL_REASONS = [
  "манифест отсутствует, локальная разработка",
  "источник отдаёт версию 5.1.3 вместо установленной",
  "по адресу манифеста находится веб-страница",
  "адрес манифеста не отвечает",
];

const bump = (v: string, step: number) => {
  const parts = v.split(".").map(Number);
  parts[2] = (parts[2] ?? 0) + step;
  return parts.join(".");
};

const version = (i: number) => `${1 + (i % 4)}.${(i * 3) % 12}.${(i * 5) % 20}`;

type Seed = Package & { group: "widening" | "pinned" | "manual" | "premium" | "settled" };

const PACKAGES: Seed[] = [];

WIDENING.forEach((id, i) => {
  const v = i === 0 ? "2.5.11" : version(i);
  PACKAGES.push({
    id, kind: "module", version: v, source: "manifest", policy: "pin",
    compat_declared: "ok", observed_on: "13.351",
    available: i === 0 ? "2.5.40" : bump(v, 4 + (i % 9)),
    compat_available: "ok", recommend: "upgrade", widens_support: true,
    entitled_by: "present-in-source-install", group: "widening",
  });
});

PINNED.forEach((id, i) => {
  const v = i === 0 ? "2.0.5" : version(i + 30);
  PACKAGES.push({
    id, kind: "module", version: v, source: "manifest", policy: "pin",
    compat_declared: "ok", observed_on: "13.351",
    available: `${Number(v.split(".")[0]) + 1}.0.1`,
    compat_available: "incompatible", recommend: "keep",
    entitled_by: "present-in-source-install", group: "pinned",
  });
});

MANUAL.forEach((id, i) => {
  PACKAGES.push({
    id, kind: "module", version: i === 0 ? "0.1.0" : version(i + 7),
    source: "upload", reason: MANUAL_REASONS[i % MANUAL_REASONS.length],
    policy: "pin", compat_declared: "ok", observed_on: "13.351",
    recommend: "none", entitled_by: "present-in-source-install", group: "manual",
  });
});

PREMIUM.forEach((id, i) => {
  PACKAGES.push({
    id, kind: "module", version: version(i + 11), source: "upload", premium: true,
    reason: "платный контент", policy: "pin", compat_declared: "ok", observed_on: "13.351",
    recommend: "none", entitled_by: "present-in-source-install", group: "premium",
  });
});

SETTLED.forEach((id, i) => {
  PACKAGES.push({
    id, kind: id === "pf2e" || id === "CoC7" ? "system" : "module",
    version: id === "pf2e" ? "7.12.2" : version(i + 50),
    source: "manifest", policy: "pin", compat_declared: "ok", observed_on: "13.351",
    recommend: "none", entitled_by: "present-in-source-install", group: "settled",
  });
});

const DIRECTORIES: Directory[] = [
  {
    path: "Карты", files: 128, bytes: 5351903806, action: "include", looks_stale: true,
    broken_references_into_it: 71,
    note: "ссылок на файлы нет, но битые ссылки ведут внутрь этой папки",
  },
  { path: "chat-stickers", files: 11, bytes: 749234, action: "skip" },
  { path: "multi-tokenart", files: 17, bytes: 515072, action: "skip" },
  { path: "sem_library", files: 1, bytes: 71782, action: "skip" },
];

const recommendFor = (p: Seed, target: string): { recommend: Recommendation; compat_available?: Compat } => {
  if (target === "13.351") return { recommend: p.recommend, compat_available: p.compat_available };
  switch (p.group) {
    case "widening":
      return { recommend: "upgrade", compat_available: "ok" };
    case "pinned":
      return { recommend: "required", compat_available: "ok" };
    case "settled":
      return { recommend: "none" };
    default:
      return { recommend: "none" };
  }
};

export function planFor(target: string): Plan {
  return {
    format: { version: 1, capabilities: ["plan/1", "digest/blake3-256", "transfer/whole-file"] },
    identity: {},
    source: {
      root: "/home/gm/fvtt-data-1", os: "linux",
      target_core_version: target, package_policy: "pin",
    },
    worlds: WORLDS.map((w) => {
      const blocker = w.blockers[target];
      return {
        id: w.id, system: w.system, system_version: w.system_version,
        core_version: w.core_version, system_installed: w.system_installed,
        include: !blocker, blocker,
      };
    }),
    packages: PACKAGES.map((p) => {
      const { group: _group, ...rest } = p;
      return { ...rest, ...recommendFor(p, target) };
    }),
    assets: {
      referenced: { files: 5120, bytes: 2899419277 },
      in_packages: { files: 34442, bytes: 3670755365 },
      user_directories: DIRECTORIES,
      broken_references: 3099,
      case_only_matches: 0,
    },
  };
}

export const TOTAL = { files: 26897, bytes: 11488862208 };

export const INSTALLATIONS: Installation[] = [
  { id: "home", title: "Домашний компьютер", machine: "ASUS TX7", core_version: "13.351", worlds: 6, bytes: 11488862208, status: "idle" },
  { id: "vps", title: "Сервер", machine: "vds-1", core_version: "13.351", worlds: 0, bytes: 0, status: "running" },
  { id: "test", title: "Тестовая установка", machine: "vds-1", core_version: "14.363", worlds: 6, bytes: 3556769, status: "world-loaded" },
];

export const VERIFY: VerifyResult = {
  missing: [], mismatch: [], rehashed: 0,
  worlds: [
    { id: "arka-i-olivkovaya-vetv", source_documents: 66283, target_documents: 66283 },
    { id: "arka-i-zelenyj-skripach", source_documents: 5650, target_documents: 5650 },
    { id: "golarion-travel-guide", source_documents: 69671, target_documents: 69671 },
    { id: "puti-azlanti", source_documents: 57078, target_documents: 57078 },
  ],
};
