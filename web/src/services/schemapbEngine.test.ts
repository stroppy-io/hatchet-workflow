import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import { SchemaSchema } from "@stroppy-io/schemapb";
import { loadSchemapbEngine } from "./schemapbEngine";

describe("loadSchemapbEngine", () => {
  it("loads the shared WASM engine and bakes a trivial schema", async () => {
    const engine = await loadSchemapbEngine();
    const schema = create(SchemaSchema, {
      id: { namespace: "stroppy.test", name: "smoke", version: "v1" },
      fields: [
        { name: "replicas", required: true, kind: { case: "int64", value: { gte: 1n } } },
      ],
    });

    const bad = engine.bake(schema, { replicas: 0 });
    expect(bad.baked).toBeUndefined();
    expect(bad.errors.length).toBeGreaterThan(0);

    const ok = engine.bake(schema, { replicas: 3 });
    expect(ok.baked).toBeDefined();
    expect(ok.errors).toHaveLength(0);
  });
});
