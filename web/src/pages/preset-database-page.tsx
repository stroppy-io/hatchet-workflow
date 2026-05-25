import { create } from "@bufbuild/protobuf";
import { useParams } from "react-router-dom";

import { DatabaseEditor } from "@/components/editors/database-editor";
import { Section } from "@/components/editors/fields";
import { TopologyView } from "@/components/editors/topology-view";
import { PresetEditorShell } from "@/components/preset-editor-shell";
import { useTenantId } from "@/hooks/use-tenant-id";
import { DatabasePresetSchema, DatabaseSchema } from "@/lib/proto/cloud/v1/domain/database_pb.ts";
import { TopologySchema } from "@/lib/proto/cloud/v1/domain/topology_pb.ts";
import { Preset_Kind, PresetSchema } from "@/lib/proto/cloud/v1/models/preset_pb.ts";

export function PresetDatabasePage() {
  const tenantId = useTenantId();
  const { presetId } = useParams();

  return (
    <PresetEditorShell tenantId={tenantId} kind={Preset_Kind.DATABASE} presetId={presetId ?? ""} routePart="database">
      {(draft, setDraft) => {
        if (draft.preset.case !== "databasePreset") {
          return <p className="text-sm text-muted-foreground">Not a database preset.</p>;
        }
        const databasePreset = draft.preset.value;
        const setDatabasePreset = (next: typeof databasePreset) =>
          setDraft(create(PresetSchema, { ...draft, preset: { case: "databasePreset", value: next } }));

        return (
          <div className="space-y-6">
            <DatabaseEditor
              value={databasePreset.database ?? create(DatabaseSchema, {})}
              onChange={(database) => setDatabasePreset(create(DatabasePresetSchema, { ...databasePreset, database }))}
            />
            <Section title="Topology (rendered from settings)">
              <TopologyView value={databasePreset.topology ?? create(TopologySchema, {})} />
            </Section>
          </div>
        );
      }}
    </PresetEditorShell>
  );
}
