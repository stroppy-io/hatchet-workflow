import { useMemo, useState } from "react";
import type {
  MachineSpecVM,
  MachineVM,
  ProviderSettingsVM,
} from "@/services/wizard";
import { Yandex_Settings_PlatformId } from "@/services/wizard";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  DiskTypeSelect,
  SliderField,
} from "@/components/ui/sliders";
import {
  closestStep,
  cpuStepsForPlatform,
  diskStepsForType,
  normalizeYandexBootDiskType,
  normalizeYandexDiskGb,
  normalizeYandexInternalIp,
  normalizeYandexNetworkAcceleration,
  platformLimits,
  ramGbSteps,
  YANDEX_NETWORK_ACCELERATIONS,
  YC_PLATFORMS,
} from "@/lib/machine-constraints";
import {
  Box,
  ChevronDown,
  ChevronRight,
  Cpu,
  Database,
  Globe,
  Layers,
  Server,
  Zap,
} from "lucide-react";

const PLATFORM_ENUM_TO_KEY: Record<number, keyof typeof YC_PLATFORMS> = {
  [Yandex_Settings_PlatformId.STANDARD_V2]: "standard-v2",
  [Yandex_Settings_PlatformId.STANDARD_V3]: "standard-v3",
  [Yandex_Settings_PlatformId.HIGHFREQ_V3]: "highfreq-v3",
};

const ROLE_ICON: Record<string, typeof Database> = {
  master: Database,
  primary: Server,
  replica: Database,
  instance: Cpu,
  storage: Database,
  node: Database,
  haproxy: Globe,
  proxysql: Globe,
  etcd: Layers,
  agent: Zap,
  metrics: Server,
};

type MachineBatchChange = { nodeId: string; spec: MachineSpecVM };

interface MachineGroup {
  key: string;
  role: string;
  engine: string;
  providerCase: MachineSpecVM["case"];
  machines: MachineVM[];
}

export function platformKey(settings: ProviderSettingsVM): keyof typeof YC_PLATFORMS {
  if (settings.case !== "yandex") return "standard-v3";
  return PLATFORM_ENUM_TO_KEY[settings.yandex.platformId] ?? "standard-v3";
}

export function machineSpecSummary(spec: MachineSpecVM | undefined): string {
  if (!spec || spec.case === undefined) return "-";
  if (spec.case === "yandex") {
    const y = spec.yandex;
    const t = y.bootDiskType
      .replace("network-ssd-nonreplicated", "ssd-nonrep")
      .replace("network-ssd-io-m3", "io-m3")
      .replace("network-hdd", "hdd")
      .replace("network-ssd", "ssd");
    return `${y.cores} vCPU / ${y.memoryGb} GB / ${y.bootDiskGb} GB ${t}`;
  }
  const d = spec.docker;
  return `${d.cpuCores} cpu / ${(d.memoryMb / 1024).toFixed(d.memoryMb % 1024 ? 1 : 0)} GB`;
}

