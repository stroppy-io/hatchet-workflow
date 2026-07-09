// LaunchForm — /recipes/:id/launch. Fetches the composed schemapb form
// schema (RecipeService.LaunchFormSchema) and renders it via
// LaunchFormRenderer; on submit it launches the stored recipe bundle
// (RecipeService.StartRun) with the form's baked payload and navigates to
// the new run, exactly like RecipeEditor's old onRun did for the no-inputs
// path.
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
import { useNavigate, useParams, useTenantSlug } from "@/lib/router";
import { LaunchFormRenderer } from "@/components/launch-form/LaunchFormRenderer";
import { fetchLaunchFormSchema, startRun, StartRunFieldError } from "@/services/recipe";

export function LaunchForm() {
  const { id } = useParams<{ id: string }>();
  const slug = useTenantSlug();
  const navigate = useNavigate();

  const [schema, setSchema] = useState<Schema | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [launching, setLaunching] = useState(false);
  const [launchError, setLaunchError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, FieldErrorJson>>({});

  useEffect(() => {
    if (!slug || !id) return;
    setLoading(true);
    setLoadError(null);
    fetchLaunchFormSchema(slug, id)
      .then(setSchema)
      .catch((e) => setLoadError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false));
  }, [slug, id]);

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
        />
      </div>
    </div>
  );
}

export default LaunchForm;
