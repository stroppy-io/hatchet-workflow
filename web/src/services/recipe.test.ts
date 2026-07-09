import { describe, it, expect, vi, beforeEach } from "vitest";
import { create } from "@bufbuild/protobuf";
import { SchemaSchema, FieldErrorSchema } from "@stroppy-io/schemapb";

const { startRunMock } = vi.hoisted(() => ({ startRunMock: vi.fn() }));

vi.mock("@/services/client", () => ({
  recipeClient: {
    launchFormSchema: vi.fn().mockResolvedValue({
      schema: create(SchemaSchema, { id: { namespace: "ns", name: "form", version: "v1" }, fields: [] }),
    }),
    startRun: startRunMock,
  },
  dslClient: {},
}));
vi.mock("@/services/tenant", () => ({ resolveTenantId: vi.fn().mockResolvedValue("t1") }));

import { fetchLaunchFormSchema, startRun, StartRunFieldError } from "./recipe";

describe("fetchLaunchFormSchema", () => {
  it("resolves the tenant and returns the composed schema", async () => {
    const schema = await fetchLaunchFormSchema("acme", "r1");
    expect(schema.id?.name).toBe("form");
  });
});

describe("startRun", () => {
  beforeEach(() => {
    startRunMock.mockClear();
  });

  it("sends the filled payload and returns the minted run id on a clean bake", async () => {
    startRunMock.mockResolvedValueOnce({
      run: { entity: { id: "run-1" } },
      fieldErrors: [],
    });

    const runId = await startRun("acme", "r1", { values: { threads: 4 } });

    expect(runId).toBe("run-1");
    const call = startRunMock.mock.calls[0][0];
    expect(call.tenantId).toBe("t1");
    expect(call.recipeId).toBe("r1");
    expect(call.filled?.values?.threads).toBe(4);
  });

  it("throws a StartRunFieldError carrying field_errors and mints no run on a server-side bake failure", async () => {
    startRunMock.mockResolvedValueOnce({
      run: undefined,
      fieldErrors: [create(FieldErrorSchema, { field: "threads", message: "must be >= 1" })],
    });

    try {
      await startRun("acme", "r1", { values: { threads: -1 } });
      expect.unreachable();
    } catch (e) {
      expect(e).toBeInstanceOf(StartRunFieldError);
      expect((e as StartRunFieldError).fieldErrors[0]?.field).toBe("threads");
    }
  });

  it("launches with no filled payload when the recipe has no launch form (today's no-inputs behavior)", async () => {
    startRunMock.mockResolvedValueOnce({
      run: { entity: { id: "run-2" } },
      fieldErrors: [],
    });

    const runId = await startRun("acme", "r1");

    expect(runId).toBe("run-2");
    expect(startRunMock.mock.calls[0][0].filled).toBeUndefined();
  });
});
