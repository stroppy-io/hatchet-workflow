import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

const SLIDER_TRACK = "w-full h-1.5 bg-zinc-800 rounded-full appearance-none cursor-pointer accent-primary disabled:opacity-50 [&::-webkit-slider-thumb]:w-3 [&::-webkit-slider-thumb]:h-3 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-primary [&::-webkit-slider-thumb]:appearance-none";

export function closestStep(val: number, steps: number[]): number {
  let best = steps[0];
  for (const s of steps) {
    if (Math.abs(s - val) < Math.abs(best - val)) best = s;
  }
  return best;
}

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

export function DurationSlider({ label, value, onChange, disabled }: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
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
    </div>
  );
}

export const CPU_STEPS = [2, 4, 8, 12, 16, 24, 32];
export const DISK_STEPS = [25, 50, 100, 200, 300, 500, 750, 1024];

export function ramSteps(cpus: number): number[] {
  const min = cpus * 1024;
  const steps: number[] = [];
  let v = min;
  while (v <= 262144) {
    steps.push(v);
    if (v < 8192) v += 1024;
    else if (v < 32768) v += 4096;
    else if (v < 65536) v += 8192;
    else v += 32768;
  }
  return steps;
}