export function MachinePlanEditor({
  machines,
  settings,
  onMachineChange,
  onMachinesChange,
  compact = false,
}: {
  machines: MachineVM[];
  settings: ProviderSettingsVM;
  onMachineChange: (nodeId: string, spec: MachineSpecVM) => void;
  onMachinesChange?: (updates: MachineBatchChange[]) => void;
  compact?: boolean;
}) {
  const pkey = platformKey(settings);
  const groups = useMemo(() => groupMachines(machines), [machines]);
  const [openGroups, setOpenGroups] = useState<Set<string>>(() => new Set());

  const applyUpdates = (updates: MachineBatchChange[]) => {
    if (updates.length === 0) return;
    if (onMachinesChange) {
      onMachinesChange(updates);
      return;
    }
    for (const u of updates) onMachineChange(u.nodeId, u.spec);
  };

  const toggleGroup = (key: string) => {
    setOpenGroups((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  return (
    <div className="space-y-3">
      {groups.map((group) => {
        const open = openGroups.has(group.key);
        return (
          <MachineGroupPanel
            key={group.key}
            group={group}
            platformKey={pkey}
            compact={compact}
            open={open}
            onToggle={() => toggleGroup(group.key)}
            onGroupChange={(spec) =>
              applyUpdates(group.machines.map((m) => ({ nodeId: m.nodeId, spec })))
            }
            onMachineChange={(nodeId, spec) => applyUpdates([{ nodeId, spec }])}
          />
        );
      })}
    </div>
  );
}

function groupMachines(machines: MachineVM[]): MachineGroup[] {
  const groups: MachineGroup[] = [];
  const byKey = new Map<string, MachineGroup>();
  for (const machine of machines) {
    const role = machine.role || "node";
    const engine = machine.engine || "";
    const providerCase = machine.spec.case;
    const key = `${providerCase ?? "unknown"}:${engine}:${role}`;
    let group = byKey.get(key);
    if (!group) {
      group = { key, role, engine, providerCase, machines: [] };
      byKey.set(key, group);
      groups.push(group);
    }
    group.machines.push(machine);
  }
  return groups;
}

function MachineGroupPanel({
  group,
  platformKey,
  compact,
  open,
  onToggle,
  onGroupChange,
  onMachineChange,
}: {
  group: MachineGroup;
  platformKey: keyof typeof YC_PLATFORMS;
  compact: boolean;
  open: boolean;
  onToggle: () => void;
  onGroupChange: (spec: MachineSpecVM) => void;
  onMachineChange: (nodeId: string, spec: MachineSpecVM) => void;
}) {
  const first = group.machines[0];
  const Icon = ROLE_ICON[group.role] ?? Box;
  const providerLabel = group.providerCase === "yandex" ? "Yandex.Vm" : group.providerCase === "docker" ? "Docker.Container" : "unknown";
  const title = [group.engine, group.role].filter(Boolean).join(" · ") || "nodes";

  return (
    <div className="border border-zinc-800 bg-[#0a0a0a]">
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-zinc-800/70 px-3 py-2.5">
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-2">
            <Icon className="h-4 w-4 shrink-0 text-zinc-500" />
            <span className="min-w-0 truncate text-sm font-medium text-zinc-200">{title}</span>
            <span className="shrink-0 bg-zinc-800/60 px-1.5 py-0.5 font-mono text-[9px] uppercase tracking-wide text-zinc-500">
              {group.machines.length} node{group.machines.length === 1 ? "" : "s"}
            </span>
          </div>
          <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 font-mono text-[10px] text-zinc-600">
            <span>{providerLabel}</span>
            <span>{machineSpecSummary(first?.spec)}</span>
          </div>
        </div>
        <button
          type="button"
          onClick={onToggle}
          className="inline-flex h-7 shrink-0 items-center gap-1.5 border border-zinc-800 px-2 font-mono text-[10px] uppercase tracking-wider text-zinc-500 transition-colors hover:border-zinc-700 hover:text-zinc-300"
        >
          {open ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
          {open ? "Hide nodes" : "Edit nodes"}
        </button>
      </div>

      {first?.spec.case ? (
        <div className="p-3">
          <MachineSpecControls
            spec={first.spec}
            platformKey={platformKey}
            compact={compact}
            onChange={onGroupChange}
          />
        </div>
      ) : (
        <div className="p-3 text-[12px] text-zinc-600">No provider machine settings for this group.</div>
      )}

      {open && (
        <div className="border-t border-zinc-800/70 p-3">
          <div className={compact ? "grid grid-cols-1 gap-2" : "grid grid-cols-1 gap-2 xl:grid-cols-2"}>
            {group.machines.map((m) => (
              <MachineRow
                key={m.nodeId}
                machine={m}
                platformKey={platformKey}
                compact={compact}
                onChange={(spec) => onMachineChange(m.nodeId, spec)}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function MachineRow({
  machine,
  platformKey,
  compact,
  onChange,
}: {
  machine: MachineVM;
  platformKey: keyof typeof YC_PLATFORMS;
  compact: boolean;
  onChange: (spec: MachineSpecVM) => void;
}) {
  const Icon = ROLE_ICON[machine.role] ?? Box;

  return (
    <div className="min-w-0 border border-zinc-800 bg-[#070707] p-2.5">
      <div className="mb-2 flex items-center gap-1.5">
        <Icon className="h-3.5 w-3.5 shrink-0 text-zinc-400" />
        <span className="min-w-0 truncate font-mono text-[11px] text-zinc-300">{machine.nodeId}</span>
        <span className="shrink-0 bg-zinc-800/60 px-1 py-0.5 font-mono text-[9px] uppercase tracking-wide text-zinc-500">
          {machine.role || "node"}
        </span>
        <span className="ml-auto shrink-0 font-mono text-[9px] text-zinc-600">
          {machine.spec.case === "yandex" ? "Yandex.Vm" : "Docker.Container"}
        </span>
      </div>

      <MachineSpecControls
        spec={machine.spec}
        platformKey={platformKey}
        compact={compact}
        onChange={onChange}
      />
    </div>
  );
}

function MachineSpecControls({
  spec,
  platformKey,
  compact,
  onChange,
}: {
  spec: MachineSpecVM;
  platformKey: keyof typeof YC_PLATFORMS;
  compact: boolean;
  onChange: (spec: MachineSpecVM) => void;
}) {
  if (spec.case === "yandex") {
    return (
      <YandexMachineControls
        spec={spec}
        platformKey={platformKey}
        compact={compact}
        onChange={onChange}
      />
    );
  }
  if (spec.case === "docker") {
    return <DockerMachineControls spec={spec} compact={compact} onChange={onChange} />;
  }
  return null;
}

function YandexMachineControls({
  spec,
  platformKey,
  compact,
  onChange,
}: {
  spec: Extract<MachineSpecVM, { case: "yandex" }>;
  platformKey: keyof typeof YC_PLATFORMS;
  compact: boolean;
  onChange: (spec: MachineSpecVM) => void;
}) {
  const limits = platformLimits(platformKey);
  const y = {
    ...spec.yandex,
    bootDiskType: normalizeYandexBootDiskType(spec.yandex.bootDiskType),
    bootDiskGb: normalizeYandexDiskGb(spec.yandex.bootDiskType, spec.yandex.bootDiskGb),
    internalIp: normalizeYandexInternalIp(spec.yandex.internalIp),
    networkAcceleration: normalizeYandexNetworkAcceleration(spec.yandex.networkAcceleration),
  };
  const cpuSteps = cpuStepsForPlatform(platformKey);
  const memorySteps = ramGbSteps(y.cores || cpuSteps[0], limits.maxRamMb);
  const update = (next: typeof y) => onChange({ case: "yandex", yandex: next });

  return (
    <div className="space-y-3">
      <div className={compact ? "grid grid-cols-1 gap-2" : "grid grid-cols-1 gap-2 md:grid-cols-2"}>
        <SliderField
          label="vCPU (cores)"
          value={y.cores}
          steps={cpuSteps}
          onChange={(cores) => {
            const nextRamSteps = ramGbSteps(cores, limits.maxRamMb);
            update({ ...y, cores, memoryGb: closestStep(Math.max(y.memoryGb, nextRamSteps[0]), nextRamSteps) });
          }}
          format={(v) => `${v}`}
        />
        <SliderField
          label="RAM (GB)"
          value={y.memoryGb}
          steps={memorySteps}
          onChange={(memoryGb) => update({ ...y, memoryGb })}
          format={(v) => `${v}`}
        />
      </div>

      <div className={compact ? "grid grid-cols-1 gap-2" : "grid grid-cols-1 gap-2 md:grid-cols-[minmax(0,1fr)_minmax(18rem,24rem)]"}>
        <SliderField
          label="boot disk (GB)"
          value={y.bootDiskGb}
          steps={diskStepsForType(y.bootDiskType)}
          onChange={(bootDiskGb) => update({ ...y, bootDiskGb })}
          format={(v) => `${v}`}
        />
        <DiskTypeSelect
          value={y.bootDiskType}
          diskSizeGb={y.bootDiskGb}
          onChange={(bootDiskType) =>
            update({
              ...y,
              bootDiskType: normalizeYandexBootDiskType(bootDiskType),
              bootDiskGb: normalizeYandexDiskGb(bootDiskType, y.bootDiskGb),
            })
          }
        />
      </div>

      <div className={compact ? "grid grid-cols-1 gap-2" : "grid grid-cols-1 gap-2 md:grid-cols-3"}>
        <div>
          <Label className="text-[9px] font-mono text-zinc-600">zone override</Label>
          <Input
            className="mt-1 h-7 font-mono text-[10px]"
            value={y.zone}
            onChange={(e) => update({ ...y, zone: e.target.value })}
            placeholder="empty = provider zone"
          />
        </div>
        <div>
          <Label className="text-[9px] font-mono text-zinc-600">internal IP</Label>
          <Input
            className="mt-1 h-7 font-mono text-[10px]"
            value={y.internalIp}
            onChange={(e) => update({ ...y, internalIp: e.target.value || "auto" })}
            onBlur={(e) => update({ ...y, internalIp: normalizeYandexInternalIp(e.target.value) })}
            placeholder="auto"
          />
        </div>
        <div className="flex items-end gap-2">
          <button
            type="button"
            onClick={() => update({ ...y, publicIp: !y.publicIp })}
            className={`h-7 flex-1 border px-2 font-mono text-[10px] transition-colors ${
              y.publicIp
                ? "border-primary/40 bg-primary/[0.08] text-primary"
                : "border-zinc-800 text-zinc-500 hover:border-zinc-700"
            }`}
          >
            public IP {y.publicIp ? "on" : "off"}
          </button>
        </div>
      </div>

      <div>
        <Label className="text-[9px] font-mono text-zinc-600">network acceleration</Label>
        <div className="mt-1 grid gap-1.5 sm:grid-cols-2">
          {YANDEX_NETWORK_ACCELERATIONS.map((item) => {
            const active = y.networkAcceleration === item.value;
            return (
              <button
                key={item.value}
                type="button"
                onClick={() => update({ ...y, networkAcceleration: item.value })}
                className={`border px-2 py-1.5 text-left transition-colors ${
                  active ? "border-primary/40 bg-primary/[0.06]" : "border-zinc-800 hover:border-zinc-700"
                }`}
              >
                <div className={`font-mono text-[10px] ${active ? "text-primary" : "text-zinc-400"}`}>{item.label}</div>
                <div className="text-[9px] text-zinc-600">{item.hint}</div>
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function DockerMachineControls({
  spec,
  compact,
  onChange,
}: {
  spec: Extract<MachineSpecVM, { case: "docker" }>;
  compact: boolean;
  onChange: (spec: MachineSpecVM) => void;
}) {
  const d = spec.docker;
  return (
    <div className="space-y-2">
      <div>
        <Label className="text-[9px] font-mono text-zinc-600">image</Label>
        <Input
          className="mt-1 h-7 font-mono text-[10px]"
          value={d.image}
          onChange={(e) => onChange({ case: "docker", docker: { ...d, image: e.target.value } })}
          placeholder="postgres:16"
        />
      </div>
      <div className={compact ? "grid grid-cols-1 gap-2" : "grid grid-cols-1 gap-2 sm:grid-cols-2"}>
        <SliderField
          label="cpu cores"
          value={d.cpuCores}
          steps={[1, 2, 4, 8, 12, 16, 24, 32]}
          onChange={(cpuCores) => onChange({ case: "docker", docker: { ...d, cpuCores } })}
          format={(v) => `${v}`}
        />
        <SliderField
          label="memory (MB)"
          value={d.memoryMb}
          steps={[512, 1024, 2048, 4096, 8192, 16384, 32768, 65536]}
          onChange={(memoryMb) => onChange({ case: "docker", docker: { ...d, memoryMb } })}
          format={(v) => `${v}`}
        />
      </div>
    </div>
  );
}
