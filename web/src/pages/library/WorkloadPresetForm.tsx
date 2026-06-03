// Workload Preset authoring — CREATE (/presets/workload/new) and EDIT
// (/presets/workload/:id/edit).
//
// Built against cloud.v1.api.WorkloadPresetService: the form edits one
// cloud.v1.models.WorkloadPresetRecord = Entity (name/description/tags) + the
// typed cloud.v1.domain.Workload (script/sql/protocol/stroppy version, k6
// Execution, data Parameters). It REUSES the wizard's workload editor
// (WorkloadParamsForm) and the same domain.Workload.Validate mirror
// (validateWorkload) so the create/edit surface is identical to the New Run
// Workload step's Parameters pane.
//
//   Create → PresetProvider.createWorkloadPreset(slug, input)  → CreateWorkloadPreset
//   Edit   → PresetProvider.getWorkloadPreset(slug, id) to load,
//            then PresetProvider.updateWorkloadPreset(slug, id, input) → UpdateWorkloadPreset
//
// System presets are platform-seeded and read-only: editing one is gated out
// (the page shows a clone hint instead). On save the page navigates to the
// preset's detail page (/presets/workload/:id).

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AlertCircle,
  ArrowLeft,
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
import { defaultWorkload, type WorkloadVM } from "@/services/wizard";
import { getPresetProvider, type WorkloadPresetInput } from "@/services/preset";
import {
  WorkloadParamsForm,
  FieldErrors,
  errorsFor,
  validateWorkload,
} from "@/components/workload/WorkloadParamsForm";

/** A fresh, immediately-valid WorkloadVM (TPC-C defaults, no version). */
function freshWorkload(): WorkloadVM {
  return { ...defaultWorkload("postgres"), stroppyVersion: "" };
}

export function WorkloadPresetForm() {
  const slug = useTenantSlug() ?? "";
  const { id } = useParams();
  const navigate = useNavigate();
  const isEdit = !!id;

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [tags, setTags] = useState<Record<string, string>>({});
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
      .getWorkloadPreset(slug, id)
      .then((p) => {
        if (cancelled) return;
        setName(p.name);
        setDescription(p.description);
        setTags(p.tags);
        setW(p.workload);
        setIsSystem(p.isSystem);
      })
      .catch((e) => !cancelled && setLoadError(e instanceof Error ? e.message : String(e)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [isEdit, slug, id]);

  // Validation mirrors the backend (domain.Workload.Validate). Save is blocked
  // while any error-severity issue (or an empty name) remains.
  const wErrors = useMemo(() => validateWorkload(w), [w]);
  const blocking = wErrors.filter((e) => e.severity === "error");
  const nameMissing = !name.trim();
  const canSave = !saving && !nameMissing && blocking.length === 0 && !isSystem;

  const onSave = useCallback(async () => {
    if (!slug) return;
    setSaving(true);
    setSaveError(null);
    const input: WorkloadPresetInput = {
      name: name.trim(),
      description: description.trim(),
      tags,
      workload: w,
    };
    try {
      const provider = getPresetProvider();
      if (isEdit && id) {
        await provider.updateWorkloadPreset(slug, id, input);
        navigate(`/presets/workload/${id}`);
      } else {
        const newId = await provider.createWorkloadPreset(slug, input);
        navigate(`/presets/workload/${newId}`);
      }
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : String(e));
      setSaving(false);
    }
  }, [slug, isEdit, id, name, description, tags, w, navigate]);

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
            onClick={() => navigate(isEdit && id ? `/presets/workload/${id}` : "/presets/workload")}
            className="flex h-7 w-7 shrink-0 items-center justify-center border border-zinc-800 text-zinc-500 transition-colors hover:border-zinc-700 hover:text-zinc-300"
            aria-label="Back"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
          <div className="min-w-0">
            <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
              {isEdit ? "Edit workload preset" : "New workload preset"}
            </div>
            <h1 className="truncate text-base font-semibold tracking-tight text-foreground">
              {name || (isEdit ? "Untitled preset" : "New workload preset")}
            </h1>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => navigate(isEdit && id ? `/presets/workload/${id}` : "/presets/workload")}
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
        <div className="mx-auto max-w-3xl space-y-6">
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
                  placeholder="e.g. TPC-C 15m"
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
            <TagsEditor tags={tags} onChange={setTags} disabled={isSystem} />
          </section>

          {/* Typed workload params */}
          <section className="space-y-4">
            <SectionLabel>Configuration</SectionLabel>
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
