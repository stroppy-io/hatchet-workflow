// Test Preset authoring — CREATE (/presets/test/new) and EDIT
// (/presets/test/:id/edit).
//
// Built against cloud.v1.api.TestPresetService: the form edits one
// cloud.v1.models.TestPresetRecord = Entity (name/description/tags) + the typed
// cloud.v1.domain.Test = a DATABASE (cloud.v1.domain.Database, engine Kind +
// per-engine Params) + a WORKLOAD (cloud.v1.domain.Workload, script/protocol/k6
// Execution/data Parameters). It REUSES BOTH shared editors — the database
// editor (EngineParamsForm + EngineVersionSelect) and the workload editor
// (WorkloadParamsForm) — and the SAME domain validators (validateDatabase +
// validateWorkload) so the surface is identical to the Database / Workload
// preset forms a Test composes.
//
//   Create → PresetProvider.createTestPreset(slug, input)  → CreateTestPreset
//   Edit   → PresetProvider.getTestPreset(slug, id) to load,
//            then PresetProvider.updateTestPreset(slug, id, input) → UpdateTestPreset
//
// System presets are platform-seeded and read-only: editing one is gated out.
// On save the page navigates to the preset's detail page (/presets/test/:id).

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AlertCircle,
  ArrowLeft,
  Check,
  Database as DatabaseIcon,
  Gauge,
  Loader2,
  Lock,
  Plus,
  Save,
  X,
} from "lucide-react";
import { useNavigate, useParams, useTenantSlug } from "@/lib/router";
import { useBreadcrumbLabel } from "@/lib/breadcrumbs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  ENGINES,
  defaultEngineParams,
  defaultWorkload,
  type DatabaseVM,
  type EngineKind,
  type WorkloadVM,
} from "@/services/wizard";
import { getPresetProvider, type TestPresetInput } from "@/services/preset";
import {
  EngineParamsForm,
  EngineVersionSelect,
  FieldErrors,
  errorsFor,
  validateDatabase,
  DB_VERSIONS,
} from "@/components/database/DatabaseParamsForm";
import {
  WorkloadParamsForm,
  validateWorkload,
} from "@/components/workload/WorkloadParamsForm";
import { TopologyPreview } from "@/pages/library/DatabaseTopologyPreview";

/** A fresh, immediately-valid DatabaseVM for a chosen engine. */
function freshDatabase(engine: EngineKind): DatabaseVM {
  return {
    kind: engine,
    version: DB_VERSIONS[engine][0] ?? "",
    params: defaultEngineParams(engine),
  };
}

/** A fresh, immediately-valid WorkloadVM (TPC-C defaults, no version). */
function freshWorkload(): WorkloadVM {
  return { ...defaultWorkload("postgres"), stroppyVersion: "" };
}

