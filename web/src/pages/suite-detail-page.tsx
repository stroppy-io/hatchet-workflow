import { useState } from "react";
import { create } from "@bufbuild/protobuf";
import { ArrowLeft, Pencil, Play } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";

import { FormDialog } from "@/components/form-dialog";
import { Panel } from "@/components/panel";
import { StateBlock } from "@/components/state-block";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAction } from "@/hooks/use-action";
import { useListQuery } from "@/hooks/use-list-query";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatId, formatTimestamp, providerLabel } from "@/lib/format";
import { testPresetSummary } from "@/lib/preset-summary";
import { idMessage, tenantIdMessage } from "@/lib/proto";
import { tenantPath } from "@/lib/routes";
import { notifySuccess } from "@/lib/toast";
import { SuiteSchema } from "@/lib/proto/cloud/v1/models/testing_pb.ts";

export function SuiteDetailPage() {
  const tenantId = useTenantId();
  const { suiteId = "" } = useParams();
  const navigate = useNavigate();

  const [reloadKey, setReloadKey] = useState(0);
  const result = useListQuery(() => api.suite.getSuite({ tenantId: tenantIdMessage(tenantId), suiteId: idMessage(suiteId) }), [tenantId, suiteId, reloadKey]);
  const suite = result.data;
  const preset = suite?.preset;

  const launch = useAction(() => api.suite.launchSuiteRun({ tenantId: tenantIdMessage(tenantId), suiteId: idMessage(suiteId) }));

  // Edit (name + description) via UpdateSuite + FieldMask.
  const [editOpen, setEditOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const save = useAction(() =>
    api.suite.updateSuite({
      tenantId: tenantIdMessage(tenantId),
      suite: create(SuiteSchema, { ...suite, name, description }),
      updateMask: { paths: ["name", "description"] },
    }),
  );

  async function doLaunch() {
    const response = await launch.run();
    if (!response) return;
    notifySuccess("Suite run launched");
    navigate(tenantPath(tenantId, `/suite-runs/${response.entity?.id?.value ?? ""}`));
  }

  function openEdit() {
    setName(suite?.name ?? "");
    setDescription(suite?.description ?? "");
    save.reset();
    setEditOpen(true);
  }

  async function submitEdit() {
    const response = await save.run();
    if (!response) return;
    setEditOpen(false);
    notifySuccess("Suite updated");
    setReloadKey((key) => key + 1);
  }

  const scheduling = preset?.scheduling?.mode.case === "parallel" ? `Parallel ×${preset.scheduling.mode.value.maxParallel}` : preset?.scheduling?.mode.case === "sequential" ? "Sequential" : "—";

  return (
    <section className="flex min-h-0 flex-col gap-5 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon-sm" onClick={() => navigate(tenantPath(tenantId, "/suites"))}>
            <ArrowLeft />
          </Button>
          <div>
            <h1 className="text-xl font-semibold tracking-normal">{suite?.name || formatId(suiteId)}</h1>
            <p className="mt-1 font-mono text-xs text-muted-foreground">{suiteId}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={openEdit} disabled={!suite}>
            <Pencil />
            Edit
          </Button>
          <Button size="sm" onClick={doLaunch} disabled={launch.loading}>
            <Play />
            {launch.loading ? "Launching…" : "Launch"}
          </Button>
        </div>
      </div>

      <StateBlock loading={result.loading && !suite} error={result.error} empty={!result.loading && !suite} emptyMessage="Suite not found.">
        {suite && preset ? (
          <div className="space-y-4">
            <Panel title="Configuration">
              <dl className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                <Fact label="Provider" value={providerLabel(preset.provider)} />
                <Fact label="Scheduling" value={scheduling} />
                <Fact label="Tests" value={String(preset.tests.length)} />
                <Fact label="Next fire" value={suite.nextFireAt ? formatTimestamp(suite.nextFireAt) : "—"} />
                {suite.cron ? <Fact label="Cron" value={<span className="font-mono text-xs">{suite.cron.expr} ({suite.cron.timezone})</span>} /> : null}
              </dl>
              {suite.description ? <p className="mt-3 text-sm text-muted-foreground">{suite.description}</p> : null}
            </Panel>

            <Panel title={`Tests (${preset.tests.length})`} action={<Button size="sm" variant="outline" onClick={() => navigate(tenantPath(tenantId, "/suite-runs"))}>View runs</Button>}>
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {preset.tests.map((test, index) => {
                  const summary = testPresetSummary(test);
                  return (
                    <div key={index} className="rounded-md border bg-card p-4">
                      <div className="text-sm font-semibold">{summary.database}</div>
                      <dl className="mt-2 space-y-1 text-sm">
                        <Row label="Workload" value={summary.workload} />
                        <Row label="Protocol" value={summary.protocol} />
                        <Row label="Topology" value={summary.topology} />
                      </dl>
                    </div>
                  );
                })}
              </div>
            </Panel>
          </div>
        ) : null}
      </StateBlock>

      <FormDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        title="Edit suite"
        submitLabel="Save"
        onSubmit={submitEdit}
        loading={save.loading}
        error={save.error}
        submitDisabled={!name.trim()}
      >
        <div className="space-y-2">
          <Label>Name</Label>
          <Input value={name} onChange={(event) => setName(event.target.value)} autoFocus />
        </div>
        <div className="space-y-2">
          <Label>Description</Label>
          <Input value={description} onChange={(event) => setDescription(event.target.value)} />
        </div>
      </FormDialog>
    </section>
  );
}

function Fact({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="mt-0.5 text-sm">{value}</dd>
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[5rem_minmax(0,1fr)] gap-2">
      <span className="text-muted-foreground">{label}</span>
      <span className="min-w-0 truncate">{value}</span>
    </div>
  );
}
