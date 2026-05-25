import { create } from "@bufbuild/protobuf";
import { Plus, X } from "lucide-react";

import { ConfigOverridesEditor } from "@/components/editors/config-overrides-editor";
import { NumberField, SelectField, Section, SwitchField, TextField, enumAuto } from "@/components/editors/fields";
import { KeyValueEditor } from "@/components/editors/key-value-editor";
import { StringListEditor } from "@/components/editors/string-list-editor";
import { ConfigSchema } from "@/lib/proto/cloud/v1/runtime/render/config_pb.ts";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Database_Kind,
  DatabaseSchema,
  Database_OptionsSchema,
  Database_TargetSchema,
  Database_Options_CockroachSchema,
  Database_Options_MysqlSchema,
  Database_Options_Mysql_AccessSchema,
  Database_Options_Mysql_ReplicationSchema,
  Database_Options_Mysql_Replication_Mode,
  Database_Options_PicodataSchema,
  Database_Options_Picodata_AccessSchema,
  Database_Options_Picodata_TierSchema,
  Database_Options_PostgresSchema,
  Database_Options_Postgres_AccessSchema,
  Database_Options_Postgres_ReplicationSchema,
  Database_Options_Postgres_Replication_Mode,
  Database_Options_YdbSchema,
  Database_Options_Ydb_AccessSchema,
  Database_Options_Ydb_ManagedSchema,
  Database_Options_Ydb_Managed_AutoScaleSchema,
  Database_Options_Ydb_Managed_ComputeType,
  Database_Options_Ydb_Managed_Kind,
  Database_Options_Ydb_Managed_LocationId,
  Database_Options_Ydb_Managed_ResourcePresetId,
  Database_Options_Ydb_Managed_StorageTypeId,
  Database_Options_Ydb_SelfHostedSchema,
  Database_Options_Ydb_SelfHosted_FailureDomain,
  Database_Options_Ydb_SelfHosted_FaultTolerance,
  Database_Target_ExternalSchema,
  Database_Target_SelfHostedSchema,
  type Database,
  type Database_Options_Cockroach,
  type Database_Options_Mysql,
  type Database_Options_Picodata,
  type Database_Options_Postgres,
  type Database_Options_Ydb,
  type Database_Target_External,
} from "@/lib/proto/cloud/v1/domain/database_pb.ts";

const KIND_OPTIONS = enumAuto(Database_Kind as unknown as Record<string, number>);
const PG_MODE = enumAuto(Database_Options_Postgres_Replication_Mode as unknown as Record<string, number>);
const MYSQL_MODE = enumAuto(Database_Options_Mysql_Replication_Mode as unknown as Record<string, number>);
const YDB_FAULT = enumAuto(Database_Options_Ydb_SelfHosted_FaultTolerance as unknown as Record<string, number>);
const YDB_DOMAIN = enumAuto(Database_Options_Ydb_SelfHosted_FailureDomain as unknown as Record<string, number>);
const YDB_MANAGED_KIND = enumAuto(Database_Options_Ydb_Managed_Kind as unknown as Record<string, number>);
const YDB_COMPUTE = enumAuto(Database_Options_Ydb_Managed_ComputeType as unknown as Record<string, number>);
const YDB_PRESET = enumAuto(Database_Options_Ydb_Managed_ResourcePresetId as unknown as Record<string, number>);
const YDB_STORAGE = enumAuto(Database_Options_Ydb_Managed_StorageTypeId as unknown as Record<string, number>);
const YDB_LOCATION = enumAuto(Database_Options_Ydb_Managed_LocationId as unknown as Record<string, number>);

const TARGET_OPTIONS = [
  { label: "Self-hosted", value: "selfHosted" },
  { label: "External", value: "external" },
];

const YDB_ROLLOUT = [
  { label: "Self-hosted", value: "selfHosted" },
  { label: "Managed", value: "managed" },
];

function optionCaseForKind(kind: Database_Kind): "postgres" | "mysql" | "ydb" | "cockroach" | "picodata" {
  switch (kind) {
    case Database_Kind.MYSQL:
    case Database_Kind.MARIADB:
      return "mysql";
    case Database_Kind.YDB:
      return "ydb";
    case Database_Kind.COCKROACH:
      return "cockroach";
    case Database_Kind.PICODATA:
      return "picodata";
    default:
      return "postgres";
  }
}