export function TestPresetForm() {
  const slug = useTenantSlug() ?? "";
  const { id } = useParams();
  const navigate = useNavigate();
  const isEdit = !!id;

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [tags, setTags] = useState<Record<string, string>>({});
  const [db, setDb] = useState<DatabaseVM>(() => freshDatabase("postgres"));
  const [w, setW] = useState<WorkloadVM>(() => freshWorkload());

  // Edit-mode load state.
  const [loading, setLoading] = useState(isEdit);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [isSystem, setIsSystem] = useState(false);

  // Save state.
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  useBreadcrumbLabel("id", isEdit ? name || id : undefined);

  useEffect(() => {
    if (!isEdit || !slug || !id) return;
    let cancelled = false;
    setLoading(true);
    setLoadError(null);
    getPresetProvider()
      .getTestPreset(slug, id)
      .then((p) => {
        if (cancelled) return;
        setName(p.name);
        setDescription(p.description);
        setTags(p.tags);
        setDb(p.database);
        setW(p.workload);
        setIsSystem(p.isSystem);
      })
      .catch((e) => !cancelled && setLoadError(e instanceof Error ? e.message : String(e)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [isEdit, slug, id]);

  // Validation mirrors the backend (domain.Database.Validate + domain.Workload
  // .Validate — a Test must validate BOTH halves). Save is blocked while any
  // error-severity issue (or an empty name) remains.
  const dbErrors = useMemo(() => validateDatabase(db), [db]);
  const wErrors = useMemo(() => validateWorkload(w), [w]);
  const blocking = [
    ...dbErrors.filter((e) => e.severity === "error"),
    ...wErrors.filter((e) => e.severity === "error"),
  ];
  const nameMissing = !name.trim();
  const canSave = !saving && !nameMissing && blocking.length === 0 && !isSystem;

  const pickEngine = useCallback((engine: EngineKind) => {
    setDb((cur) => (cur.kind === engine ? cur : freshDatabase(engine)));
  }, []);

  const onSave = useCallback(async () => {
    if (!slug) return;
    setSaving(true);
    setSaveError(null);
    const input: TestPresetInput = {
      name: name.trim(),
      description: description.trim(),
      tags,
      database: db,
      workload: w,
    };
    try {
      const provider = getPresetProvider();
      if (isEdit && id) {
        await provider.updateTestPreset(slug, id, input);
        navigate(`/presets/test/${id}`);
      } else {
        const newId = await provider.createTestPreset(slug, input);
        navigate(`/presets/test/${newId}`);
      }
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : String(e));
      setSaving(false);
    }
  }, [slug, isEdit, id, name, description, tags, db, w, navigate]);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-zinc-500">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Loading preset…
      </div>
    );
  }
  if (loadError) {
    return (
      <div className="p-5">
        <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-4 w-4" /> {loadError}
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-zinc-800/80 px-5 py-3">
        <div className="flex items-center gap-3 min-w-0">
          <button
            type="button"
            onClick={() => navigate(isEdit && id ? `/presets/test/${id}` : "/presets/test")}
            className="flex h-7 w-7 shrink-0 items-center justify-center border border-zinc-800 text-zinc-500 transition-colors hover:border-zinc-700 hover:text-zinc-300"
            aria-label="Back"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
          <div className="min-w-0">
            <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
              {isEdit ? "Edit test preset" : "New test preset"}
            </div>
            <h1 className="truncate text-base font-semibold tracking-tight text-foreground">
              {name || (isEdit ? "Untitled preset" : "New test preset")}
            </h1>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => navigate(isEdit && id ? `/presets/test/${id}` : "/presets/test")}
          >
            Cancel
          </Button>
          <Button size="sm" disabled={!canSave} onClick={() => void onSave()}>
            {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
            {isEdit ? "Save changes" : "Create preset"}
          </Button>
        </div>
      </div>

      {isSystem && (
        <div className="mx-5 mt-4 flex items-center gap-2 border border-amber-900/50 bg-amber-950/20 px-3 py-2 text-xs text-amber-400">
          <Lock className="h-3.5 w-3.5 shrink-0" />
          System presets are platform-seeded and read-only. Duplicate it to create an editable copy.
        </div>
      )}

      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5">
        <div className="mx-auto max-w-6xl space-y-6">
          {/* Identity */}
          <section className="space-y-4">
            <SectionLabel>Identity</SectionLabel>
            <div className="grid gap-4 sm:grid-cols-2">
              <div>
                <Label>Name</Label>
                <Input
                  className="mt-1"
                  value={name}
                  disabled={isSystem}
                  placeholder="e.g. Nightly PG TPC-C"
                  onChange={(e) => setName(e.target.value)}
                />
                {nameMissing && (
                  <p className="mt-1 text-[11px] text-red-400">A preset needs a name.</p>
                )}
              </div>
              <div>
                <Label>Description</Label>
                <Input
                  className="mt-1"
                  value={description}
                  disabled={isSystem}
                  placeholder="Short blurb shown in the picker"
                  onChange={(e) => setDescription(e.target.value)}
                />
              </div>
            </div>
            <TagsEditor tags={tags} onChange={setTags} disabled={isSystem} />
          </section>

          {/* Database section — the domain.Database half. */}
          <section className="space-y-4 border border-zinc-800/60 bg-[#0a0a0a]/40 p-4">
            <div className="flex items-center gap-2">
              <DatabaseIcon className="h-4 w-4 text-primary/70" />
              <SectionLabel>Database under test</SectionLabel>
            </div>
            <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
              <div className="space-y-4">
                {/* Engine */}
                <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                  {ENGINES.map((meta) => {
                    const sel = db.kind === meta.kind;
                    return (
                      <button
                        key={meta.kind}
                        type="button"
                        disabled={isSystem}
                        onClick={() => pickEngine(meta.kind)}
                        className={`flex items-center gap-2 border p-2.5 text-left text-xs transition-all disabled:cursor-not-allowed disabled:opacity-60 ${
                          sel
                            ? "border-primary/50 bg-primary/[0.07] text-primary"
                            : "border-zinc-800 bg-[#0a0a0a] text-foreground hover:border-zinc-700 hover:bg-zinc-900/50"
                        }`}
                      >
                        <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: meta.hex }} />
                        <span className="truncate font-medium">{meta.label}</span>
                        {sel && <Check className="ml-auto h-3.5 w-3.5 shrink-0" />}
                      </button>
                    );
                  })}
                </div>
                <EngineVersionSelect db={db} apply={setDb} />
                <div className={isSystem ? "pointer-events-none opacity-60" : undefined}>
                  <EngineParamsForm db={db} apply={setDb} advancedInitiallyOpen={isSystem} />
                </div>
                <FieldErrors errs={errorsFor(dbErrors, "database")} />
              </div>

              {/* Topology preview */}
              <aside className="xl:sticky xl:top-0 xl:self-start">
                <SectionLabel>Derived topology</SectionLabel>
                <div className="mt-3">
                  <TopologyPreview database={db} />
                </div>
              </aside>
            </div>
          </section>

          {/* Workload section — the domain.Workload half. */}
          <section className="space-y-4 border border-zinc-800/60 bg-[#0a0a0a]/40 p-4">
            <div className="flex items-center gap-2">
              <Gauge className="h-4 w-4 text-primary/70" />
              <SectionLabel>Workload</SectionLabel>
            </div>
            <div className="sm:max-w-xs">
              <Label>Stroppy version (optional)</Label>
              <Input
                className="mt-1 font-mono text-xs"
                value={w.stroppyVersion}
                disabled={isSystem}
                placeholder="e.g. 5.1.2 — empty defers to the run"
                onChange={(e) => setW({ ...w, stroppyVersion: e.target.value })}
              />
              <p className="mt-1 text-[11px] text-zinc-600">
                Pins the stroppy build this workload targets; leave empty to let the run choose.
              </p>
            </div>
            <WorkloadParamsForm w={w} apply={setW} disabled={isSystem} />
            <FieldErrors errs={errorsFor(wErrors, "workload")} />
          </section>

          {saveError && (
            <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
              <AlertCircle className="h-4 w-4 shrink-0" /> {saveError}
            </div>
          )}
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

// --- Tags editor (entity.labels) ---------------------------------------------

function TagsEditor({
  tags,
  onChange,
  disabled,
}: {
  tags: Record<string, string>;
  onChange: (next: Record<string, string>) => void;
  disabled?: boolean;
}) {
  const [k, setK] = useState("");
  const [v, setV] = useState("");
  const entries = Object.entries(tags);

  const add = () => {
    const key = k.trim();
    if (!key) return;
    onChange({ ...tags, [key]: v.trim() });
    setK("");
    setV("");
  };
  const remove = (key: string) => {
    const next = { ...tags };
    delete next[key];
    onChange(next);
  };

  return (
    <div>
      <Label>Tags</Label>
      {entries.length > 0 && (
        <div className="mt-1.5 flex flex-wrap gap-1.5">
          {entries.map(([key, val]) => (
            <span
              key={key}
              className="inline-flex items-center gap-1 border border-zinc-800 bg-[#0a0a0a] px-2 py-0.5 font-mono text-[11px] text-zinc-300"
            >
              <span className="text-zinc-500">{key}</span>
              {val && <span className="text-zinc-600">=</span>}
              {val && <span>{val}</span>}
              {!disabled && (
                <button
                  type="button"
                  onClick={() => remove(key)}
                  className="ml-0.5 text-zinc-600 transition-colors hover:text-red-400"
                  aria-label={`Remove tag ${key}`}
                >
                  <X className="h-3 w-3" />
                </button>
              )}
            </span>
          ))}
        </div>
      )}
      {!disabled && (
        <div className="mt-2 flex items-center gap-2">
          <Input
            className="h-8 max-w-[10rem] font-mono text-xs"
            placeholder="key"
            value={k}
            onChange={(e) => setK(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), add())}
          />
          <Input
            className="h-8 max-w-[12rem] font-mono text-xs"
            placeholder="value (optional)"
            value={v}
            onChange={(e) => setV(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), add())}
          />
          <Button type="button" variant="outline" size="sm" onClick={add} disabled={!k.trim()}>
            <Plus className="h-3.5 w-3.5" /> Add
          </Button>
        </div>
      )}
    </div>
  );
}
