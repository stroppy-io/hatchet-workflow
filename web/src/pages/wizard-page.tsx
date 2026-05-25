import { useMemo, useState } from "react";
import { create, toJsonString } from "@bufbuild/protobuf";
import { useNavigate } from "react-router-dom";

import { DatabaseEditor } from "@/components/editors/database-editor";
import { TopologyEditor } from "@/components/editors/topology-editor";
import { WorkloadEditor } from "@/components/editors/workload-editor";
import { DiagnosticsList } from "@/components/diagnostics-list";
import { FormDialog } from "@/components/form-dialog";
import { JsonPanel } from "@/components/json-panel";
import { Panel } from "@/components/panel";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { JsonEditor } from "@/components/ui/json-editor";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useAction } from "@/hooks/use-action";
import { useListQuery } from "@/hooks/use-list-query";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { presetTitle } from "@/lib/preset-summary";
import { tenantIdMessage } from "@/lib/proto";
import { tenantPath } from "@/lib/routes";
import { notifySuccess } from "@/lib/toast";
import { Severity, type Diagnostic } from "@/lib/proto/cloud/v1/api/ui/authoring_pb.ts";
import { DatabasePresetSchema, DatabaseSchema } from "@/lib/proto/cloud/v1/domain/database_pb.ts";
import { TestPresetSchema, type TestPreset } from "@/lib/proto/cloud/v1/domain/test_pb.ts";
import { TopologySchema } from "@/lib/proto/cloud/v1/domain/topology_pb.ts";
import { WorkloadPresetSchema, WorkloadSchema } from "@/lib/proto/cloud/v1/domain/workload_pb.ts";
import { DeploymentIntentSchema } from "@/lib/proto/cloud/v1/deployment/deployment_pb.ts";
import { Provider } from "@/lib/proto/cloud/v1/deployment/deployment_pb.ts";
import { Preset_Kind, PresetSchema, type Preset } from "@/lib/proto/cloud/v1/models/preset_pb.ts";
import type { Config } from "@/lib/proto/cloud/v1/runtime/render/config_pb.ts";
import type { Dag } from "@/lib/proto/cloud/v1/runtime/primitive/dag_pb.ts";

const STEPS = ["Presets", "Database", "Workload", "Topology", "Deployment", "Review"] as const;

