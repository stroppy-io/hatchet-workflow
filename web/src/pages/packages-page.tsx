import { useMemo, useRef, useState } from "react";
import { Upload } from "lucide-react";

import { useConfirm } from "@/components/confirm-dialog";
import { EntityDataTable, type EntityColumnDef } from "@/components/data-table/entity-data-table";
import { DataBadge, FilterSelect } from "@/components/data-table/table-controls";
import { FormDialog } from "@/components/form-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAction } from "@/hooks/use-action";
import { useListQuery } from "@/hooks/use-list-query";
import { useTableState } from "@/hooks/use-table-state";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatId } from "@/lib/format";
import { idMessage, tenantIdMessage } from "@/lib/proto";
import { notifyError, notifyInfo, notifySuccess } from "@/lib/toast";
import { ListPackagesRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/package_pb.ts";
import type { Package } from "@/lib/proto/cloud/v1/models/package_pb.ts";

export function PackagesPage() {
  const tenantId = useTenantId();
  const confirm = useConfirm();
  const [builtin, setBuiltin] = useState("all");
  const table = useTableState(ListPackagesRequest_SortField.CREATED_AT, [tenantId, builtin]);

  const result = useListQuery(
    () =>
      api.package.listPackages({
        tenantId: tenantIdMessage(tenantId),
        search: table.debouncedSearch || undefined,
        isBuiltin: builtin === "all" ? undefined : builtin === "true",
        sortField: table.sortField,
        order: table.order,
        page: table.pagination.page,
      }),
    [tenantId, builtin, table.debouncedSearch, table.sortField, table.order, table.pagination.page, table.reloadKey],
  );

  const [uploadOpen, setUploadOpen] = useState(false);
  const [name, setName] = useState("");
  const [dbKind, setDbKind] = useState("");
  const [dbVersion, setDbVersion] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Two-step upload: the API mints a record + presigned PUT URL, then the
  // browser uploads the .deb bytes straight to object storage (G2).
  const upload = useAction(async () => {
    if (!file) throw new Error("Choose a .deb file to upload.");
    const { package: created, uploadUrl } = await api.package.requestPackageUpload({
      tenantId: tenantIdMessage(tenantId),
      name: name.trim(),
      dbKind: dbKind.trim(),
      dbVersion: dbVersion.trim(),
      debFilename: file.name,
    });
    const put = await fetch(uploadUrl, { method: "PUT", body: file });
    if (!put.ok) throw new Error(`Upload failed (${put.status}). The package record was created but has no file.`);
    return created;
  });

  function openUpload() {
    setName("");
    setDbKind("");
    setDbVersion("");
    setFile(null);
    if (fileInputRef.current) fileInputRef.current.value = "";
    upload.reset();
    setUploadOpen(true);
  }

  async function submitUpload() {
    const created = await upload.run();
    if (!created) return;
    setUploadOpen(false);
    notifySuccess("Package uploaded");
    table.reload();
  }

  async function remove(pkg: Package) {
    if (pkg.isBuiltin) {
      notifyInfo("Builtin packages cannot be deleted");
      return;
    }
    const ok = await confirm({ title: `Delete package "${pkg.name}"?`, danger: true });
    if (!ok) return;
    try {
      await api.package.deletePackage({ tenantId: tenantIdMessage(tenantId), id: idMessage(pkg.entity?.id?.value) });
      notifySuccess("Package deleted");
      table.reload();
    } catch (error) {
      notifyError(error, "Could not delete package");
    }
  }

  const columns = useMemo<EntityColumnDef<Package>[]>(
    () => [
      { accessorKey: "name", id: "name", header: "Name", sortField: ListPackagesRequest_SortField.NAME },
      { accessorKey: "dbKind", id: "dbKind", header: "DB kind", sortField: ListPackagesRequest_SortField.DB_KIND },
      { accessorKey: "dbVersion", id: "dbVersion", header: "Version" },
      { id: "origin", header: "Origin", cell: ({ row }) => <DataBadge>{row.original.isBuiltin ? "Builtin" : "Custom"}</DataBadge> },
      { id: "checksum", header: "Checksum", cell: ({ row }) => <span className="font-mono text-xs">{formatId(row.original.checksum)}</span> },
    ],
    [],
  );

  return (
    <>
      <EntityDataTable
        actions={[{ label: "Delete", onSelect: remove }]}
        columns={columns}
        data={result.data?.packages ?? []}
        error={result.error}
        filters={<FilterSelect onChange={setBuiltin} options={[{ label: "All packages", value: "all" }, { label: "Builtin", value: "true" }, { label: "Custom", value: "false" }]} value={builtin} />}
        getRowId={(row) => row.entity?.id?.value ?? row.name}
        headerActions={
          <Button size="sm" onClick={openUpload}>
            <Upload />
            Upload package
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
        searchPlaceholder="Search packages"
        title="Packages"
      />

      <FormDialog
        open={uploadOpen}
        onOpenChange={setUploadOpen}
        title="Upload package"
        description="Upload a custom .deb. Bytes go straight to object storage."
        submitLabel="Upload"
        onSubmit={submitUpload}
        loading={upload.loading}
        error={upload.error}
        submitDisabled={!name.trim() || !dbKind.trim() || !file}
      >
        <div className="space-y-2">
          <Label htmlFor="pkg-name">Name</Label>
          <Input id="pkg-name" value={name} onChange={(event) => setName(event.target.value)} placeholder="postgresql-16" autoFocus />
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-2">
            <Label htmlFor="pkg-kind">DB kind</Label>
            <Input id="pkg-kind" value={dbKind} onChange={(event) => setDbKind(event.target.value)} placeholder="postgresql" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="pkg-version">DB version</Label>
            <Input id="pkg-version" value={dbVersion} onChange={(event) => setDbVersion(event.target.value)} placeholder="16.2" />
          </div>
        </div>
        <div className="space-y-2">
          <Label htmlFor="pkg-file">.deb file</Label>
          <Input
            id="pkg-file"
            ref={fileInputRef}
            type="file"
            accept=".deb"
            onChange={(event) => setFile(event.target.files?.[0] ?? null)}
          />
        </div>
      </FormDialog>
    </>
  );
}
