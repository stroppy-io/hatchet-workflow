import { useEffect, useState, type ReactNode } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { ArrowLeft, Copy, Save, Trash2 } from "lucide-react";
import { create } from "@bufbuild/protobuf";

import { useConfirm } from "@/components/confirm-dialog";
import { StateBlock } from "@/components/state-block";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAction } from "@/hooks/use-action";
import { useListQuery } from "@/hooks/use-list-query";
import { api } from "@/lib/connect";
import { presetTitle } from "@/lib/preset-summary";
import { idMessage, tenantIdMessage } from "@/lib/proto";
import { tenantPath } from "@/lib/routes";
import { notifyError, notifySuccess } from "@/lib/toast";
import { PresetSchema, type Preset, type Preset_Kind } from "@/lib/proto/cloud/v1/models/preset_pb.ts";

type PresetEditorShellProps = {
  tenantId: string;
  kind: Preset_Kind;
  presetId: string;
  routePart: string;
  children: (draft: Preset, setDraft: (next: Preset) => void) => ReactNode;
};

// Reusable detail/edit scaffold for every preset kind. No GetPreset RPC exists,
// so it finds the preset in a ListPresets page (fallback) and prefers the one
// passed via router state from the catalog card. Holds a draft, persists with
// UpdatePreset, and offers Clone/Delete.
export function PresetEditorShell({ tenantId, kind, presetId, routePart, children }: PresetEditorShellProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const confirm = useConfirm();
  const [reloadKey, setReloadKey] = useState(0);

  const listResult = useListQuery(
    () => api.preset.listPresets({ tenantId: tenantIdMessage(tenantId), kinds: [kind], page: { size: 200 } }),
    [tenantId, kind, reloadKey],
  );

  const statePreset = (location.state as { preset?: Preset } | null)?.preset;
  const fetched = listResult.data?.presets.find((preset) => preset.entity?.id?.value === presetId);
  const preset = fetched ?? (statePreset?.entity?.id?.value === presetId ? statePreset : undefined);

  const [draft, setDraft] = useState<Preset | null>(null);
  useEffect(() => {
    if (preset) setDraft(preset);
  }, [preset]);

  const save = useAction(() => api.preset.updatePreset(draft as Preset));

  async function onSave() {
    if (!draft) return;
    const response = await save.run();
    if (!response) return;
    notifySuccess("Preset saved");
    setReloadKey((key) => key + 1);
  }

  async function onClone() {
    try {
      await api.preset.clonePreset({ tenantId: tenantIdMessage(tenantId), id: idMessage(presetId) });
      notifySuccess("Preset cloned");
      navigate(tenantPath(tenantId, `/presets/${routePart}`));
    } catch (error) {
      notifyError(error, "Could not clone preset");
    }
  }

  async function onDelete() {
    const ok = await confirm({ title: "Delete preset?", danger: true });
    if (!ok) return;
    try {
      await api.preset.deletePreset({ tenantId: tenantIdMessage(tenantId), id: idMessage(presetId) });
      notifySuccess("Preset deleted");
      navigate(tenantPath(tenantId, `/presets/${routePart}`));
    } catch (error) {
      notifyError(error, "Could not delete preset");
    }
  }

  const listLoading = listResult.loading && !preset;

  return (
    <section className="flex min-h-0 flex-col gap-5 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon-sm" onClick={() => navigate(tenantPath(tenantId, `/presets/${routePart}`))}>
            <ArrowLeft />
          </Button>
          <div>
            <h1 className="text-xl font-semibold tracking-normal">{draft ? presetTitle(draft) : "Preset"}</h1>
            <p className="mt-1 font-mono text-xs text-muted-foreground">{presetId}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={onClone}>
            <Copy />
            Clone
          </Button>
          <Button variant="outline" size="sm" onClick={onDelete}>
            <Trash2 />
            Delete
          </Button>
          <Button size="sm" onClick={onSave} disabled={!draft || save.loading}>
            <Save />
            {save.loading ? "Saving…" : "Save"}
          </Button>
        </div>
      </div>

      {save.error ? <div className="border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger">{save.error}</div> : null}

      <StateBlock loading={listLoading} error={listResult.error} empty={!listLoading && !draft} emptyMessage="Preset not found.">
        {draft ? (
          <div className="space-y-6">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label>Name</Label>
                <Input value={draft.name ?? ""} onChange={(event) => setDraft(create(PresetSchema, { ...draft, name: event.target.value }))} />
              </div>
              <div className="space-y-1.5">
                <Label>Description</Label>
                <Input value={draft.description ?? ""} onChange={(event) => setDraft(create(PresetSchema, { ...draft, description: event.target.value }))} />
              </div>
            </div>
            {children(draft, setDraft)}
          </div>
        ) : null}
      </StateBlock>
    </section>
  );
}