export function WizardPage() {
  const tenantId = useTenantId();
  const navigate = useNavigate();
  const [step, setStep] = useState(0);
  const [provider, setProvider] = useState<Provider>(Provider.DOCKER);
  const [dbPreset, setDbPreset] = useState<Preset | null>(null);
  const [wlPreset, setWlPreset] = useState<Preset | null>(null);
  const [draft, setDraft] = useState<TestPreset | null>(null);
  const [diagnostics, setDiagnostics] = useState<Diagnostic[]>([]);

  // Step-specific preview state.
  const [renderConfigs, setRenderConfigs] = useState<Config[] | null>(null);
  const [stroppyConfig, setStroppyConfig] = useState<string | null>(null);
  const [dagPreview, setDagPreview] = useState<Dag | null>(null);
  const [saveOpen, setSaveOpen] = useState(false);
  const [saveName, setSaveName] = useState("");
  const [saveKind, setSaveKind] = useState(String(Preset_Kind.TEST));

  const hasErrors = diagnostics.some((d) => d.severity === Severity.ERROR);
  const ref = () => ({ tenantId: tenantIdMessage(tenantId), testPreset: draft ?? create(TestPresetSchema, {}) });
  const patchDraft = (partial: Partial<TestPreset>) => setDraft((current) => create(TestPresetSchema, { ...(current ?? {}), ...partial }));

  const assembleFrom = useAction(() =>
    api.authoring.assembleFromPresets({
      tenantId: tenantIdMessage(tenantId),
      databasePreset: dbPreset?.preset.case === "databasePreset" ? dbPreset.preset.value : create(DatabasePresetSchema, {}),
      workloadPreset: wlPreset?.preset.case === "workloadPreset" ? wlPreset.preset.value : create(WorkloadPresetSchema, {}),
      provider,
    }),
  );
  const validate = useAction(() => api.authoring.validateTestPreset(ref()));
  const previewRender = useAction(() => api.authoring.previewDatabaseRender(ref()));
  const probe = useAction(() => api.authoring.probeWorkload(ref()));
  const previewWorkload = useAction(() => api.authoring.previewWorkloadConfig(ref()));
  const materialize = useAction(() => api.authoring.materializeDeploymentIntent({ testPreset: draft ?? create(TestPresetSchema, {}), provider }));
  const checkDeployment = useAction(() => api.authoring.checkDeployment(ref()));
  const compile = useAction(() => api.authoring.compileTestPresetPreview(ref()));
  const submit = useAction(() => api.run.submitTestRun({ tenantId: tenantIdMessage(tenantId), testPreset: draft ?? create(TestPresetSchema, {}) }));
  const savePreset = useAction(() => api.preset.createPreset(buildSavePreset()));

  function buildSavePreset(): Preset {
    const kind = Number(saveKind) as Preset_Kind;
    const owned = { tenantId: tenantIdMessage(tenantId) };
    const current = draft ?? create(TestPresetSchema, {});
    if (kind === Preset_Kind.DATABASE) {
      return create(PresetSchema, { kind, name: saveName, owned, preset: { case: "databasePreset", value: create(DatabasePresetSchema, { database: current.database, topology: current.topology }) } });
    }
    if (kind === Preset_Kind.WORKLOAD) {
      return create(PresetSchema, { kind, name: saveName, owned, preset: { case: "workloadPreset", value: create(WorkloadPresetSchema, { workload: current.workload, topology: current.topology }) } });
    }
    return create(PresetSchema, { kind: Preset_Kind.TEST, name: saveName, owned, preset: { case: "testPreset", value: current } });
  }

  async function runAssemble() {
    const response = await assembleFrom.run();
    if (!response) return;
    setDraft(response.testPreset ?? null);
    setDiagnostics(response.diagnostics);
    setStep(1);
  }

  async function runValidate() {
    const response = await validate.run();
    if (response) setDiagnostics(response.diagnostics);
  }

  async function runProbe() {
    const response = await probe.run();
    if (response) setDiagnostics(response.diagnostics);
  }

  async function runCheck() {
    const response = await checkDeployment.run();
    if (response) setDiagnostics(response.diagnostics);
  }

  async function runSubmit() {
    const response = await submit.run();
    if (!response) return;
    notifySuccess("Run submitted");
    navigate(tenantPath(tenantId, `/runs/${response.entity?.id?.value ?? ""}`));
  }

  async function runSave() {
    const response = await savePreset.run();
    if (!response) return;
    setSaveOpen(false);
    notifySuccess("Preset saved");
  }

  return (
    <div className="flex h-[calc(100vh-0px)] flex-col overflow-hidden">
      <div className="border-b bg-[#070708] px-5 py-3">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h1 className="text-base font-semibold">TestPreset Wizard</h1>
            <p className="mt-1 text-xs text-muted-foreground">Define → Render → Plan for tenant {tenantId}. Draft assembled by value; nothing persisted until you submit or save.</p>
          </div>
          <Select value={String(provider)} onValueChange={(v) => setProvider(Number(v) as Provider)}>
            <SelectTrigger className="w-44">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={String(Provider.DOCKER)}>Docker</SelectItem>
              <SelectItem value={String(Provider.YANDEX)}>Yandex Cloud</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="flex min-h-0 flex-1">
        <aside className="w-60 shrink-0 border-r bg-[#070708] p-3">
          <nav className="space-y-1">
            {STEPS.map((label, index) => {
              const disabled = index > 0 && !draft;
              return (
                <button
                  key={label}
                  type="button"
                  disabled={disabled}
                  onClick={() => setStep(index)}
                  className={[
                    "flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm",
                    step === index ? "bg-muted text-foreground" : "text-muted-foreground hover:bg-muted/40 hover:text-foreground",
                    disabled ? "cursor-not-allowed opacity-40" : "",
                  ].join(" ")}
                >
                  <span>{label}</span>
                  <span className="font-mono text-[10px]">{index + 1}</span>
                </button>
              );
            })}
          </nav>
        </aside>

        <section className="min-w-0 flex-1 overflow-auto p-5">
          {step === 0 ? (
            <Panel title="Choose building blocks" description="Pick a DatabasePreset and a WorkloadPreset; the backend copies them by value into a TestPreset draft.">
              <div className="grid gap-4 xl:grid-cols-2">
                <div className="space-y-1.5">
                  <Label>Database preset</Label>
                  <PresetSelect tenantId={tenantId} kind={Preset_Kind.DATABASE} value={dbPreset?.entity?.id?.value ?? ""} onSelect={setDbPreset} placeholder="Select database preset" />
                </div>
                <div className="space-y-1.5">
                  <Label>Workload preset</Label>
                  <PresetSelect tenantId={tenantId} kind={Preset_Kind.WORKLOAD} value={wlPreset?.entity?.id?.value ?? ""} onSelect={setWlPreset} placeholder="Select workload preset" />
                </div>
              </div>
              <div className="mt-4 flex items-center gap-3">
                <Button onClick={runAssemble} disabled={!dbPreset || !wlPreset || assembleFrom.loading}>
                  {assembleFrom.loading ? "Assembling…" : "Assemble draft"}
                </Button>
                {assembleFrom.error ? <span className="text-sm text-destructive">{assembleFrom.error}</span> : null}
              </div>
            </Panel>
          ) : null}

          {step === 1 && draft ? (
            <StepShell title="Database" actions={[{ label: validate.loading ? "Validating…" : "Validate", onClick: runValidate }, { label: previewRender.loading ? "Rendering…" : "Preview render", onClick: async () => { const r = await previewRender.run(); if (r) setRenderConfigs(r.configs); } }]}>
              <DatabaseEditor value={draft.database ?? create(DatabaseSchema, {})} onChange={(database) => patchDraft({ database })} />
              {renderConfigs ? <Panel title="Rendered config preview"><JsonEditor readOnly minHeight="200px" value={renderConfigs.map((c) => c.id).join("\n") || "no configs"} /></Panel> : null}
            </StepShell>
          ) : null}

          {step === 2 && draft ? (
            <StepShell title="Workload" actions={[{ label: probe.loading ? "Probing…" : "Probe", onClick: runProbe }, { label: previewWorkload.loading ? "…" : "Preview stroppy config", onClick: async () => { const r = await previewWorkload.run(); if (r) setStroppyConfig(r.stroppyConfig); } }]}>
              <WorkloadEditor value={draft.workload ?? create(WorkloadSchema, {})} onChange={(workload) => patchDraft({ workload })} />
              {stroppyConfig ? <Panel title="Generated stroppy config"><JsonEditor readOnly minHeight="200px" value={stroppyConfig} /></Panel> : null}
            </StepShell>
          ) : null}

          {step === 3 && draft ? (
            <StepShell title="Topology" actions={[{ label: "Validate topology", onClick: async () => { const r = await api.authoring.validateTopology(draft.topology ?? create(TopologySchema, {})); setDiagnostics(r.diagnostics); } }]}>
              <TopologyEditor value={draft.topology ?? create(TopologySchema, {})} onChange={(topology) => patchDraft({ topology })} />
            </StepShell>
          ) : null}

          {step === 4 && draft ? (
            <StepShell title="Deployment" actions={[{ label: materialize.loading ? "Materializing…" : "Materialize", onClick: async () => { const r = await materialize.run(); if (r) patchDraft({ deployment: r }); } }, { label: checkDeployment.loading ? "Checking…" : "Check feasibility", onClick: runCheck }]}>
              <p className="text-sm text-muted-foreground">Materialize the provider-specific DeploymentIntent from the topology, then check quota feasibility.</p>
              <Panel title="DeploymentIntent">
                <JsonEditor readOnly minHeight="240px" value={toJsonString(DeploymentIntentSchema, draft.deployment ?? create(DeploymentIntentSchema, {}), { prettySpaces: 2 })} />
              </Panel>
            </StepShell>
          ) : null}

          {step === 5 && draft ? (
            <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_22rem]">
              <div className="space-y-4">
                <Panel title="TestPreset snapshot" description="Exactly what SubmitTestRun will send." action={<Button size="sm" variant="outline" onClick={async () => { const r = await compile.run(); if (r) setDagPreview(r); }}>{compile.loading ? "Compiling…" : "Compile DAG blueprint"}</Button>}>
                  <JsonPanel value={draft} />
                </Panel>
                <Panel title="DAG blueprint" description="Compiled execution graph. No run, no state.">
                  <JsonPanel value={dagPreview} emptyLabel="Compile to preview the blueprint." />
                </Panel>
                <div className="flex items-center gap-3">
                  <Button onClick={runSubmit} disabled={hasErrors || submit.loading}>{submit.loading ? "Submitting…" : "Submit run"}</Button>
                  <Button variant="outline" onClick={() => setSaveOpen(true)}>Save as preset</Button>
                  {hasErrors ? <span className="text-sm text-destructive">Resolve ERROR diagnostics before submitting.</span> : null}
                </div>
              </div>
              <Panel title="Diagnostics">
                <DiagnosticsList diagnostics={diagnostics} />
              </Panel>
            </div>
          ) : null}

          {step !== 5 && draft ? (
            <div className="mt-6">
              <Panel title="Diagnostics">
                <DiagnosticsList diagnostics={diagnostics} />
              </Panel>
            </div>
          ) : null}
        </section>
      </div>

      <FormDialog
        open={saveOpen}
        onOpenChange={setSaveOpen}
        title="Save as preset"
        submitLabel="Save"
        onSubmit={runSave}
        loading={savePreset.loading}
        error={savePreset.error}
        submitDisabled={!saveName.trim()}
      >
        <div className="space-y-2">
          <Label>Name</Label>
          <Input value={saveName} onChange={(event) => setSaveName(event.target.value)} autoFocus />
        </div>
        <div className="space-y-2">
          <Label>Scope</Label>
          <Select value={saveKind} onValueChange={setSaveKind}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={String(Preset_Kind.TEST)}>Test preset (full)</SelectItem>
              <SelectItem value={String(Preset_Kind.DATABASE)}>Database preset</SelectItem>
              <SelectItem value={String(Preset_Kind.WORKLOAD)}>Workload preset</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </FormDialog>
    </div>
  );
}

