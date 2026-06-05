export const CPU_STEPS = [2, 4, 8, 12, 16, 24, 32, 48, 64, 80, 96, 128, 192, 256, 288];
export const DISK_STEPS = [25, 50, 100, 200, 300, 500, 750, 1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072, 262144];

export const IO_M3_CHUNK_GB = 93;

export type YandexNetworkAcceleration = "standard" | "software_accelerated";

export interface PlatformLimits {
  label: string;
  desc: string;
  maxCores: number;
  maxRamMb: number;
}

export const YC_PLATFORMS: Record<string, PlatformLimits> = {
  "standard-v2": { label: "Standard v2", desc: "Intel Cascade Lake", maxCores: 80, maxRamMb: 1280 * 1024 },
  "standard-v3": { label: "Standard v3", desc: "Intel Ice Lake", maxCores: 96, maxRamMb: 640 * 1024 },
  "highfreq-v3": { label: "High-freq v3", desc: "Intel Ice Lake HF", maxCores: 56, maxRamMb: 448 * 1024 },
};

export const YANDEX_NETWORK_ACCELERATIONS: { value: YandexNetworkAcceleration; label: string; hint: string }[] = [
  { value: "standard", label: "Standard", hint: "regular VM network" },
  { value: "software_accelerated", label: "Software accelerated", hint: "extra service cores for network traffic" },
];

export const YANDEX_DISK_SPECS: Record<string, {
  label: string;
  maxSizeGb: number;
  multipleGb: number;
  hint: string;
}> = {
  "network-hdd": {
    label: "HDD",
    maxSizeGb: 262144,
    multipleGb: 1,
    hint: "replicated network HDD",
  },
  "network-ssd": {
    label: "SSD",
    maxSizeGb: 262144,
    multipleGb: 1,
    hint: "replicated network SSD",
  },
  "network-ssd-nonreplicated": {
    label: "SSD non-replicated",
    maxSizeGb: 262144,
    multipleGb: IO_M3_CHUNK_GB,
    hint: "93 GB chunks, no replication",
  },
  "network-ssd-io-m3": {
    label: "SSD io-m3",
    maxSizeGb: 262144,
    multipleGb: IO_M3_CHUNK_GB,
    hint: "93 GB chunks, replicated high I/O",
  },
};

export function closestStep(val: number, steps: number[]): number {
  let best = steps[0] ?? 0;
  for (const s of steps) {
    if (Math.abs(s - val) < Math.abs(best - val)) best = s;
  }
  return best;
}

function buildChunkedDiskSteps(maxGb = 65536): number[] {
  const steps: number[] = [];
  for (let i = 1; i <= 10; i++) steps.push(i * IO_M3_CHUNK_GB);
  const big = [12, 16, 24, 32, 48, 64, 96, 128, 192, 256, 384, 512, 704, 1024, 1536, 2048, 2818];
  for (const c of big) steps.push(c * IO_M3_CHUNK_GB);
  return steps.filter((s) => s <= maxGb);
}

export const IO_M3_DISK_STEPS = buildChunkedDiskSteps();

export function normalizeYandexBootDiskType(diskType: string | undefined): string {
  return diskType && YANDEX_DISK_SPECS[diskType] ? diskType : "network-ssd";
}

export function diskStepsForType(diskType: string): number[] {
  const type = normalizeYandexBootDiskType(diskType);
  const spec = YANDEX_DISK_SPECS[type];
  if (spec.multipleGb === IO_M3_CHUNK_GB) return buildChunkedDiskSteps(spec.maxSizeGb);
  return DISK_STEPS.filter((s) => s <= spec.maxSizeGb);
}

export function normalizeYandexDiskGb(diskType: string | undefined, diskGb: number | undefined): number {
  const type = normalizeYandexBootDiskType(diskType);
  const steps = diskStepsForType(type);
  const safe = Number.isFinite(diskGb) && (diskGb ?? 0) > 0 ? Number(diskGb) : steps[0];
  return closestStep(safe, steps);
}

export function normalizeYandexInternalIp(value: string | undefined): string {
  return value?.trim() || "auto";
}

export function normalizeYandexNetworkAcceleration(value: string | undefined): YandexNetworkAcceleration {
  return value === "software_accelerated" ? "software_accelerated" : "standard";
}

export function platformLimits(platformId: string): PlatformLimits {
  return YC_PLATFORMS[platformId] ?? YC_PLATFORMS["standard-v3"];
}

export function cpuStepsForPlatform(platformId: string): number[] {
  const { maxCores } = platformLimits(platformId);
  return CPU_STEPS.filter((s) => s <= maxCores);
}

export function ramSteps(cpus: number, maxRamMb?: number): number[] {
  const min = Math.max(1, cpus) * 1024;
  const cap = maxRamMb ?? 262144;
  const steps: number[] = [];
  let v = min;
  while (v <= cap) {
    steps.push(v);
    if (v < 8192) v += 1024;
    else if (v < 32768) v += 4096;
    else if (v < 65536) v += 8192;
    else v += 32768;
  }
  return steps.length > 0 ? steps : [min];
}

export function ramGbSteps(cpus: number, maxRamMb?: number): number[] {
  return ramSteps(cpus, maxRamMb).map((mb) => mb / 1024);
}
