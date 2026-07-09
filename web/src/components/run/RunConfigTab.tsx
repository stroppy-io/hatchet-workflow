// Run CONFIG (Overview) tab — the run's resolved launch parameters + compiled
// plan, plus a read-only view of the originating recipe bundle.
//
// Recipe runs never carried a `domain.TestRun` spec (they're launched from a
// DSL bundle, not a baked classic-TestWorkflow spec) — the classic
// workload-segments/database-spec render this tab used to have was ALWAYS
// empty for a recipe run (see cloud.v1.models.Run's doc / SP-E spec §5,§6.C).
// SP-E fixes that hole: `run.baked` (the sealed launch-form snapshot, nil
// until SP-D's generated-form path exists) and `run.compiledPlan` (the
// compiled DSL plan — machine groups / services / jobs RunRecipeWorkflow
// actually executed) are rendered here as the primary "resolved parameters"
// view. `RecipeBundleView` (the underlying recipe source files) is kept as a
// secondary/raw section — Baked/CompiledPlan don't replace it, they show
// resolved VALUES, not the DSL source that produced them.
import { useEffect, useState } from "react";
import { ChevronRight, Code2, ExternalLink, FileCode, Layers, Settings2 } from "lucide-react";
import type { RunVM } from "@/services/runs";
import { getRecipe, type RecipeVM } from "@/services/recipe";
import { DslEditor } from "@/components/ui/dsl-editor";
import { Link, useTenantSlug } from "@/lib/router";
import { cn } from "@/lib/utils";

function humanizeKey(k: string): string {
  return k
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2") // camelCase
    .replace(/[_-]+/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase())
    .trim();
}

function isPlainObject(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function isEmptyValue(v: unknown): boolean {
  if (v === null || v === undefined || v === "") return true;
  if (Array.isArray(v)) return v.length === 0;
  if (isPlainObject(v)) return Object.keys(v).length === 0;
  return false;
}

function Prim({ value }: { value: unknown }) {
  return <span className="break-all font-mono text-[11px] text-foreground">{String(value)}</span>;
}

// Compact label→value row (mono, dense terminal aesthetic).
function KV({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-2 py-0.5 text-[11px]">
      <span className="w-40 shrink-0 truncate font-mono text-muted-foreground" title={label}>
        {label}
      </span>
      <span className="min-w-0 flex-1">{children}</span>
    </div>
  );
}

// Generic recursive renderer for an arbitrary decoded-proto value. Skips empty
// values so a spec with unset optionals stays clean.
function ConfigValue({ value }: { value: unknown }): React.ReactElement | null {
  if (isEmptyValue(value)) return null;

  if (Array.isArray(value)) {
    // Array of primitives → inline; array of objects → stacked cards.
    if (value.every((v) => v === null || typeof v !== "object")) {
      return <Prim value={value.join(", ")} />;
    }
    return (
      <div className="flex flex-col gap-2">
        {value.map((v, i) => (
          <div key={i} className="rounded border border-border/60 p-2">
            <ConfigValue value={v} />
          </div>
        ))}
      </div>
    );
  }

  if (isPlainObject(value)) {
    const entries = Object.entries(value).filter(([, v]) => !isEmptyValue(v));
    if (entries.length === 0) return null;
    return (
      <div className="flex flex-col">
        {entries.map(([k, v]) => (
          <KV key={k} label={humanizeKey(k)}>
            {isPlainObject(v) || Array.isArray(v) ? <ConfigValue value={v} /> : <Prim value={v} />}
          </KV>
        ))}
      </div>
    );
  }

  return <Prim value={value} />;
}

function Card({ icon, title, children }: { icon: React.ReactNode; title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-border bg-card/40 p-4">
      <div className="mb-3 flex items-center gap-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
        {icon}
        {title}
      </div>
      {children}
    </div>
  );
}

// Recipe runs persist no classic spec by design (they're launched from a DSL bundle,
// not a baked domain.TestRun). Instead of the generic "No spec available"
// empty-state, fetch the recipe that produced the run and show its bundle
// read-only, so the run stays auditable — you can see exactly which
// cluster.yaml/workflow.yaml/components produced it, and jump to the recipe.
type RecipeBundleState =
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "ready"; recipe: RecipeVM };