function StepShell({ title, actions, children }: { title: string; actions: { label: string; onClick: () => void }[]; children: React.ReactNode }) {
  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-semibold">{title}</h2>
        <div className="flex items-center gap-2">
          {actions.map((action) => (
            <Button key={action.label} size="sm" variant="outline" onClick={action.onClick}>
              {action.label}
            </Button>
          ))}
        </div>
      </div>
      {children}
    </div>
  );
}

function PresetSelect({ tenantId, kind, value, onSelect, placeholder }: { tenantId: string; kind: Preset_Kind; value: string; onSelect: (preset: Preset) => void; placeholder: string }) {
  const result = useListQuery(
    () => api.preset.listPresets({ tenantId: tenantIdMessage(tenantId), kinds: [kind], page: { size: 100 } }),
    [tenantId, kind],
  );
  const presets = useMemo(() => result.data?.presets ?? [], [result.data]);

  return (
    <Select
      value={value}
      onValueChange={(id) => {
        const preset = presets.find((p) => p.entity?.id?.value === id);
        if (preset) onSelect(preset);
      }}
    >
      <SelectTrigger className="w-full">
        <SelectValue placeholder={result.loading ? "Loading…" : placeholder} />
      </SelectTrigger>
      <SelectContent>
        {presets.map((preset) => (
          <SelectItem key={preset.entity?.id?.value ?? presetTitle(preset)} value={preset.entity?.id?.value ?? ""}>
            {presetTitle(preset)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
