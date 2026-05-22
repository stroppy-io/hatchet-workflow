import { useMemo, useState } from "react";

import { DiagnosticsList } from "@/components/diagnostics-list";
import { JsonPanel } from "@/components/json-panel";
import { Panel } from "@/components/panel";
import { useTenantId } from "@/hooks/use-tenant-id";
import type { Diagnostic, TestPresetAssembly } from "@/lib/proto/cloud/v1/api/ui/authoring_pb.ts";
import { Provider } from "@/lib/proto/cloud/v1/deployment/deployment_pb.ts";
import type { Dag } from "@/lib/proto/cloud/v1/runtime/primitive/dag_pb.ts";

const steps = [
  "Presets",
  "Database",
  "Workload",
  "Topology",
  "Deployment",
  "Review",
] as const;

export function WizardPage() {
  const tenantId = useTenantId();
  const [step, setStep] = useState(0);
  const [provider, setProvider] = useState<Provider>(Provider.DOCKER);
  const [assembly] = useState<TestPresetAssembly | null>(null);
  const [dagPreview] = useState<Dag | null>(null);

  const diagnostics = useMemo<Diagnostic[]>(
    () => assembly?.diagnostics ?? [],
    [assembly],
  );

  return (
    <div className="flex h-[calc(100vh-3.5rem)] flex-col overflow-hidden">
      <div className="border-b bg-[#070708] px-5 py-3">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h1 className="text-base font-semibold">TestPreset Wizard</h1>
            <p className="mt-1 text-xs text-muted-foreground">
              Tenant-scoped authoring flow for DatabasePreset + WorkloadPreset into an executable TestPreset.
            </p>
          </div>
          <select
            className="border bg-background px-3 py-2 text-sm"
            value={provider}
            onChange={(event) => setProvider(Number(event.target.value) as Provider)}
          >
            <option value={Provider.DOCKER}>Docker</option>
            <option value={Provider.YANDEX}>Yandex Cloud</option>
          </select>
        </div>
      </div>

      <div className="flex min-h-0 flex-1">
        <aside className="w-64 shrink-0 border-r bg-[#070708] p-4">
          <div className="mb-4 font-mono text-[11px] text-muted-foreground">/t/{tenantId}/wizard</div>
          <nav className="space-y-1">
            {steps.map((label, index) => (
              <button
                key={label}
                type="button"
                onClick={() => setStep(index)}
                className={[
                  "flex w-full items-center justify-between px-3 py-2 text-left text-sm",
                  step === index ? "bg-muted text-foreground" : "text-muted-foreground hover:text-foreground",
                ].join(" ")}
              >
                <span>{label}</span>
                <span className="font-mono text-[10px]">{index + 1}</span>
              </button>
            ))}
          </nav>
        </aside>

        <section className="min-w-0 flex-1 overflow-auto p-5">
          {step === 0 ? <PresetsStep /> : null}
          {step === 1 ? <DatabaseStep /> : null}
          {step === 2 ? <WorkloadStep /> : null}
          {step === 3 ? <TopologyStep /> : null}
          {step === 4 ? <DeploymentStep provider={provider} /> : null}
          {step === 5 ? (
            <ReviewStep assembly={assembly} dagPreview={dagPreview} diagnostics={diagnostics} />
          ) : null}
        </section>
      </div>
    </div>
  );
}

function PresetsStep() {
  return (
    <div className="grid gap-4 xl:grid-cols-2">
      <Panel title="Database Preset" description="Pick a database intent and topology fragment from the tenant catalog.">
        <EmptyState label="Database preset picker will call PresetService.ListPresets(kind=DATABASE)." />
      </Panel>
      <Panel title="Workload Preset" description="Pick a workload intent and topology fragment from the tenant catalog.">
        <EmptyState label="Workload preset picker will call PresetService.ListPresets(kind=WORKLOAD)." />
      </Panel>
      <Panel title="Assembly" description="The backend copies both presets by value and returns the first TestPreset draft.">
        <EmptyState label="AuthoringService.AssembleFromPresets result appears here." />
      </Panel>
    </div>
  );
}

function DatabaseStep() {
  return (
    <Panel title="Database Intent" description="Structured editor for domain.Database with a raw proto escape hatch.">
      <EmptyState label="Database form and render preview go here." />
    </Panel>
  );
}

function WorkloadStep() {
  return (
    <Panel title="Workload Intent" description="Structured editor for domain.Workload with probe and stroppy config preview.">
      <EmptyState label="Workload form, probe output and generated stroppy config go here." />
    </Panel>
  );
}

function TopologyStep() {
  return (
    <Panel title="Merged Topology" description="Graph editor for machines, components and connections.">
      <EmptyState label="Topology graph and provenance badges go here." />
    </Panel>
  );
}

function DeploymentStep({ provider }: { provider: Provider }) {
  return (
    <Panel title="Deployment Intent" description="Reasonable defaults first; provider-specific edits only when opened.">
      <EmptyState label={`Materialized deployment preview for ${provider === Provider.YANDEX ? "Yandex Cloud" : "Docker"} goes here.`} />
    </Panel>
  );
}

function ReviewStep({
  assembly,
  dagPreview,
  diagnostics,
}: {
  assembly: TestPresetAssembly | null;
  dagPreview: Dag | null;
  diagnostics: Diagnostic[];
}) {
  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_24rem]">
      <div className="space-y-4">
        <Panel title="TestPreset Snapshot" description="The exact proto snapshot that SubmitTestRun will send.">
          <JsonPanel value={assembly?.testPreset} />
        </Panel>
        <Panel title="DAG Blueprint" description="Compiled execution graph preview. No run state, no started tasks.">
          <JsonPanel value={dagPreview} />
        </Panel>
      </div>
      <Panel title="Diagnostics">
        <DiagnosticsList diagnostics={diagnostics} />
      </Panel>
    </div>
  );
}

function EmptyState({ label }: { label: string }) {
  return <div className="border border-dashed p-6 text-sm text-muted-foreground">{label}</div>;
}
