import { useMemo, useState } from "react";
import { Plus } from "lucide-react";

import { useConfirm } from "@/components/confirm-dialog";
import { EntityDataTable, type EntityColumnDef } from "@/components/data-table/entity-data-table";
import { DataBadge, FilterSelect } from "@/components/data-table/table-controls";
import { FormDialog } from "@/components/form-dialog";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { useAction } from "@/hooks/use-action";
import { useListQuery } from "@/hooks/use-list-query";
import { useTableState } from "@/hooks/use-table-state";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatBool, formatTimestamp, webhookEventLabel } from "@/lib/format";
import { idMessage, tenantIdMessage } from "@/lib/proto";
import { notifyError, notifySuccess } from "@/lib/toast";
import { ListWebhooksRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/webhook_pb.ts";
import type { Webhook } from "@/lib/proto/cloud/v1/models/webhook_pb.ts";
import { Webhook_Event } from "@/lib/proto/cloud/v1/models/webhook_pb.ts";

const EVENT_OPTIONS = [
  Webhook_Event.RUN_COMPLETED,
  Webhook_Event.RUN_FAILED,
  Webhook_Event.SUITE_COMPLETED,
  Webhook_Event.SUITE_FAILED,
];

export function WebhooksPage() {
  const tenantId = useTenantId();
  const confirm = useConfirm();
  const [enabledFilter, setEnabledFilter] = useState("all");
  const table = useTableState(ListWebhooksRequest_SortField.CREATED_AT, [tenantId, enabledFilter]);

  const result = useListQuery(
    () =>
      api.webhook.listWebhooks({
        tenantId: tenantIdMessage(tenantId),
        search: table.debouncedSearch || undefined,
        enabled: enabledFilter === "all" ? undefined : enabledFilter === "true",
        sortField: table.sortField,
        order: table.order,
        page: table.pagination.page,
      }),
    [tenantId, enabledFilter, table.debouncedSearch, table.sortField, table.order, table.pagination.page, table.reloadKey],
  );

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Webhook | null>(null);
  const [url, setUrl] = useState("");
  const [events, setEvents] = useState<Webhook_Event[]>([]);
  const [enabled, setEnabled] = useState(true);
  const [secret, setSecret] = useState("");

  const save = useAction(async () => {
    const webhook = editTarget
      ? { ...editTarget, url, events, enabled }
      : { url, events, enabled };
    return editTarget
      ? api.webhook.updateWebhook({ tenantId: tenantIdMessage(tenantId), webhook, secret })
      : api.webhook.createWebhook({ tenantId: tenantIdMessage(tenantId), webhook, secret });
  });

  function openCreate() {
    setEditTarget(null);
    setUrl("");
    setEvents([]);
    setEnabled(true);
    setSecret("");
    save.reset();
    setDialogOpen(true);
  }

  function openEdit(webhook: Webhook) {
    setEditTarget(webhook);
    setUrl(webhook.url);
    setEvents([...webhook.events]);
    setEnabled(webhook.enabled);
    setSecret("");
    save.reset();
    setDialogOpen(true);
  }

  function toggleEvent(event: Webhook_Event) {
    setEvents((current) => (current.includes(event) ? current.filter((value) => value !== event) : [...current, event]));
  }

  async function submit() {
    const response = await save.run();
    if (!response) return;
    setDialogOpen(false);
    notifySuccess(editTarget ? "Webhook updated" : "Webhook created");
    table.reload();
  }

  async function test(webhook: Webhook) {
    try {
      await api.webhook.testWebhook({ tenantId: tenantIdMessage(tenantId), id: idMessage(webhook.entity?.id?.value) });
      notifySuccess("Test payload sent");
    } catch (error) {
      notifyError(error, "Test failed");
    }
  }

  async function remove(webhook: Webhook) {
    const ok = await confirm({ title: "Delete webhook?", description: webhook.url, danger: true });
    if (!ok) return;
    try {
      await api.webhook.deleteWebhook({ tenantId: tenantIdMessage(tenantId), id: idMessage(webhook.entity?.id?.value) });
      notifySuccess("Webhook deleted");
      table.reload();
    } catch (error) {
      notifyError(error, "Could not delete webhook");
    }
  }

  const columns = useMemo<EntityColumnDef<Webhook>[]>(
    () => [
      { accessorKey: "url", id: "url", header: "URL", sortField: ListWebhooksRequest_SortField.URL },
      { id: "enabled", header: "Enabled", cell: ({ row }) => <DataBadge>{formatBool(row.original.enabled)}</DataBadge> },
      { id: "events", header: "Events", cell: ({ row }) => row.original.events.map(webhookEventLabel).join(", ") || "—" },
      { id: "created", header: "Created", sortField: ListWebhooksRequest_SortField.CREATED_AT, cell: ({ row }) => formatTimestamp(row.original.entity?.timestamps?.createdAt) },
    ],
    [],
  );

  return (
    <>
      <EntityDataTable
        actions={[
          { label: "Edit", onSelect: openEdit },
          { label: "Send test", onSelect: test },
          { label: "Delete", onSelect: remove },
        ]}
        columns={columns}
        data={result.data?.webhooks ?? []}
        error={result.error}
        filters={<FilterSelect onChange={setEnabledFilter} options={[{ label: "All states", value: "all" }, { label: "Enabled", value: "true" }, { label: "Disabled", value: "false" }]} value={enabledFilter} />}
        getRowId={(row) => row.entity?.id?.value ?? row.url}
        headerActions={
          <Button size="sm" onClick={openCreate}>
            <Plus />
            New webhook
          </Button>
        }
        loading={result.loading}
        onNextPage={() => table.pagination.next(result.data?.pageInfo)}
        onPageSizeChange={table.pagination.setPageSize}
        onPreviousPage={table.pagination.previous}
        onSearchChange={table.setSearch}
        onSortChange={table.setSort}
        pageIndex={table.pagination.index}
        pageInfo={result.data?.pageInfo}
        pageSize={table.pagination.pageSize}
        search={table.search}
        searchPlaceholder="Search webhooks"
        title="Webhooks"
      />

      <FormDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        title={editTarget ? "Edit webhook" : "New webhook"}
        submitLabel={editTarget ? "Save" : "Create"}
        onSubmit={submit}
        loading={save.loading}
        error={save.error}
        submitDisabled={!url.trim() || events.length === 0}
      >
        <div className="space-y-2">
          <Label htmlFor="wh-url">URL</Label>
          <Input id="wh-url" type="url" value={url} onChange={(event) => setUrl(event.target.value)} placeholder="https://example.com/hook" autoFocus />
        </div>
        <div className="space-y-2">
          <Label>Events</Label>
          <div className="grid grid-cols-2 gap-2">
            {EVENT_OPTIONS.map((event) => (
              <label key={event} className="flex cursor-pointer items-center gap-2 text-sm">
                <Checkbox checked={events.includes(event)} onCheckedChange={() => toggleEvent(event)} />
                {webhookEventLabel(event)}
              </label>
            ))}
          </div>
        </div>
        <div className="flex items-center justify-between">
          <Label htmlFor="wh-enabled">Enabled</Label>
          <Switch id="wh-enabled" checked={enabled} onCheckedChange={setEnabled} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="wh-secret">Signing secret {editTarget ? "(leave blank to keep current)" : "(optional)"}</Label>
          <Input id="wh-secret" type="password" value={secret} onChange={(event) => setSecret(event.target.value)} autoComplete="off" />
        </div>
      </FormDialog>
    </>
  );
}