export function DatabaseEditor({ value, onChange }: { value: Database; onChange: (value: Database) => void }) {
  const patch = (partial: Partial<Database>) => onChange(create(DatabaseSchema, { ...value, ...partial }));
  // target/options are wrapper messages; the oneof lives one level deeper.
  const targetCase = value.target?.target.case ?? "selfHosted";
  const optionCase = optionCaseForKind(value.kind);

  function setKind(kind: Database_Kind) {
    const nextCase = optionCaseForKind(kind);
    if (value.options?.options.case === nextCase) {
      patch({ kind });
      return;
    }
    switch (nextCase) {
      case "postgres":
        patch({ kind, options: create(Database_OptionsSchema, { options: { case: "postgres", value: create(Database_Options_PostgresSchema, {}) } }) });
        break;
      case "mysql":
        patch({ kind, options: create(Database_OptionsSchema, { options: { case: "mysql", value: create(Database_Options_MysqlSchema, {}) } }) });
        break;
      case "ydb":
        patch({ kind, options: create(Database_OptionsSchema, { options: { case: "ydb", value: create(Database_Options_YdbSchema, {}) } }) });
        break;
      case "cockroach":
        patch({ kind, options: create(Database_OptionsSchema, { options: { case: "cockroach", value: create(Database_Options_CockroachSchema, {}) } }) });
        break;
      case "picodata":
        patch({ kind, options: create(Database_OptionsSchema, { options: { case: "picodata", value: create(Database_Options_PicodataSchema, {}) } }) });
        break;
    }
  }

  function setTargetCase(next: string) {
    if (next === "external") {
      const external = value.target?.target.case === "external" ? value.target.target.value : create(Database_Target_ExternalSchema, {});
      patch({ target: create(Database_TargetSchema, { target: { case: "external", value: external } }) });
    } else {
      patch({ target: create(Database_TargetSchema, { target: { case: "selfHosted", value: create(Database_Target_SelfHostedSchema, {}) } }) });
    }
  }

  return (
    <div className="space-y-6">
      <Section title="Database">
        <div className="grid gap-4 sm:grid-cols-2">
          <SelectField label="Engine" value={String(value.kind)} onChange={(v) => setKind(Number(v) as Database_Kind)} options={KIND_OPTIONS} />
          <TextField label="Version" value={value.version} onChange={(v) => patch({ version: v })} placeholder="16" />
        </div>
      </Section>

      <Section title="Target">
        <SelectField label="Target" value={targetCase} onChange={setTargetCase} options={TARGET_OPTIONS} />
        {targetCase === "external" ? (
          <ExternalTargetEditor
            value={value.target?.target.case === "external" ? value.target.target.value : create(Database_Target_ExternalSchema, {})}
            onChange={(external) => patch({ target: create(Database_TargetSchema, { target: { case: "external", value: external } }) })}
          />
        ) : (
          <p className="text-xs text-muted-foreground">Ephemeral self-hosted database provisioned by the run.</p>
        )}
      </Section>

      <Section title="Engine options">
        {optionCase === "postgres" ? (
          <PostgresEditor
            value={value.options?.options.case === "postgres" ? value.options.options.value : create(Database_Options_PostgresSchema, {})}
            onChange={(next) => patch({ options: create(Database_OptionsSchema, { options: { case: "postgres", value: next } }) })}
          />
        ) : null}
        {optionCase === "mysql" ? (
          <MysqlEditor
            value={value.options?.options.case === "mysql" ? value.options.options.value : create(Database_Options_MysqlSchema, {})}
            onChange={(next) => patch({ options: create(Database_OptionsSchema, { options: { case: "mysql", value: next } }) })}
          />
        ) : null}
        {optionCase === "ydb" ? (
          <YdbEditor
            value={value.options?.options.case === "ydb" ? value.options.options.value : create(Database_Options_YdbSchema, {})}
            onChange={(next) => patch({ options: create(Database_OptionsSchema, { options: { case: "ydb", value: next } }) })}
          />
        ) : null}
        {optionCase === "cockroach" ? (
          <CockroachEditor
            value={value.options?.options.case === "cockroach" ? value.options.options.value : create(Database_Options_CockroachSchema, {})}
            onChange={(next) => patch({ options: create(Database_OptionsSchema, { options: { case: "cockroach", value: next } }) })}
          />
        ) : null}
        {optionCase === "picodata" ? (
          <PicodataEditor
            value={value.options?.options.case === "picodata" ? value.options.options.value : create(Database_Options_PicodataSchema, {})}
            onChange={(next) => patch({ options: create(Database_OptionsSchema, { options: { case: "picodata", value: next } }) })}
          />
        ) : null}
      </Section>

      <Section title="Config overrides">
        <p className="text-xs text-muted-foreground">The config is rendered from the settings above; add overrides to patch specific rendered items.</p>
        <ConfigOverridesEditor value={value.config ?? create(ConfigSchema, {})} onChange={(config) => patch({ config })} />
      </Section>
    </div>
  );
}

