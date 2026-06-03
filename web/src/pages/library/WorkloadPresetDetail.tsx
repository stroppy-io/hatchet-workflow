// Workload Preset detail — /presets/workload/:id.
//
// Renders the full cloud.v1.models.WorkloadPresetRecord read-only: identity
// (name/description/tags), author + created/updated timings, the system flag,
// and the typed cloud.v1.domain.Workload (script/protocol/version + k6
// execution + data params rendered read-only via the shared WorkloadParamsForm).
// Actions map to the WorkloadPresetService RPCs + in-app routes:
//   * Edit         → /presets/workload/:id/edit (UpdateWorkloadPreset). System
//                    presets are read-only, operator+ required → gated.
//   * Duplicate    → CloneWorkloadPreset (PresetProvider.clonePreset). Always.
//   * Delete       → DeleteWorkloadPreset (PresetProvider.deletePreset) behind a
//                    confirm. System presets / non-operators → gated.
//   * Use in run   → /runs/new?preset=:id&kind=workload. Always.

import { useCallback, useEffect, useState } from "react";
import {
  AlertCircle,
  CheckCircle2,
  Copy,
  Loader2,
  Lock,
  Pencil,
  PlayCircle,
  Trash2,
} from "lucide-react";
import { useNavigate, useParams, useTenantSlug } from "@/lib/router";
import { useBreadcrumbLabel } from "@/lib/breadcrumbs";
import { useAuth } from "@/hooks/useAuth";
import { roleLevel } from "@/lib/roles";
import { Avatar } from "@/components/Avatar";
import { Button } from "@/components/ui/button";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { PROTOCOL_LABEL, relTime, type Protocol } from "@/components/library-table/labels";
import { Workload_Protocol, type WorkloadVM } from "@/services/wizard";
import { WorkloadParamsForm } from "@/components/workload/WorkloadParamsForm";
import { getPresetProvider, type WorkloadPresetDetail as Detail } from "@/services/preset";

/** Workload_Protocol (enum) -> Protocol (proto-cased) for the label catalog. */
const ENUM_TO_PROTOCOL: Record<Workload_Protocol, Protocol> = {
  [Workload_Protocol.UNSPECIFIED]: "",
  [Workload_Protocol.PG]: "pg",
  [Workload_Protocol.MYSQL]: "mysql",
  [Workload_Protocol.PICODATA]: "picodata",
  [Workload_Protocol.YDB_GRPC]: "ydb_grpc",
  [Workload_Protocol.YDB_GRPCS]: "ydb_grpcs",
  [Workload_Protocol.COCKROACH]: "cockroach",
};

function protocolLabel(p: Workload_Protocol): string {
  const proto = ENUM_TO_PROTOCOL[p] ?? "";
  return proto ? PROTOCOL_LABEL[proto] : "default";
}

function limitLabel(w: WorkloadVM): string {
  const l = w.execution.limit;
  return l.case === "duration"
    ? l.duration || "—"
    : `${l.iterations} iter`;
}

