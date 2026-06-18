// Test Preset detail — /presets/test/:id.
//
// Renders the full cloud.v1.models.TestPresetRecord read-only: identity
// (name/description/tags), author + created/updated timings, the system flag,
// and the typed cloud.v1.domain.Test = a DATABASE (domain.Database rendered
// read-only via the shared EngineParamsForm + derived topology) + a WORKLOAD
// (domain.Workload rendered read-only via the shared WorkloadParamsForm).
// Actions map to the TestPresetService RPCs + in-app routes:
//   * Edit         → /presets/test/:id/edit (UpdateTestPreset). System presets
//                    are read-only, operator+ required → gated.
//   * Duplicate    → CloneTestPreset (PresetProvider.clonePreset). Always.
//   * Delete       → DeleteTestPreset (PresetProvider.deletePreset) behind a
//                    confirm. System presets / non-operators → gated.
//   * Use in run   → /runs/new?preset=:id&kind=test (seeds a prefilled,
//                    editable wizard draft). Always.

import { useCallback, useEffect, useState } from "react";
import {
  AlertCircle,
  CheckCircle2,
  Copy,
  Database as DatabaseIcon,
  Gauge,
  Loader2,
  Lock,
  Pencil,
  PlayCircle,
  Trash2,
} from "lucide-react";
import { useNavigate, useParams, useTenantSlug } from "@/lib/router";
import { useBreadcrumbLabel } from "@/lib/breadcrumbs";
import { useAuth } from "@/hooks/useAuth";
import { useAuthorDisplay } from "@/hooks/useAuthorDisplays";
import { roleLevel } from "@/lib/roles";
import { Avatar } from "@/components/Avatar";
import { Button } from "@/components/ui/button";
import { useConfirm } from "@/components/ui/confirm-dialog";
import {
  DB_COLOR,
  DB_LABEL,
  PROTOCOL_LABEL,
  relTime,
  type DbKind,
  type Protocol,
} from "@/components/library-table/labels";
import { ENGINES, Workload_Protocol, type DatabaseVM, type WorkloadVM } from "@/services/wizard";
import {
  EngineParamsForm,
  EngineVersionSelect,
} from "@/components/database/DatabaseParamsForm";
import { WorkloadParamsForm } from "@/components/workload/WorkloadParamsForm";
import { getPresetProvider, type TestPresetDetail as Detail } from "@/services/preset";
import { TopologyPreview } from "@/pages/library/DatabaseTopologyPreview";

/** EngineKind (camel) -> DbKind (proto-cased) for the label/colour catalog. */
const ENGINE_TO_DBKIND: Record<string, Exclude<DbKind, "">> = {
  postgres: "postgres",
  mysql: "mysql",
  mariadb: "mariadb",
  ydb: "ydb",
  ydbManaged: "ydb_managed",
  cockroach: "cockroach",
  picodata: "picodata",
  external: "external",
};

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
  const l = w.segments[0]?.execution.limit;
  if (!l) return "—";
  return l.case === "duration" ? l.duration || "—" : `${l.iterations} iter`;
}