function ExternalTargetEditor({ value, onChange }: { value: Database_Target_External; onChange: (value: Database_Target_External) => void }) {
  const patch = (partial: Partial<Database_Target_External>) => onChange(create(Database_Target_ExternalSchema, { ...value, ...partial }));
  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <TextField label="Endpoint" value={value.endpoint} onChange={(v) => patch({ endpoint: v })} placeholder="host:5432" />
        <TextField label="Database" value={value.database} onChange={(v) => patch({ database: v })} />
        <TextField label="Username" value={value.username} onChange={(v) => patch({ username: v })} />
        <TextField label="Password" type="password" value={value.password} onChange={(v) => patch({ password: v })} />
        <TextField label="SSL mode" value={value.sslMode} onChange={(v) => patch({ sslMode: v })} placeholder="disable" />
      </div>
      <KeyValueEditor label="Extra params" value={value.extraParams} onChange={(extraParams) => patch({ extraParams })} />
    </div>
  );
}

function PostgresEditor({ value, onChange }: { value: Database_Options_Postgres; onChange: (value: Database_Options_Postgres) => void }) {
  const patch = (partial: Partial<Database_Options_Postgres>) => onChange(create(Database_Options_PostgresSchema, { ...value, ...partial }));
  const replication = value.replication ?? create(Database_Options_Postgres_ReplicationSchema, {});
  const access = value.access ?? create(Database_Options_Postgres_AccessSchema, {});
  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-3">
        <SelectField label="Replication" value={String(replication.mode)} onChange={(v) => patch({ replication: { ...replication, mode: Number(v) as Database_Options_Postgres_Replication_Mode } })} options={PG_MODE} />
        <NumberField label="Replicas" value={replication.replicas} min={0} onChange={(v) => patch({ replication: { ...replication, replicas: v } })} />
        <NumberField label="Sync replicas" value={replication.syncReplicas} min={0} onChange={(v) => patch({ replication: { ...replication, syncReplicas: v } })} />
      </div>
      <div className="grid gap-2 sm:grid-cols-2">
        <SwitchField label="PgBouncer" checked={access.pgbouncer} onCheckedChange={(c) => patch({ access: { ...access, pgbouncer: c } })} />
        <SwitchField label="HAProxy" checked={access.haproxy} onCheckedChange={(c) => patch({ access: { ...access, haproxy: c } })} />
      </div>
      <StringListEditor label="Extensions" value={value.extensions} onChange={(extensions) => patch({ extensions })} placeholder="pg_stat_statements" />
      <KeyValueEditor label="Parameters" value={value.parameters} onChange={(parameters) => patch({ parameters })} />
    </div>
  );
}

function MysqlEditor({ value, onChange }: { value: Database_Options_Mysql; onChange: (value: Database_Options_Mysql) => void }) {
  const patch = (partial: Partial<Database_Options_Mysql>) => onChange(create(Database_Options_MysqlSchema, { ...value, ...partial }));
  const replication = value.replication ?? create(Database_Options_Mysql_ReplicationSchema, {});
  const access = value.access ?? create(Database_Options_Mysql_AccessSchema, {});
  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <SelectField label="Replication" value={String(replication.mode)} onChange={(v) => patch({ replication: { ...replication, mode: Number(v) as Database_Options_Mysql_Replication_Mode } })} options={MYSQL_MODE} />
        <NumberField label="Replicas" value={replication.replicas} min={0} onChange={(v) => patch({ replication: { ...replication, replicas: v } })} />
      </div>
      <SwitchField label="ProxySQL" checked={access.proxysql} onCheckedChange={(c) => patch({ access: { ...access, proxysql: c } })} />
      <KeyValueEditor label="Parameters" value={value.parameters} onChange={(parameters) => patch({ parameters })} />
    </div>
  );
}

function CockroachEditor({ value, onChange }: { value: Database_Options_Cockroach; onChange: (value: Database_Options_Cockroach) => void }) {
  const patch = (partial: Partial<Database_Options_Cockroach>) => onChange(create(Database_Options_CockroachSchema, { ...value, ...partial }));
  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <NumberField label="Nodes" value={value.nodes} min={1} onChange={(v) => patch({ nodes: v })} />
        <NumberField label="Zone replicas" value={value.zoneReplicas} min={1} onChange={(v) => patch({ zoneReplicas: v })} />
      </div>
      <KeyValueEditor label="Parameters" value={value.parameters} onChange={(parameters) => patch({ parameters })} />
    </div>
  );
}

