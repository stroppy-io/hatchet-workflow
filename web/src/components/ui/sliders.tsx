import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  closestStep,
  CPU_STEPS,
  DISK_STEPS,
  diskStepsForType,
  IO_M3_CHUNK_GB,
  IO_M3_DISK_STEPS,
  platformLimits,
  ramSteps,
  YANDEX_DISK_SPECS,
  YC_PLATFORMS,
} from "@/lib/machine-constraints";

export {
  closestStep,
  CPU_STEPS,
  DISK_STEPS,
  diskStepsForType,
  IO_M3_CHUNK_GB,
  IO_M3_DISK_STEPS,
  platformLimits,
  ramSteps,
  YC_PLATFORMS,
};

const SLIDER_TRACK = "w-full h-1.5 bg-zinc-800 rounded-full appearance-none cursor-pointer accent-primary disabled:opacity-50 [&::-webkit-slider-thumb]:w-3 [&::-webkit-slider-thumb]:h-3 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-primary [&::-webkit-slider-thumb]:appearance-none";

export function SliderField({ label, value, steps, onChange, disabled, format }: {
  label: string;
  value: number;
  steps: number[];
  onChange: (v: number) => void;
  disabled?: boolean;
  format?: (v: number) => string;
}) {
  const idx = steps.indexOf(closestStep(value, steps));
  return (
    <div className="space-y-1">
      <Label className="text-[9px] font-mono text-zinc-600">{label}</Label>
      <div className="flex items-center gap-2">
        <input type="range" min={0} max={steps.length - 1} value={idx >= 0 ? idx : 0}
          onChange={(e) => onChange(steps[parseInt(e.target.value)])}
          disabled={disabled}
          className={SLIDER_TRACK + " flex-1"} />
        <Input value={format ? format(value) : String(value)}
          onChange={(e) => {
            const n = parseInt(e.target.value.replace(/[^\d]/g, ""));
            if (!isNaN(n) && n >= steps[0]) onChange(closestStep(n, steps));
          }}
          className="h-6 w-16 text-[10px] font-mono text-right tabular-nums shrink-0" disabled={disabled} />
      </div>
    </div>
  );
}

export function NumericSlider({ label, value, min, max, step, onChange, disabled, hint }: {
  label: string;
  value: number;
  min: number;
  max: number;
  step?: number;
  onChange: (v: number) => void;
  disabled?: boolean;
  hint?: string;
}) {
  return (
    <div className="space-y-1">
      <Label className="text-[9px] font-mono text-zinc-600">{label}</Label>
      <div className="flex items-center gap-2">
        <input type="range" min={min} max={max} step={step || 1} value={value}
          onChange={(e) => onChange(parseInt(e.target.value))}
          disabled={disabled}
          className={SLIDER_TRACK + " flex-1"} />
        <Input type="number" min={min} value={value}
          onChange={(e) => {
            const n = parseInt(e.target.value);
            if (!isNaN(n) && n >= min) onChange(n);
          }}
          className="h-6 w-16 text-[10px] font-mono text-right tabular-nums shrink-0" disabled={disabled} />
      </div>
      {hint && <span className="text-[9px] text-zinc-700 font-mono">{hint}</span>}
    </div>
  );
}

// ─── Duration Slider ─────────────────────────────────────────────

const DURATION_STEPS = ["1m", "2m", "5m", "10m", "15m", "30m", "1h", "2h", "4h", "8h", "12h", "24h"];

function parseDuration(s: string): string {
  const trimmed = s.trim().toLowerCase();
  if (/^\d+[smh]$/.test(trimmed)) return trimmed;
  if (/^\d+$/.test(trimmed)) return trimmed + "m";
  return trimmed || "5m";
}

export function DurationSlider({ label, value, onChange, disabled, hint }: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  hint?: string;
}) {
  const normalized = parseDuration(value);
  const idx = DURATION_STEPS.indexOf(normalized);

  return (
    <div className="space-y-1">
      <Label className="text-[9px] font-mono text-zinc-600">{label}</Label>
      <div className="flex items-center gap-2">
        <input type="range" min={0} max={DURATION_STEPS.length - 1} value={idx >= 0 ? idx : 0}
          onChange={(e) => onChange(DURATION_STEPS[parseInt(e.target.value)])}
          disabled={disabled}
          className={SLIDER_TRACK + " flex-1"} />
        <Input value={value}
          onChange={(e) => onChange(e.target.value)}
          onBlur={(e) => onChange(parseDuration(e.target.value))}
          className="h-6 w-16 text-[10px] font-mono text-right tabular-nums shrink-0" disabled={disabled} />
      </div>
      {hint && <span className="text-[9px] text-zinc-700 font-mono">{hint}</span>}
    </div>
  );
}

// ─── Yandex Cloud Platforms ──────────────────────────────────────

export function cpuStepsForPlatform(platformId: string): number[] {
  return CPU_STEPS.filter((s) => s <= platformLimits(platformId).maxCores);
}

export function PlatformSelect({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  return (
    <div>
      <label className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider mb-1 block">Platform</label>
      <div className="flex gap-1.5">
        {Object.entries(YC_PLATFORMS).map(([id, spec]) => {
          const active = value === id;
          return (
            <button
              key={id}
              type="button"
              onClick={() => onChange(id)}
              className={`flex-1 px-2 py-1.5 text-left border transition-colors ${
                active
                  ? "border-primary/40 bg-primary/[0.06]"
                  : "border-zinc-800 hover:border-zinc-700"
              }`}
            >
              <div className={`text-[11px] font-mono font-medium ${active ? "text-primary" : "text-zinc-400"}`}>{spec.label}</div>
              <div className="text-[9px] text-zinc-600">{spec.desc}</div>
              <div className="text-[9px] text-zinc-700">{spec.maxCores} vCPU / {spec.maxRamMb / 1024} GB</div>
            </button>
          );
        })}
      </div>
    </div>
  );
}

export function DiskTypeSelect({ value, onChange }: { value: string; onChange: (v: string) => void; diskSizeGb?: number }) {
  return (
    <div>
      <label className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider mb-1 block">Disk Type</label>
      <div className="flex gap-1.5">
        {Object.entries(YANDEX_DISK_SPECS).map(([id, spec]) => {
          const active = value === id;
          return (
            <button
              key={id}
              type="button"
              onClick={() => onChange(id)}
              className={`flex-1 px-2 py-1.5 text-left border transition-colors ${
                active
                  ? "border-primary/40 bg-primary/[0.06]"
                  : "border-zinc-800 hover:border-zinc-700"
              }`}
            >
              <div className={`text-[11px] font-mono font-medium ${active ? "text-primary" : "text-zinc-400"}`}>{spec.label}</div>
              <div className="text-[9px] text-zinc-600">
                max {spec.maxSizeGb >= 1024 ? `${spec.maxSizeGb / 1024} TB` : `${spec.maxSizeGb} GB`}
              </div>
              <div className="text-[9px] text-zinc-700">
                {spec.multipleGb > 1 ? `${spec.multipleGb} GB chunks` : spec.hint}
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}
