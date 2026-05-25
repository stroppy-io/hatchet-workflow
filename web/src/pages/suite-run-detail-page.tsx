import { useState } from "react";
import { ArrowLeft, ExternalLink, XCircle } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";

import { useConfirm } from "@/components/confirm-dialog";
import { Panel } from "@/components/panel";
import { StateBlock } from "@/components/state-block";
import { StatusBadge } from "@/components/status-badge";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useListQuery } from "@/hooks/use-list-query";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatId, formatTimestamp } from "@/lib/format";
import { idMessage, tenantIdMessage } from "@/lib/proto";
import { tenantPath } from "@/lib/routes";
import { notifyError, notifySuccess } from "@/lib/toast";

export function SuiteRunDetailPage() {
  const tenantId = useTenantId();
  const { suiteRunId = "" } = useParams();
  const navigate = useNavigate();
  const confirm = useConfirm();
  const [reloadKey, setReloadKey] = useState(0);

  const result = useListQuery(() => api.suite.getSuiteRun({ tenantId: tenantIdMessage(tenantId), suiteRunId: idMessage(suiteRunId) }), [tenantId, suiteRunId, reloadKey]);
  const suiteRun = result.data;

  async function cancel() {
    const ok = await confirm({ title: "Cancel suite run?", danger: true, confirmLabel: "Cancel run" });
    if (!ok) return;
    try {
      await api.suite.cancelSuiteRun({ tenantId: tenantIdMessage(tenantId), suiteRunId: idMessage(suiteRunId) });
      notifySuccess("Suite run cancelling");
      setReloadKey((key) => key + 1);
    } catch (error) {
      notifyError(error, "Could not cancel suite run");
    }
  }

  return (
    <section className="flex min-h-0 flex-col gap-5 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon-sm" onClick={() => navigate(tenantPath(tenantId, "/suite-runs"))}>
            <ArrowLeft />
          </Button>
          <div>
            <h1 className="text-xl font-semibold tracking-normal">Suite run</h1>
            <p className="mt-1 font-mono text-xs text-muted-foreground">{suiteRunId}</p>
          </div>
        </div>
        <Button variant="outline" size="sm" onClick={cancel}>
          <XCircle />
          Cancel
        </Button>
      </div>

      <StateBlock loading={result.loading && !suiteRun} error={result.error} empty={!result.loading && !suiteRun} emptyMessage="Suite run not found.">
        {suiteRun ? (
          <div className="space-y-4">
            <Panel title="Summary">
              <dl className="grid gap-3 sm:grid-cols-3">
                <Fact label="Suite" value={<button className="font-mono text-xs text-primary hover:underline" onClick={() => navigate(tenantPath(tenantId, `/suites/${suiteRun.suiteId?.value ?? ""}`))}>{formatId(suiteRun.suiteId?.value)}</button>} />
                <Fact label="DAG" value={<span className="font-mono text-xs">{formatId(suiteRun.dag?.value)}</span>} />
                <Fact label="Created" value={formatTimestamp(suiteRun.entity?.timestamps?.createdAt)} />
              </dl>
            </Panel>

            <Panel title={`Test runs (${suiteRun.testRuns.length})`}>
              <div className="overflow-hidden rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Created</TableHead>
                      <TableHead className="w-10" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {suiteRun.testRuns.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={4} className="h-20 text-center text-muted-foreground">No test runs</TableCell>
                      </TableRow>
                    ) : (
                      suiteRun.testRuns.map((testRun) => (
                        <TableRow key={testRun.entity?.id?.value ?? crypto.randomUUID()}>
                          <TableCell>{testRun.name || formatId(testRun.entity?.id?.value)}</TableCell>
                          <TableCell><StatusBadge status={testRun.status} /></TableCell>
                          <TableCell>{formatTimestamp(testRun.entity?.timestamps?.createdAt)}</TableCell>
                          <TableCell>
                            <Button variant="ghost" size="icon-sm" onClick={() => navigate(tenantPath(tenantId, `/runs/${testRun.entity?.id?.value ?? ""}`))}>
                              <ExternalLink className="size-3.5" />
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </div>
            </Panel>
          </div>
        ) : null}
      </StateBlock>
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