function PicodataEditor({ value, onChange }: { value: Database_Options_Picodata; onChange: (value: Database_Options_Picodata) => void }) {
  const patch = (partial: Partial<Database_Options_Picodata>) => onChange(create(Database_Options_PicodataSchema, { ...value, ...partial }));
  const access = value.access ?? create(Database_Options_Picodata_AccessSchema, {});
  return (
    <div className="space-y-4">
      <NumberField label="Shards" value={value.shards} min={0} onChange={(v) => patch({ shards: v })} />
      <SwitchField label="HAProxy" checked={access.haproxy} onCheckedChange={(c) => patch({ access: { ...access, haproxy: c } })} />
      <div className="space-y-2">
        <Label>Tiers</Label>
        {value.tiers.length === 0 ? <p className="text-xs text-muted-foreground">No tiers</p> : null}
        {value.tiers.map((tier, index) => (
          <div key={index} className="flex flex-wrap items-end gap-2 rounded-md border p-3">
            <div className="flex-1 space-y-1">
              <Label>Name</Label>
              <Input className="font-mono text-xs" value={tier.name} onChange={(event) => patch({ tiers: value.tiers.map((t, i) => (i === index ? { ...t, name: event.target.value } : t)) })} />
            </div>
            <NumberField label="Count" value={tier.count} min={1} onChange={(v) => patch({ tiers: value.tiers.map((t, i) => (i === index ? { ...t, count: v } : t)) })} />
            <NumberField label="Repl. factor" value={tier.replicationFactor} min={1} onChange={(v) => patch({ tiers: value.tiers.map((t, i) => (i === index ? { ...t, replicationFactor: v } : t)) })} />
            <div className="flex items-center gap-2 pb-2">
              <SwitchField label="Can vote" checked={tier.canVote} onCheckedChange={(c) => patch({ tiers: value.tiers.map((t, i) => (i === index ? { ...t, canVote: c } : t)) })} />
              <Button type="button" variant="ghost" size="icon-sm" onClick={() => patch({ tiers: value.tiers.filter((_, i) => i !== index) })}>
                <X className="size-3.5" />
              </Button>
            </div>
          </div>
        ))}
        <Button type="button" variant="outline" size="sm" onClick={() => patch({ tiers: [...value.tiers, create(Database_Options_Picodata_TierSchema, { count: 1, replicationFactor: 1 })] })}>
          <Plus className="size-3.5" />
          Add tier
        </Button>
      </div>
      <KeyValueEditor label="Parameters" value={value.parameters} onChange={(parameters) => patch({ parameters })} />
    </div>
  );
}

