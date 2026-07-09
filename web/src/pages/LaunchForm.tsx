// LaunchForm — /recipes/:id/launch. Fetches the composed schemapb form
// schema (RecipeService.LaunchFormSchema) and renders it via
// LaunchFormRenderer; on submit it launches the stored recipe bundle
// (RecipeService.StartRun) with the form's baked payload and navigates to
// the new run, exactly like RecipeEditor's old onRun did for the no-inputs
// path.
//
// Rerun (finding I2): RunDetail's Rerun action navigates here with
// `?from=<runId>` instead of calling StartRun directly with no Filled — that
// old behavior silently discarded whatever the user originally typed into
// the launch form and launched with the bundle's static defaults instead.
// When `from` is present, this page fetches the ORIGINATING run's overview
// (services/run_overview.getRunOverview — the same tenant-scoped read path
// RunDetail itself already uses; no new RPC needed, since cloud.v1.models.Run
// already persists Run.baked and TestRunOverviewSnapshot already embeds the
// full Run) and, if that run carries a stored Baked (schemapb.Baked; nil for
// pre-SP-D/no-form runs), seeds LaunchFormRenderer's initialValues from
// Baked.values — the same nested (top-level workflow inputs + nested
// "provider" object) shape ComposeFormSchema/BakeForm already produce, so no
// client-side splitting is needed. The form always renders against the
// CURRENT recipe's schema (fetchLaunchFormSchema(slug, id) below, never a
// stored one), so a value that no longer matches the current schema (recipe
// drift since the original run) is not silently dropped — LaunchFormRenderer
// re-validates on mount and surfaces the mismatch as an ordinary field error
// on this same form (see its own initialValues doc). The user reviews (and
// may edit) every value before pressing Launch; StartRun re-Bakes and
// validates server-side exactly as it does for any other launch — this is
// display-only prefill, never a client-trusted Baked bypass.
// A `from` run with no stored Baked (or missing/cross-tenant/failed to load)
// falls back to today's behavior: an empty form seeded from the bundle's own
// schema defaults.
//
// Two distinct error surfaces:
//   - onInvalid (LaunchFormRenderer's own prop): the LOCAL WASM Bake
//     rejected the submission before it ever reached the network — never
//     even calls onSubmit.
//   - a StartRunFieldError thrown by startRun: the SERVER re-Baked the
//     (already locally-sealed) payload and rejected it — schemapb is the
//     single source of truth on both sides, but only the server-side Bake
//     is actually trusted (StartRun's own doc comment).
// Both render through the same fieldErrors state, keyed by field path
// (errorsByField), so the form shows one consistent per-field error UI
// regardless of which side caught the problem.
import { useEffect, useState, useCallback } from "react";
import { AlertCircle, ArrowLeft, Loader2 } from "lucide-react";
import { errorsByField } from "@stroppy-io/schemapb-react";
import type { BakedJson, FieldErrorJson, Schema } from "@stroppy-io/schemapb";
import { useNavigate, useParams, useSearchParams, useTenantSlug } from "@/lib/router";
import { LaunchFormRenderer } from "@/components/launch-form/LaunchFormRenderer";
import { fetchLaunchFormSchema, startRun, StartRunFieldError } from "@/services/recipe";
import { getRunOverview } from "@/services/run_overview";

export function LaunchForm() {
  const { id } = useParams<{ id: string }>();
  const slug = useTenantSlug();
  const navigate = useNavigate();
  const [sp] = useSearchParams();
  const fromRunId = sp.get("from") ?? "";

  const [schema, setSchema] = useState<Schema | null>(null);
  const [initialValues, setInitialValues] = useState<Record<string, unknown> | undefined>(undefined);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [launching, setLaunching] = useState(false);
  const [launchError, setLaunchError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, FieldErrorJson>>({});

  useEffect(() => {
    if (!slug || !id) return;
    setLoading(true);
    setLoadError(null);
    Promise.all([
      fetchLaunchFormSchema(slug, id),
      // A from-run lookup failure (deleted/cross-tenant/older no-Baked run)
      // must NOT block the launch form itself — it only means no prefill,
      // exactly the "Baked == nil" fallback. getRunOverview is tenant-scoped
      // (GetTestRunOverview.authorizeRun -> Runs.Exists(tenantID, runID)),
      // so a run id from another tenant resolves as not-found here too.
      fromRunId ? getRunOverview(slug, fromRunId).catch(() => null) : Promise.resolve(null),
    ])
      .then(([sc, origin]) => {
        setSchema(sc);
        const values = origin?.run?.baked?.values as Record<string, unknown> | undefined;
        setInitialValues(values);
      })
      .catch((e) => setLoadError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false));
  }, [slug, id, fromRunId]);

  const onSubmit = useCallback(
    (baked: BakedJson) => {
      if (!slug || !id) return;
      setLaunching(true);
      setLaunchError(null);
      setFieldErrors({});
      startRun(slug, id, baked)
        .then((runId) => navigate(`/runs/${runId}`))
        .catch((e) => {
          if (e instanceof StartRunFieldError) {
            setFieldErrors(errorsByField(e.fieldErrors));
          } else {
            setLaunchError(e instanceof Error ? e.message : String(e));
          }
          setLaunching(false);
        });
    },
    [slug, id, navigate],
  );

  const onInvalid = useCallback((errors: Record<string, FieldErrorJson>) => {
    setFieldErrors(errors);
  }, []);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-zinc-500">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Loading launch form…
      </div>
    );
  }
  if (loadError || !schema) {
    return (
      <div className="p-5">
        <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-4 w-4" /> {loadError ?? "no form schema"}
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-3 border-b border-zinc-800/80 px-5 py-3">
        <button
          type="button"
          onClick={() => navigate(`/recipes/${id}`)}
          className="flex h-7 w-7 items-center justify-center border border-zinc-800 text-zinc-500 hover:border-zinc-700 hover:text-zinc-300"
          aria-label="Back"
        >
          <ArrowLeft className="h-4 w-4" />
        </button>
        <div className="text-sm font-semibold">Launch</div>
      </div>
      <div className="max-w-xl p-5">
        {launchError && (
          <div className="mb-4 flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
            <AlertCircle className="h-4 w-4" /> {launchError}
          </div>
        )}
        {Object.keys(fieldErrors).length > 0 && (
          <div
            role="alert"
            className="mb-4 flex flex-col gap-1 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400"
          >
            {Object.entries(fieldErrors).map(([field, e]) => (
              <div key={field}>
                <span className="font-semibold">{field}</span>: {e.message}
              </div>
            ))}
          </div>
        )}
        <LaunchFormRenderer
          schema={schema}
          onSubmit={onSubmit}
          onInvalid={onInvalid}
          submitLabel={launching ? "Launching…" : "Launch"}
          initialValues={initialValues}
        />
      </div>
    </div>
  );
}

export default LaunchForm;
