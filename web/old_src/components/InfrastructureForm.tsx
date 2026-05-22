import type { Provider } from "@/api/types";
import { Cloud, Container } from "lucide-react";
import { PlatformSelect } from "@/components/ui/sliders";

// Shared provider metadata (icon + label). Defined here so any consumer of
// InfrastructureForm — NewRun's wizard, the suite builder — uses the same
// presentation without redefining it.
export const PROVIDER_META: Record<Provider, { icon: typeof Cloud; label: string }> = {
  docker: { icon: Container, label: "Docker" },
  yandex: { icon: Cloud, label: "Yandex Cloud" },
};

export const ALL_PROVIDERS: Provider[] = ["docker", "yandex"];

// InfrastructureForm renders the "where to run" picker — provider tiles
// plus the conditional Yandex platform select. Extracted from NewRun's
// StepInfra so the suite builder reuses the same surface (Identity is
// optional and rendered only when name/desc setters are passed).
//
// All state is owned by the caller (controlled component).
export interface InfrastructureFormProps {
  provider: Provider;
  setProvider: (p: Provider) => void;
  platformId: string;
  setPlatformId: (p: string) => void;
  providers?: Provider[];

  // Optional identity block. When name/desc setters are provided the
  // top-of-form Identity section renders; omit them to skip it.
  name?: string;
  setName?: (v: string) => void;
  description?: string;
  setDescription?: (v: string) => void;
  identityLabel?: string;
  namePlaceholder?: string;
  descriptionPlaceholder?: string;
}

export function InfrastructureForm({
  provider,
  setProvider,
  platformId,
  setPlatformId,
  providers = ALL_PROVIDERS,
  name,
  setName,
  description,
  setDescription,
  identityLabel = "Test Identity",
  namePlaceholder = "Test name (e.g. baseline-tpcc-pg17-100vu)",
  descriptionPlaceholder = "Description — what changed vs the baseline, why this run exists, what to compare against",
}: InfrastructureFormProps) {
  const showIdentity = setName !== undefined || setDescription !== undefined;
  return (
    <div className="space-y-5 max-w-lg">
      {showIdentity && (
        <div className="border border-zinc-800/60 bg-[#070707] px-3 py-2.5 space-y-2">
          <div className="flex items-baseline justify-between">
            <h2 className="text-[11px] font-mono uppercase tracking-wider text-zinc-500">
              {identityLabel}
            </h2>
            <span className="text-[10px] font-mono text-zinc-700">optional</span>
          </div>
          {setName && (
            <input
              type="text"
              value={name ?? ""}
              onChange={(e) => setName(e.target.value)}
              placeholder={namePlaceholder}
              className="w-full bg-transparent border-0 border-b border-zinc-800 px-0 py-1.5 text-sm font-mono text-zinc-200 placeholder:text-zinc-700 focus:outline-none focus:border-primary/60 transition-colors"
              maxLength={128}
            />
          )}
          {setDescription && (
            <textarea
              value={description ?? ""}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={descriptionPlaceholder}
              className="w-full bg-transparent border-0 border-b border-zinc-800 px-0 py-1.5 text-xs font-mono text-zinc-300 placeholder:text-zinc-700 focus:outline-none focus:border-primary/60 transition-colors resize-none"
              rows={2}
              maxLength={1024}
            />
          )}
        </div>
      )}

      <div>
        <h2 className="text-sm font-semibold mb-1">Where to run?</h2>
        <p className="text-xs text-zinc-500">Choose the infrastructure provider for provisioning machines.</p>
      </div>
      <div className="grid grid-cols-2 gap-3">
        {providers.map((p) => {
          const pm = PROVIDER_META[p];
          const PIcon = pm.icon;
          const active = provider === p;
          return (
            <button type="button" key={p} onClick={() => setProvider(p)}
              className={`flex items-center gap-3 border p-4 transition-all cursor-pointer ${
                active
                  ? "border-primary/40 text-primary bg-primary/[0.06]"
                  : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
              }`}
            >
              <PIcon className={`h-5 w-5 shrink-0 ${active ? "text-primary" : "text-zinc-600"}`} />
              <div className="text-left">
                <div className={`text-xs font-mono font-medium ${active ? "text-primary" : "text-zinc-400"}`}>{pm.label}</div>
                <div className="text-[10px] text-zinc-600">
                  {p === "docker" ? "Local containers" : "Yandex Cloud VMs"}
                </div>
              </div>
            </button>
          );
        })}
      </div>
      {provider === "yandex" && (
        <PlatformSelect value={platformId} onChange={setPlatformId} />
      )}
    </div>
  );
}

export default InfrastructureForm;
