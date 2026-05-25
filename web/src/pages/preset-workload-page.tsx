import { create } from "@bufbuild/protobuf";
import { useParams } from "react-router-dom";

import { Section } from "@/components/editors/fields";
import { TopologyView } from "@/components/editors/topology-view";
import { WorkloadEditor } from "@/components/editors/workload-editor";
import { PresetEditorShell } from "@/components/preset-editor-shell";
import { useTenantId } from "@/hooks/use-tenant-id";
import { TopologySchema } from "@/lib/proto/cloud/v1/domain/topology_pb.ts";
import { WorkloadPresetSchema, WorkloadSchema } from "@/lib/proto/cloud/v1/domain/workload_pb.ts";
import { Preset_Kind, PresetSchema } from "@/lib/proto/cloud/v1/models/preset_pb.ts";

export function PresetWorkloadPage() {
  const tenantId = useTenantId();
  const { presetId } = useParams();

  return (
    <PresetEditorShell tenantId={tenantId} kind={Preset_Kind.WORKLOAD} presetId={presetId ?? ""} routePart="workload">
      {(draft, setDraft) => {
        if (draft.preset.case !== "workloadPreset") {
          return <p className="text-sm text-muted-foreground">Not a workload preset.</p>;
        }
        const workloadPreset = draft.preset.value;
        const setWorkloadPreset = (next: typeof workloadPreset) =>
          setDraft(create(PresetSchema, { ...draft, preset: { case: "workloadPreset", value: next } }));

        return (
          <div className="space-y-6">
            <WorkloadEditor
              value={workloadPreset.workload ?? create(WorkloadSchema, {})}
              onChange={(workload) => setWorkloadPreset(create(WorkloadPresetSchema, { ...workloadPreset, workload }))}
            />
            <Section title="Topology (rendered from settings)">
              <TopologyView value={workloadPreset.topology ?? create(TopologySchema, {})} />
            </Section>
          </div>
        );
      }}
    </PresetEditorShell>
  );
}
