import { describe, it, expect, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import { SchemaSchema } from "@stroppy-io/schemapb";

vi.mock("@/services/client", () => ({
  recipeClient: {
    launchFormSchema: vi.fn().mockResolvedValue({
      schema: create(SchemaSchema, { id: { namespace: "ns", name: "form", version: "v1" }, fields: [] }),
    }),
  },
  dslClient: {},
}));
vi.mock("@/services/tenant", () => ({ resolveTenantId: vi.fn().mockResolvedValue("t1") }));

import { fetchLaunchFormSchema } from "./recipe";

describe("fetchLaunchFormSchema", () => {
  it("resolves the tenant and returns the composed schema", async () => {
    const schema = await fetchLaunchFormSchema("acme", "r1");
    expect(schema.id?.name).toBe("form");
  });
});
