import { describe, it, expect, vi, beforeEach } from "vitest";
import { create } from "@bufbuild/protobuf";
import { CatalogEntrySchema } from "@/lib/proto/cloud/v1/catalog/models_pb";

const {
  listOrgProviders,
  listOrgWorkflows,
  getOrgProvider,
  createOrgProvider,
  updateOrgProvider,
  linkInstanceProvider,
  checkCatalogProvider,
  checkInstanceProvider,
  deleteOrgWorkflow,
  getInstanceEntryFiles,
  getOrgProviderFiles,
  getOrgWorkflowFiles,
} = vi.hoisted(() => ({
  listOrgProviders: vi.fn(),
  listOrgWorkflows: vi.fn(),
  getOrgProvider: vi.fn(),
  createOrgProvider: vi.fn(),
  updateOrgProvider: vi.fn(),
  linkInstanceProvider: vi.fn(),
  checkCatalogProvider: vi.fn(),
  checkInstanceProvider: vi.fn(),
  deleteOrgWorkflow: vi.fn(),
  getInstanceEntryFiles: vi.fn(),
  getOrgProviderFiles: vi.fn(),
  getOrgWorkflowFiles: vi.fn(),
}));

vi.mock("@/services/client", () => ({
  catalogClient: {
    listOrgProviders,
    listOrgWorkflows,
    getOrgProvider,
    createOrgProvider,
    updateOrgProvider,
    linkInstanceProvider,
    checkCatalogProvider,
    checkInstanceProvider,
    deleteOrgWorkflow,
    getInstanceEntryFiles,
    getOrgProviderFiles,
    getOrgWorkflowFiles,
  },
}));
vi.mock("@/services/tenant", () => ({ resolveTenantId: vi.fn().mockResolvedValue("t1") }));

import {
  checkCatalogBundle,
  checkInstanceBundle,
  createOrgEntry,
  deleteOrgEntry,
  getOrgEntry,
  getInstanceEntryFiles as getInstanceEntryFilesFn,
  getOrgEntryFiles,
  linkInstanceEntry,
  listOrgEntries,
  updateOrgEntry,
  cachedEntryFiles,
} from "./catalog";

function entry(opts: {
  id?: string;
  name?: string;
  origin?: number;
  sourceEntryId?: string;
  version?: number;
} = {}) {
  return create(CatalogEntrySchema, {
    entity: { id: opts.id ?? "e1", tenantId: "t1", name: opts.name ?? "yandex", description: "" },
    level: 2, // LEVEL_ORG
    kind: 1, // KIND_PROVIDER
    slug: "yandex",
    version: opts.version ?? 1,
    origin: opts.origin ?? 0, // ORIGIN_NATIVE
    sourceEntryId: opts.sourceEntryId ?? "",
    summary: { provides: ["machines"], compiles: true },
  });
}

beforeEach(() => {
  listOrgProviders.mockReset();
  listOrgWorkflows.mockReset();
  getOrgProvider.mockReset();
  createOrgProvider.mockReset();
  updateOrgProvider.mockReset();
  linkInstanceProvider.mockReset();
  checkCatalogProvider.mockReset();
  checkInstanceProvider.mockReset();
  deleteOrgWorkflow.mockReset();
  getInstanceEntryFiles.mockReset();
  getOrgProviderFiles.mockReset();
  getOrgWorkflowFiles.mockReset();
});

describe("listOrgEntries", () => {
  it("routes KIND_PROVIDER to listOrgProviders with the resolved tenant id", async () => {
    listOrgProviders.mockResolvedValue({ entries: [entry()] });
    const rows = await listOrgEntries("acme", "KIND_PROVIDER");
    expect(listOrgProviders).toHaveBeenCalledWith({ tenantId: "t1" });
    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({ id: "e1", slug: "yandex", kind: "KIND_PROVIDER", origin: "ORIGIN_NATIVE" });
  });

  it("routes KIND_WORKFLOW to listOrgWorkflows", async () => {
    listOrgWorkflows.mockResolvedValue({ entries: [] });
    await listOrgEntries("acme", "KIND_WORKFLOW");
    expect(listOrgWorkflows).toHaveBeenCalledWith({ tenantId: "t1" });
    expect(listOrgProviders).not.toHaveBeenCalled();
  });
});

describe("getOrgEntry", () => {
  it("throws when the RPC returns no entry", async () => {
    getOrgProvider.mockResolvedValue({ entry: undefined });
    await expect(getOrgEntry("acme", "KIND_PROVIDER", "missing")).rejects.toThrow(/not found/);
  });
});

describe("createOrgEntry / cachedEntryFiles", () => {
  it("caches the files it just sent, keyed by the returned entry id", async () => {
    createOrgProvider.mockResolvedValue({ entry: entry({ id: "e2", name: "n" }) });
    const files = { "manifest.yaml": "name: yandex\nprovides: [machines]\n" };
    const vm = await createOrgEntry("acme", "KIND_PROVIDER", {
      slug: "yandex",
      name: "Yandex",
      description: "",
      files,
    });
    expect(vm.id).toBe("e2");
    expect(cachedEntryFiles("e2")).toEqual(files);
    expect(cachedEntryFiles("does-not-exist")).toBeUndefined();
  });
});

