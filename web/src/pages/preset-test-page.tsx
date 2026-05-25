import { create } from "@bufbuild/protobuf";
import { Play } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";

import { DatabaseEditor } from "@/components/editors/database-editor";
import { Section } from "@/components/editors/fields";
import { RawProtoEditor } from "@/components/editors/raw-proto-editor";
import { TopologyView } from "@/components/editors/topology-view";
import { WorkloadEditor } from "@/components/editors/workload-editor";
import { PresetEditorShell } from "@/components/preset-editor-shell";
import { Button } from "@/components/ui/button";
import { useAction } from "@/hooks/use-action";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { tenantIdMessage } from "@/lib/proto";
import { tenantPath } from "@/lib/routes";
import { notifySuccess } from "@/lib/toast";
import { DatabaseSchema } from "@/lib/proto/cloud/v1/domain/database_pb.ts";
import { TestPresetSchema, type TestPreset } from "@/lib/proto/cloud/v1/domain/test_pb.ts";
import { TopologySchema } from "@/lib/proto/cloud/v1/domain/topology_pb.ts";
import { WorkloadSchema } from "@/lib/proto/cloud/v1/domain/workload_pb.ts";
import { DeploymentIntentSchema } from "@/lib/proto/cloud/v1/deployment/deployment_pb.ts";
import { Preset_Kind, PresetSchema } from "@/lib/proto/cloud/v1/models/preset_pb.ts";

export function PresetTestPage() {
  const tenantId = useTenantId();
  const { presetId } = useParams();

  return (
    <PresetEditorShell tenantId={tenantId} kind={Preset_Kind.TEST} presetId={presetId ?? ""} routePart="test">
      {(draft, setDraft) => {
        if (draft.preset.case !== "testPreset") {
          return <p className="text-sm text-muted-foreground">Not a test preset.</p>;
        }
        const testPreset = draft.preset.value;
        const setTestPreset = (next: TestPreset) =>
          setDraft(create(PresetSchema, { ...draft, preset: { case: "testPreset", value: next } }));

        return (
          <div className="space-y-8">
            <Section title="Database">
              <DatabaseEditor
                value={testPreset.database ?? create(DatabaseSchema, {})}
                onChange={(database) => setTestPreset(create(TestPresetSchema, { ...testPreset, database }))}
              />
            </Section>
            <Section title="Workload">
              <WorkloadEditor
                value={testPreset.workload ?? create(WorkloadSchema, {})}
                onChange={(workload) => setTestPreset(create(TestPresetSchema, { ...testPreset, workload }))}
              />
            </Section>
            <Section title="Topology (rendered)">
              <TopologyView value={testPreset.topology ?? create(TopologySchema, {})} />
            </Section>
            <Section title="Deployment">
              <p className="text-xs text-muted-foreground">Materialized from the topology + provider; edit the raw intent and Apply.</p>
              <RawProtoEditor
                schema={DeploymentIntentSchema}
                value={testPreset.deployment ?? create(DeploymentIntentSchema, {})}
                onChange={(deployment) => setTestPreset(create(TestPresetSchema, { ...testPreset, deployment }))}
              />
            </Section>
            <LaunchRun tenantId={tenantId} testPreset={testPreset} />
          </div>
        );
      }}
    </PresetEditorShell>
  );
}

function LaunchRun({ tenantId, testPreset }: { tenantId: string; testPreset: TestPreset }) {
  const navigate = useNavigate();
  const launch = useAction(() => api.run.submitTestRun({ tenantId: tenantIdMessage(tenantId), testPreset }));

  async function go() {
    const response = await launch.run();
    if (!response) return;
    notifySuccess("Run submitted");
    navigate(tenantPath(tenantId, `/runs/${response.entity?.id?.value ?? ""}`));
  }

  return (
    <Section title="Launch">
      <div className="flex items-center gap-3">
        <Button onClick={go} disabled={launch.loading}>
          <Play />
          {launch.loading ? "Launching…" : "Launch run"}
        </Button>
        {launch.error ? <span className="text-sm text-destructive">{launch.error}</span> : null}
      </div>
    </Section>
  );
}