export function WorkloadPresetDetail() {
  const slug = useTenantSlug() ?? "";
  const { id } = useParams();
  const navigate = useNavigate();
  const confirm = useConfirm();
  const { user } = useAuth();

  const [preset, setPreset] = useState<Detail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useBreadcrumbLabel("id", preset?.name || id);

  const load = useCallback(() => {
    if (!slug || !id) return;
    let cancelled = false;
    setLoading(true);
    setError(null);
    getPresetProvider()
      .getWorkloadPreset(slug, id)
      .then((p) => !cancelled && setPreset(p))
      .catch((e) => !cancelled && setError(e instanceof Error ? e.message : String(e)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [slug, id]);

  useEffect(() => {
    const cleanup = load();
    return cleanup;
  }, [load]);

  // operator+ on the active tenant (or platform admin) may mutate.
  const tenant = user?.tenants.find((t) => t.slug === slug);
  const level = user?.isAdmin ? 99 : tenant ? roleLevel[tenant.role] : 0;
  const canMutate = level >= roleLevel.operator;
  const isSystem = preset?.isSystem ?? false;
  const mutable = canMutate && !isSystem;

  const onDuplicate = useCallback(async () => {
    if (!slug || !preset) return;
    setBusy(true);
    setError(null);
    try {
      const newId = await getPresetProvider().clonePreset(slug, "workload", preset.id, `${preset.name} (copy)`);
      navigate(`/presets/workload/${newId}`);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setBusy(false);
    }
  }, [slug, preset, navigate]);

  const onDelete = useCallback(async () => {
    if (!slug || !preset) return;
    const ok = await confirm({
      title: "Delete preset?",
      description: `“${preset.name || preset.id}” will be permanently removed. This cannot be undone.`,
      danger: true,
      confirmLabel: "Delete",
    });
    if (!ok) return;
    setBusy(true);
    setError(null);
    try {
      await getPresetProvider().deletePreset(slug, "workload", preset.id);
      navigate("/presets/workload");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setBusy(false);
    }
  }, [slug, preset, confirm, navigate]);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-zinc-500">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Loading preset…
      </div>
    );
  }
  if (error && !preset) {
    return (
      <div className="p-5">
        <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-4 w-4" /> {error}
        </div>
      </div>
    );
  }
  if (!preset) return null;

  const w = preset.workload;
  const tagEntries = Object.entries(preset.tags);

  return (
    <div className="flex h-full flex-col">
      {/* Header + actions */}
      <div className="flex items-start justify-between gap-4 border-b border-zinc-800/80 px-5 py-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="h-2.5 w-2.5 shrink-0 rounded-full bg-primary/70" />
            <h1 className="truncate text-base font-semibold tracking-tight text-foreground">
              {preset.name || preset.id}
            </h1>
            {isSystem ? (
              <span
                className="inline-flex items-center gap-1 border border-zinc-800 bg-zinc-900/50 px-1.5 py-0.5 font-mono text-[10px] text-zinc-400"
                title="Platform-seeded, read-only"
              >
                <Lock className="h-3 w-3" /> System
              </span>
            ) : (
              <span className="inline-flex items-center gap-1 border border-zinc-800 bg-zinc-900/50 px-1.5 py-0.5 font-mono text-[10px] text-zinc-500">
                <CheckCircle2 className="h-3 w-3 text-success/70" /> User
              </span>
            )}
          </div>
          {preset.description && (
            <p className="mt-1 max-w-2xl text-sm text-zinc-500">{preset.description}</p>
          )}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Button
            variant="default"
            size="sm"
            onClick={() => navigate(`/runs/new?preset=${encodeURIComponent(preset.id)}&kind=workload`)}
          >
            <PlayCircle className="h-3.5 w-3.5" /> Use in new run
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={!mutable}
            title={isSystem ? "System presets are read-only" : !canMutate ? "Requires the operator role or higher" : undefined}
            onClick={() => navigate(`/presets/workload/${preset.id}/edit`)}
          >
            <Pencil className="h-3.5 w-3.5" /> Edit
          </Button>
          <Button variant="outline" size="sm" disabled={busy} onClick={() => void onDuplicate()}>
            <Copy className="h-3.5 w-3.5" /> Duplicate
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={!mutable || busy}
            title={isSystem ? "System presets cannot be deleted" : !canMutate ? "Requires the operator role or higher" : undefined}
            className="text-red-400 hover:text-red-300"
            onClick={() => void onDelete()}
          >
            <Trash2 className="h-3.5 w-3.5" /> Delete
          </Button>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5">
        <div className="mx-auto max-w-3xl space-y-6">
          {error && (
            <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
              <AlertCircle className="h-4 w-4 shrink-0" /> {error}
            </div>
          )}

          {/* Meta strip */}
          <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <MetaCard label="Script">
              <span className="font-mono text-sm text-zinc-300">{w.script || "—"}</span>
            </MetaCard>
            <MetaCard label="Protocol">
              <span className="font-mono text-sm text-zinc-300">{protocolLabel(w.protocol)}</span>
            </MetaCard>
            <MetaCard label="Stroppy version">
              <span className="font-mono text-sm text-zinc-300">
                {w.stroppyVersion ? w.stroppyVersion : "—"}
              </span>
            </MetaCard>
            <MetaCard label="Execution">
              <span className="font-mono text-sm text-zinc-300">
                {w.execution.vus} VU · {limitLabel(w)}
              </span>
            </MetaCard>
          </section>

          {/* Author / timings */}
          <section className="grid gap-3 sm:grid-cols-2">
            <MetaCard label="Author">
              {preset.authorId ? (
                <div className="flex items-center gap-2">
                  <Avatar name={preset.authorId} size={20} />
                  <span className="truncate font-mono text-xs text-zinc-300">{preset.authorId}</span>
                </div>
              ) : (
                <span className="font-mono text-xs text-zinc-600">—</span>
              )}
            </MetaCard>
            <MetaCard label="Updated">
              <span className="font-mono text-xs text-zinc-300" title={preset.updatedAt}>
                {relTime(preset.updatedAt)}
              </span>
              <span className="font-mono text-[10px] text-zinc-600" title={preset.createdAt}>
                created {relTime(preset.createdAt)}
              </span>
            </MetaCard>
          </section>

          {/* Tags */}
          <section>
            <SectionLabel>Tags</SectionLabel>
            <div className="mt-2 flex flex-wrap gap-1.5">
              {tagEntries.length === 0 ? (
                <span className="font-mono text-xs text-zinc-600">No tags</span>
              ) : (
                tagEntries.map(([k, v]) => (
                  <span
                    key={k}
                    className="inline-flex items-center gap-1 border border-zinc-800 bg-[#0a0a0a] px-2 py-0.5 font-mono text-[11px] text-zinc-300"
                  >
                    <span className="text-zinc-500">{k}</span>
                    {v && <span className="text-zinc-600">=</span>}
                    {v && <span>{v}</span>}
                  </span>
                ))
              )}
            </div>
          </section>

          {/* Typed configuration (read-only) */}
          <section className="space-y-4">
            <SectionLabel>Configuration</SectionLabel>
            <WorkloadParamsForm w={w} apply={() => {}} disabled />
          </section>
        </div>
      </div>
    </div>
  );
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">{children}</div>
  );
}

function MetaCard({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1 border border-zinc-800/60 bg-[#0a0a0a] p-3">
      <span className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">{label}</span>
      {children}
    </div>
  );
}