describe("updateOrgEntry — fork-on-edit", () => {
  // Locks the client-side half of the fork-on-edit contract: the server
  // (internal/services/catalog/service.go's updateOrgEntry) forks a LINKED
  // row implicitly on any edit — origin flips to FORKED, a new entity id is
  // minted — and there is NO separate client-callable fork RPC (ForkEntry
  // exists only as an internal Go helper). The client's only job is to call
  // the same UpdateOrgProvider/UpdateOrgWorkflow RPC either way and reflect
  // whatever origin comes back; this test asserts that mapping.
  it("reflects the server's ORIGIN_FORKED response after editing a linked entry", async () => {
    const linkedId = "linked-1";
    const forkedId = "forked-2";
    updateOrgProvider.mockResolvedValue({
      entry: entry({
        id: forkedId,
        origin: 2, // ORIGIN_FORKED
        sourceEntryId: "instance-yandex",
        version: 2,
      }),
    });

    const files = { "manifest.yaml": "name: yandex\nprovides: [machines, volumes]\n" };
    const vm = await updateOrgEntry("acme", "KIND_PROVIDER", linkedId, files);

    expect(updateOrgProvider).toHaveBeenCalledWith(
      expect.objectContaining({ tenantId: "t1", id: linkedId }),
    );
    expect(vm.id).toBe(forkedId);
    expect(vm.id).not.toBe(linkedId);
    expect(vm.origin).toBe("ORIGIN_FORKED");
    expect(vm.sourceEntryId).toBe("instance-yandex");
    // The new (forked) row's files are now what's cached, not the linked row's.
    expect(cachedEntryFiles(forkedId)).toEqual(files);
    expect(cachedEntryFiles(linkedId)).toBeUndefined();
  });
});

describe("linkInstanceEntry", () => {
  it("creates a LINKED org row via LinkInstanceProvider", async () => {
    linkInstanceProvider.mockResolvedValue({
      entry: entry({ origin: 1 /* ORIGIN_LINKED */, sourceEntryId: "instance-1" }),
    });
    const vm = await linkInstanceEntry("acme", "KIND_PROVIDER", "instance-1");
    expect(linkInstanceProvider).toHaveBeenCalledWith({ tenantId: "t1", instanceEntryId: "instance-1" });
    expect(vm.origin).toBe("ORIGIN_LINKED");
    expect(vm.sourceEntryId).toBe("instance-1");
  });
});

describe("checkCatalogBundle", () => {
  it("sends encoded files and tenant id, decodes diagnostics", async () => {
    checkCatalogProvider.mockResolvedValue({
      diagnostics: [{ severity: 1, path: "manifest.yaml", line: 1, col: 1, message: "bad", module: "m" }],
    });
    const diags = await checkCatalogBundle("acme", "KIND_PROVIDER", { "manifest.yaml": "name: x\n" });
    expect(diags).toEqual([
      { severity: "error", path: "manifest.yaml", line: 1, col: 1, message: "bad", module: "m" },
    ]);
    const call = checkCatalogProvider.mock.calls[0][0];
    expect(call.tenantId).toBe("t1");
    expect(new TextDecoder().decode(call.files["manifest.yaml"])).toBe("name: x\n");
  });
});

describe("deleteOrgEntry", () => {
  it("routes KIND_WORKFLOW to deleteOrgWorkflow with the resolved tenant id", async () => {
    deleteOrgWorkflow.mockResolvedValue({});
    await deleteOrgEntry("acme", "KIND_WORKFLOW", "w1");
    expect(deleteOrgWorkflow).toHaveBeenCalledWith({ tenantId: "t1", id: "w1" });
  });
});

describe("getInstanceEntryFiles", () => {
  it("decodes the RPC's byte-map files back to strings", async () => {
    getInstanceEntryFiles.mockResolvedValue({
      files: { "manifest.yaml": new TextEncoder().encode("name: yandex\n") },
    });
    const files = await getInstanceEntryFilesFn("i1");
    expect(getInstanceEntryFiles).toHaveBeenCalledWith({ id: "i1" });
    expect(files).toEqual({ "manifest.yaml": "name: yandex\n" });
  });
});

describe("getOrgEntryFiles", () => {
  it("routes KIND_PROVIDER to GetOrgProviderFiles with the resolved tenant id", async () => {
    getOrgProviderFiles.mockResolvedValue({
      files: { "manifest.yaml": new TextEncoder().encode("name: yandex\n") },
    });
    const files = await getOrgEntryFiles("acme", "KIND_PROVIDER", "e1");
    expect(getOrgProviderFiles).toHaveBeenCalledWith({ tenantId: "t1", id: "e1" });
    expect(files).toEqual({ "manifest.yaml": "name: yandex\n" });
  });

  it("routes KIND_WORKFLOW to GetOrgWorkflowFiles", async () => {
    getOrgWorkflowFiles.mockResolvedValue({ files: {} });
    await getOrgEntryFiles("acme", "KIND_WORKFLOW", "e2");
    expect(getOrgWorkflowFiles).toHaveBeenCalledWith({ tenantId: "t1", id: "e2" });
    expect(getOrgProviderFiles).not.toHaveBeenCalled();
  });
});

describe("checkInstanceBundle", () => {
  it("sends encoded files with no tenant id, decodes diagnostics", async () => {
    checkInstanceProvider.mockResolvedValue({
      diagnostics: [{ severity: 1, path: "manifest.yaml", line: 1, col: 1, message: "bad", module: "m" }],
    });
    const diags = await checkInstanceBundle("KIND_PROVIDER", { "manifest.yaml": "name: x\n" });
    expect(diags).toEqual([
      { severity: "error", path: "manifest.yaml", line: 1, col: 1, message: "bad", module: "m" },
    ]);
    const call = checkInstanceProvider.mock.calls[0][0];
    expect(call.tenantId).toBeUndefined();
    expect(new TextDecoder().decode(call.files["manifest.yaml"])).toBe("name: x\n");
  });
});