function YdbEditor({ value, onChange }: { value: Database_Options_Ydb; onChange: (value: Database_Options_Ydb) => void }) {
  const patch = (partial: Partial<Database_Options_Ydb>) => onChange(create(Database_Options_YdbSchema, { ...value, ...partial }));
  const access = value.access ?? create(Database_Options_Ydb_AccessSchema, {});
  const rolloutCase = value.rollout.case ?? "selfHosted";

  function setRollout(next: string) {
    if (next === "managed") {
      patch({ rollout: { case: "managed", value: value.rollout.case === "managed" ? value.rollout.value : create(Database_Options_Ydb_ManagedSchema, {}) } });
    } else {
      patch({ rollout: { case: "selfHosted", value: value.rollout.case === "selfHosted" ? value.rollout.value : create(Database_Options_Ydb_SelfHostedSchema, {}) } });
    }
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <TextField label="Database path" value={value.databasePath} onChange={(v) => patch({ databasePath: v })} placeholder="/cluster/db" />
        <SelectField label="Rollout" value={rolloutCase} onChange={setRollout} options={YDB_ROLLOUT} />
      </div>
      <SwitchField label="HAProxy" checked={access.haproxy} onCheckedChange={(c) => patch({ access: { ...access, haproxy: c } })} />

      {rolloutCase === "selfHosted" && value.rollout.case === "selfHosted" ? (
        (() => {
          const sh = value.rollout.value;
          const setSh = (partial: Partial<typeof sh>) => patch({ rollout: { case: "selfHosted", value: create(Database_Options_Ydb_SelfHostedSchema, { ...sh, ...partial }) } });
          return (
            <div className="grid gap-4 sm:grid-cols-3">
              <NumberField label="Storage nodes" value={sh.storageNodes} min={0} onChange={(v) => setSh({ storageNodes: v })} />
              <NumberField label="Database nodes" value={sh.databaseNodes} min={0} onChange={(v) => setSh({ databaseNodes: v })} />
              <NumberField label="Storage groups" value={sh.storageGroups} min={0} onChange={(v) => setSh({ storageGroups: v })} />
              <SelectField label="Fault tolerance" value={String(sh.faultTolerance)} onChange={(v) => setSh({ faultTolerance: Number(v) as Database_Options_Ydb_SelfHosted_FaultTolerance })} options={YDB_FAULT} />
              <SelectField label="Failure domain" value={String(sh.failureDomain)} onChange={(v) => setSh({ failureDomain: Number(v) as Database_Options_Ydb_SelfHosted_FailureDomain })} options={YDB_DOMAIN} />
            </div>
          );
        })()
      ) : null}

      {rolloutCase === "managed" && value.rollout.case === "managed" ? (
        (() => {
          const mg = value.rollout.value;
          const setMg = (partial: Partial<typeof mg>) => patch({ rollout: { case: "managed", value: create(Database_Options_Ydb_ManagedSchema, { ...mg, ...partial }) } });
          const autoScale = mg.autoScale ?? create(Database_Options_Ydb_Managed_AutoScaleSchema, {});
          return (
            <div className="space-y-4">
              <div className="grid gap-4 sm:grid-cols-3">
                <SelectField label="Kind" value={String(mg.kind)} onChange={(v) => setMg({ kind: Number(v) as Database_Options_Ydb_Managed_Kind })} options={YDB_MANAGED_KIND} />
                <SelectField label="Compute type" value={String(mg.computeType)} onChange={(v) => setMg({ computeType: Number(v) as Database_Options_Ydb_Managed_ComputeType })} options={YDB_COMPUTE} />
                <NumberField label="Node count" value={mg.nodeCount} min={1} onChange={(v) => setMg({ nodeCount: v })} />
                <SelectField label="Resource preset" value={String(mg.resourcePresetId)} onChange={(v) => setMg({ resourcePresetId: Number(v) as Database_Options_Ydb_Managed_ResourcePresetId })} options={YDB_PRESET} />
                <SelectField label="Storage type" value={String(mg.storageTypeId)} onChange={(v) => setMg({ storageTypeId: Number(v) as Database_Options_Ydb_Managed_StorageTypeId })} options={YDB_STORAGE} />
                <SelectField label="Location" value={String(mg.locationId)} onChange={(v) => setMg({ locationId: Number(v) as Database_Options_Ydb_Managed_LocationId })} options={YDB_LOCATION} />
                <NumberField label="Storage groups" value={mg.storageGroups} min={0} onChange={(v) => setMg({ storageGroups: v })} />
                <NumberField label="Throttling RCUs" value={mg.throttlingRcus} min={0} onChange={(v) => setMg({ throttlingRcus: v })} />
                <NumberField label="Provisioned RCUs" value={mg.provisionedRcus} min={0} onChange={(v) => setMg({ provisionedRcus: v })} />
                <NumberField label="Storage limit (GB)" value={mg.storageSizeLimitGb} min={0} onChange={(v) => setMg({ storageSizeLimitGb: v })} />
              </div>
              <div className="grid gap-2 sm:grid-cols-2">
                <SwitchField label="Deletion protection" checked={mg.deletionProtection} onCheckedChange={(c) => setMg({ deletionProtection: c })} />
                <SwitchField label="Assign public IPs" checked={mg.assignPublicIps} onCheckedChange={(c) => setMg({ assignPublicIps: c })} />
              </div>
              <div className="grid gap-4 sm:grid-cols-3">
                <NumberField label="Autoscale min" value={autoScale.minSize} min={1} onChange={(v) => setMg({ autoScale: create(Database_Options_Ydb_Managed_AutoScaleSchema, { ...autoScale, minSize: v }) })} />
                <NumberField label="Autoscale max" value={autoScale.maxSize} min={1} onChange={(v) => setMg({ autoScale: create(Database_Options_Ydb_Managed_AutoScaleSchema, { ...autoScale, maxSize: v }) })} />
                <NumberField label="Autoscale CPU %" value={autoScale.cpuUtilizationPercent} min={1} max={100} onChange={(v) => setMg({ autoScale: create(Database_Options_Ydb_Managed_AutoScaleSchema, { ...autoScale, cpuUtilizationPercent: v }) })} />
              </div>
            </div>
          );
        })()
      ) : null}

      <KeyValueEditor label="Parameters" value={value.parameters} onChange={(parameters) => patch({ parameters })} />
    </div>
  );
}
