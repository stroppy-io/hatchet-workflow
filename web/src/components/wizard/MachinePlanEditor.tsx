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
  diskStepsForType,
  NumericSlider,
  platformLimits,
  SliderField,
  YC_PLATFORMS,
} from "@/components/ui/sliders";
import { Box, Cpu, Database, Globe, Layers, Server, Zap } from "lucide-react";

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

export function platformKey(settings: ProviderSettingsVM): keyof typeof YC_PLATFORMS {
  if (settings.case !== "yandex") return "standard-v3";
  return PLATFORM_ENUM_TO_KEY[settings.yandex.platformId] ?? "standard-v3";
}

export function machineSpecSummary(spec: MachineSpecVM | undefined): string {
  if (!spec || spec.case === undefined) return "-";
  if (spec.case === "yandex") {
    const y = spec.yandex;
    const t = y.bootDiskType.replace("network-ssd-io-m3", "io-m3").replace("network-ssd", "ssd");
    return `${y.cores} vCPU / ${y.memoryGb} GB / ${y.bootDiskGb} GB ${t}`;
  }
  const d = spec.docker;
  return `${d.cpuCores} cpu / ${(d.memoryMb / 1024).toFixed(d.memoryMb % 1024 ? 1 : 0)} GB`;
}

export function MachinePlanEditor({
  machines,
  settings,
  onMachineChange,
}: {
  machines: MachineVM[];
  settings: ProviderSettingsVM;
  onMachineChange: (nodeId: string, spec: MachineSpecVM) => void;
}) {
  const pkey = platformKey(settings);
  return (
    <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
      {machines.map((m) => (
        <MachineRow
          key={m.nodeId}
          machine={m}
          platformKey={pkey}
          onChange={(spec) => onMachineChange(m.nodeId, spec)}
        />
      ))}
    </div>
  );
}

function MachineRow({
  machine,
  platformKey,
  onChange,
}: {
  machine: MachineVM;
  platformKey: keyof typeof YC_PLATFORMS;
  onChange: (spec: MachineSpecVM) => void;
}) {
  const Icon = ROLE_ICON[machine.role] ?? Box;
  const limits = platformLimits(platformKey);
  const spec = machine.spec;

  return (
    <div className="border border-zinc-800 bg-[#0a0a0a] p-2.5">
      <div className="mb-2 flex items-center gap-1.5">
        <Icon className="h-3.5 w-3.5 shrink-0 text-zinc-400" />
        <span className="min-w-0 truncate font-mono text-[11px] text-zinc-300">{machine.nodeId}</span>
        <span className="shrink-0 bg-zinc-800/60 px-1 py-0.5 font-mono text-[9px] uppercase tracking-wide text-zinc-500">
          {machine.role}
        </span>
        <span className="ml-auto shrink-0 font-mono text-[9px] text-zinc-600">
          {spec.case === "yandex" ? "Yandex.Vm" : "Docker.Container"}
        </span>
      </div>

      {spec.case === "yandex" && (
        <div className="space-y-2">
          <div className="grid grid-cols-2 gap-2">
            <NumericSlider
              label="vCPU (cores)"
              value={spec.yandex.cores}
              min={2}
              max={limits.maxCores}
              onChange={(v) => onChange({ case: "yandex", yandex: { ...spec.yandex, cores: v } })}
            />
            <NumericSlider
              label="RAM (GB)"
              value={spec.yandex.memoryGb}
              min={1}
              max={Math.round(limits.maxRamMb / 1024)}
              onChange={(v) => onChange({ case: "yandex", yandex: { ...spec.yandex, memoryGb: v } })}
            />
          </div>
          <SliderField
            label="boot disk (GB)"
            value={spec.yandex.bootDiskGb}
            steps={diskStepsForType(spec.yandex.bootDiskType)}
            onChange={(v) => onChange({ case: "yandex", yandex: { ...spec.yandex, bootDiskGb: v } })}
            format={(v) => `${v}`}
          />
          <DiskTypeSelect
            value={spec.yandex.bootDiskType}
            diskSizeGb={spec.yandex.bootDiskGb}
            onChange={(v) => onChange({ case: "yandex", yandex: { ...spec.yandex, bootDiskType: v } })}
          />
        </div>
      )}

      {spec.case === "docker" && (
        <div className="space-y-2">
          <div>
            <Label className="text-[9px] font-mono text-zinc-600">image</Label>
            <Input
              className="mt-1 h-6 font-mono text-[10px]"
              value={spec.docker.image}
              onChange={(e) => onChange({ case: "docker", docker: { ...spec.docker, image: e.target.value } })}
              placeholder="postgres:16"
            />
          </div>
          <div className="grid grid-cols-2 gap-2">
            <SliderField
              label="cpu cores"
              value={spec.docker.cpuCores}
              steps={[1, 2, 4, 8, 12, 16, 24, 32]}
              onChange={(v) => onChange({ case: "docker", docker: { ...spec.docker, cpuCores: v } })}
              format={(v) => `${v}`}
            />
            <SliderField
              label="memory (MB)"
              value={spec.docker.memoryMb}
              steps={[512, 1024, 2048, 4096, 8192, 16384, 32768, 65536]}
              onChange={(v) => onChange({ case: "docker", docker: { ...spec.docker, memoryMb: v } })}
              format={(v) => `${v}`}
            />
          </div>
        </div>
      )}
    </div>
  );
}