export function TestPresetDetail() {
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
      .getTestPreset(slug, id)
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
  const authorDisplay = useAuthorDisplay(preset?.authorId);

  const onDuplicate = useCallback(async () => {
    if (!slug || !preset) return;
    setBusy(true);
    setError(null);
    try {
      const newId = await getPresetProvider().clonePreset(slug, "test", preset.id, `${preset.name} (copy)`);
      navigate(`/presets/test/${newId}`);
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
      await getPresetProvider().deletePreset(slug, "test", preset.id);
      navigate("/presets/test");
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

  const db = preset.database;
  const w = preset.workload;
  const dbKind = ENGINE_TO_DBKIND[db.kind] ?? "postgres";
  const color = DB_COLOR[dbKind];
  const tagEntries = Object.entries(preset.tags);

  return (
    <div className="flex h-full flex-col">
      {/* Header + actions */}
      <div className="flex items-start justify-between gap-4 border-b border-zinc-800/80 px-5 py-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: color }} />
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
            onClick={() => navigate(`/runs/new?preset=${encodeURIComponent(preset.id)}&kind=test`)}
          >
            <PlayCircle className="h-3.5 w-3.5" /> Use in new run
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={!mutable}
            title={isSystem ? "System presets are read-only" : !canMutate ? "Requires the operator role or higher" : undefined}
            onClick={() => navigate(`/presets/test/${preset.id}/edit`)}
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
        <div className="mx-auto max-w-6xl space-y-6">
          {error && (
            <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
              <AlertCircle className="h-4 w-4 shrink-0" /> {error}
            </div>
          )}

          {/* Meta strip — database + workload summary. */}
          <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <MetaCard label="Engine">
              <span className="font-mono text-sm" style={{ color }}>
                {DB_LABEL[dbKind]}
              </span>
            </MetaCard>
            <MetaCard label="Version">
              <span className="font-mono text-sm text-zinc-300">
                {db.kind === "external" ? "external" : db.version ? `v${db.version}` : "—"}
              </span>
            </MetaCard>
            <MetaCard label="Workload">
              <span className="font-mono text-sm text-zinc-300">
                {w.segments[0]?.script || "—"}
                {w.segments.length > 1 ? ` +${w.segments.length - 1}` : ""}
              </span>
              <span className="font-mono text-[10px] text-zinc-600">
                {protocolLabel(w.protocol)} · {w.segments[0]?.execution.vus ?? 0} VU · {limitLabel(w)}
              </span>
            </MetaCard>
            <MetaCard label="Stroppy version">
              <span className="font-mono text-sm text-zinc-300">
                {w.stroppyVersion ? w.stroppyVersion : "—"}
              </span>
            </MetaCard>
          </section>

          {/* Author / timings */}
          <section className="grid gap-3 sm:grid-cols-2">
            <MetaCard label="Author">
              {authorDisplay ? (
                <div className="flex items-center gap-2">
                  <Avatar name={authorDisplay.avatarName} size={20} />
                  <span className="truncate text-xs text-zinc-300" title={authorDisplay.title}>
                    {authorDisplay.label}
                  </span>
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

          {/* Database (read-only) */}
          <section className="space-y-4 border border-zinc-800/60 bg-[#0a0a0a]/40 p-4">
            <div className="flex items-center gap-2">
              <DatabaseIcon className="h-4 w-4 text-primary/70" />
              <SectionLabel>Database under test</SectionLabel>
            </div>
            <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
              <ReadOnlyDatabase database={db} />
              <aside className="xl:sticky xl:top-0 xl:self-start">
                <SectionLabel>Derived topology</SectionLabel>
                <div className="mt-3">
                  <TopologyPreview database={db} />
                </div>
              </aside>
            </div>
          </section>

          {/* Workload (read-only) */}
          <section className="space-y-4 border border-zinc-800/60 bg-[#0a0a0a]/40 p-4">
            <div className="flex items-center gap-2">
              <Gauge className="h-4 w-4 text-primary/70" />
              <SectionLabel>Workload</SectionLabel>
            </div>
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

/**
 * Render the typed domain.Database read-only by reusing the shared editor with a
 * no-op apply behind a pointer-events guard — so the detail page shows EXACTLY
 * the same fields the editor exposes, with the same labels, in a locked view.
 */
function ReadOnlyDatabase({ database }: { database: DatabaseVM }) {
  const noop = () => {};
  const meta = ENGINES.find((e) => e.kind === database.kind);
  return (
    <div>
      {meta && (
        <p className="mb-3 text-[11px] leading-snug text-zinc-600">{meta.blurb}</p>
      )}
      <div className="pointer-events-none select-none opacity-90" aria-readonly>
        <div className="space-y-4">
          <EngineVersionSelect db={database} apply={noop} />
          <EngineParamsForm db={database} apply={noop} advancedInitiallyOpen />
        </div>
      </div>
    </div>
  );
}