function RecipeBundleView({ tenantSlug, recipeId }: { tenantSlug: string; recipeId: string }) {
  const [state, setState] = useState<RecipeBundleState>({ status: "loading" });
  const [activePath, setActivePath] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setState({ status: "loading" });
    getRecipe(tenantSlug, recipeId)
      .then((recipe) => {
        if (cancelled) return;
        setState({ status: "ready", recipe });
        const paths = Object.keys(recipe.files).sort();
        setActivePath(paths.includes("cluster.yaml") ? "cluster.yaml" : (paths[0] ?? null));
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setState({ status: "error", message: err instanceof Error ? err.message : String(err) });
      });
    return () => {
      cancelled = true;
    };
  }, [tenantSlug, recipeId]);

  if (state.status === "loading") {
    return <div className="p-6 text-sm text-muted-foreground">Loading recipe bundle…</div>;
  }

  if (state.status === "error") {
    return (
      <div className="p-6 text-sm text-muted-foreground">
        Recipe no longer available — this run was launched from recipe{" "}
        <span className="font-mono text-foreground">{recipeId}</span>.
      </div>
    );
  }

  const { recipe } = state;
  const paths = Object.keys(recipe.files).sort();

  return (
    <div className="flex flex-col gap-3 p-1">
      <div className="flex items-center justify-between rounded-lg border border-border bg-card/40 p-4">
        <div>
          <div className="text-sm font-semibold text-foreground">{recipe.name}</div>
          <div className="text-[11px] text-muted-foreground">
            v{recipe.version} · {recipe.provider || "—"}
          </div>
        </div>
        <Link
          to={`/recipes/${recipe.id}`}
          className="flex items-center gap-1 text-[11px] font-medium text-primary hover:underline"
        >
          <ExternalLink className="h-3.5 w-3.5" />
          Open recipe
        </Link>
      </div>

      {paths.length === 0 ? (
        <div className="rounded-lg border border-border bg-card/40 p-3 text-[11px] text-muted-foreground">
          Recipe bundle has no files.
        </div>
      ) : (
        <div className="rounded-lg border border-border bg-card/40">
          <div className="flex flex-wrap gap-1 border-b border-border p-2">
            {paths.map((p) => (
              <button
                key={p}
                type="button"
                onClick={() => setActivePath(p)}
                className={cn(
                  "rounded px-2 py-1 font-mono text-[11px]",
                  p === activePath ? "bg-primary/20 text-foreground" : "text-muted-foreground hover:text-foreground",
                )}
              >
                {p}
              </button>
            ))}
          </div>
          {activePath && (
            <div className="h-[28rem]">
              <DslEditor
                path={activePath}
                value={recipe.files[activePath] ?? ""}
                onChange={() => {}}
                files={recipe.files}
                readOnly
              />
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// Compiled-plan machine-groups/services/jobs, rendered as compact cards — the
// generic ConfigValue renderer handles arbitrary nested shapes so new
// dsl.CompiledPlan fields show up without a UI change.
function PlanSection({ title, items }: { title: string; items: Record<string, unknown>[] }) {
  if (items.length === 0) return null;
  return (
    <div>
      <div className="mb-1.5 flex items-center gap-1.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
        <Settings2 className="h-3 w-3" />
        {title} ({items.length})
      </div>
      <div className="flex flex-col gap-2">
        {items.map((item, i) => (
          <div key={i} className="rounded border border-border/60 p-2">
            <ConfigValue value={item} />
          </div>
        ))}
      </div>
    </div>
  );
}

export function RunConfigTab({ run }: { run: RunVM }) {
  const [rawOpen, setRawOpen] = useState(false);
  const tenantSlug = useTenantSlug() ?? "";

  const baked = run.baked;
  const bakedValues = isPlainObject(baked?.values) ? (baked.values as Record<string, unknown>) : undefined;
  const plan = run.compiledPlan;
  const hasBaked = !!bakedValues && Object.keys(bakedValues).length > 0;
  const hasPlan =
    !!plan &&
    ((plan.machineGroups?.length ?? 0) > 0 || (plan.services?.length ?? 0) > 0 || (plan.jobs?.length ?? 0) > 0);

  if (!hasBaked && !hasPlan && !run.workflowId) {
    return (
      <div className="p-6 text-sm text-muted-foreground">
        No resolved parameters or compiled plan available for this run.
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4 p-1">
      {(hasBaked || hasPlan) && (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          {/* Resolved parameters — the sealed launch-form snapshot (schemapb.Baked). */}
          <Card icon={<FileCode className="h-3.5 w-3.5" />} title="Resolved parameters">
            {hasBaked ? (
              <ConfigValue value={bakedValues} />
            ) : (
              <span className="text-[11px] text-muted-foreground">
                No baked launch-form values for this run (generated-form launch path not in use yet).
              </span>
            )}
          </Card>

          {/* Compiled plan — machine groups / services / jobs RunRecipeWorkflow executed. */}
          <Card icon={<Layers className="h-3.5 w-3.5" />} title="Compiled plan">
            {hasPlan ? (
              <div className="flex flex-col gap-3">
                {plan?.provider?.name && <KV label="Provider"><Prim value={plan.provider.name} /></KV>}
                <PlanSection title="Machine groups" items={(plan?.machineGroups ?? []) as unknown as Record<string, unknown>[]} />
                <PlanSection title="Services" items={(plan?.services ?? []) as unknown as Record<string, unknown>[]} />
                <PlanSection title="Jobs" items={(plan?.jobs ?? []) as unknown as Record<string, unknown>[]} />
              </div>
            ) : (
              <span className="text-[11px] text-muted-foreground">No compiled plan persisted for this run.</span>
            )}
          </Card>
        </div>
      )}

      {/* Recipe bundle — the DSL source that produced baked/compiledPlan above. */}
      {run.workflowId && <RecipeBundleView tenantSlug={tenantSlug} recipeId={run.workflowId} />}

      {(hasBaked || hasPlan) && (
        <div className="rounded-lg border border-border bg-card/40">
          <button
            type="button"
            onClick={() => setRawOpen((v) => !v)}
            className="flex w-full items-center gap-2 p-3 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground hover:text-foreground"
          >
            <ChevronRight className={cn("h-3.5 w-3.5 transition-transform", rawOpen && "rotate-90")} />
            <Code2 className="h-3.5 w-3.5" />
            Full baked + compiled plan (raw)
          </button>
          {rawOpen && (
            <pre className="max-h-[32rem] overflow-auto border-t border-border bg-black/30 p-3 font-mono text-[11px] leading-relaxed text-foreground">
              {JSON.stringify({ baked, compiledPlan: plan }, null, 2)}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}
