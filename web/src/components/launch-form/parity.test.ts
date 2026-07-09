import { describe, it, expect } from "vitest";
import { createHash } from "node:crypto";
import { fromJson, type JsonValue } from "@bufbuild/protobuf";
import { SchemaSchema } from "@stroppy-io/schemapb";
import { loadSchemapbEngine } from "@/services/schemapbEngine";
import fixture from "../../../../internal/dsl/schema/testdata/parity_form.json";

// This is the SP-D Task 9 half of the invariant the whole launch-form design
// rests on: the browser's WASM bake (schemapbBake, driven here through
// @stroppy-io/schemapb's Schemapb.bake) and the server's Go bake
// (schemapb.Schema.Bake, via internal/dsl/schema.BakeForm) must agree, over
// the SAME schema and the SAME values. internal/dsl/schema/parity_wasm_fixture_test.go
// writes testdata/parity_form.json from one schema-construction call and
// records what Go does with four value sets; this file imports that exact
// JSON (crossing out of web/ into the Go module root -- confirmed the
// vitest node/jsdom pool reads it fine, no fs.allow issue under `vitest
// run`) and re-derives the same outcomes through the real .wasm artifact.
// A real behavioral divergence between the two engines must fail HERE, not
// be silently reconciled -- so this test drives loadSchemapbEngine()
// (backed by the actual vendored schemapb.wasm), never a stub.

// canonicalJSON mirrors the Go-side helper of the same name in
// parity_wasm_fixture_test.go byte-for-byte in algorithm (object keys
// sorted recursively, arrays kept in order, then JSON-encoded) so a sha256
// of the result is comparable across languages for this fixture's plain
// JSON scalars (strings, small integers/decimals, bools). schemapb.Hash
// itself (the protowire-based hasher schemapb.Baked implements in Go) has
// no WASM/TS counterpart exposed by @stroppy-io/schemapb -- see the
// Schemapb class in its .d.ts, whose only bake-shaped exports are
// validate/compute/bake/merge -- so this canonical-JSON sha256 is the
// documented fallback ("compare the canonical serialized Baked") rather
// than a re-implementation of schemapb's own hash algorithm in TS.
function canonicalJSON(v: unknown): string {
  if (Array.isArray(v)) {
    return "[" + v.map(canonicalJSON).join(",") + "]";
  }
  if (v !== null && typeof v === "object") {
    const obj = v as Record<string, unknown>;
    const keys = Object.keys(obj).sort();
    return "{" + keys.map((k) => JSON.stringify(k) + ":" + canonicalJSON(obj[k])).join(",") + "}";
  }
  return JSON.stringify(v);
}

function sha256Hex(s: string): string {
  return createHash("sha256").update(s).digest("hex");
}

function fieldPaths(errors: { field?: string }[]): string[] {
  return errors.map((e) => e.field ?? "");
}

describe("WASM/Go BakeForm parity (SP-D Task 9)", () => {
  it("agrees with Go on valid_values: accepts, and the resolved values hash matches Go's canonical sha256", async () => {
    const engine = await loadSchemapbEngine();
    const schema = fromJson(SchemaSchema, fixture.schema as unknown as JsonValue);

    const result = engine.bake(schema, fixture.valid_values);
    expect(result.errors).toHaveLength(0);
    expect(result.baked).toBeDefined();

    const gotHash = sha256Hex(canonicalJSON(result.baked?.values ?? {}));
    expect(gotHash).toBe(fixture.valid_baked_values_sha256);
  });

  it("agrees with Go on invalid_values: rejects, blocking the same field path (threads, Gte(1))", async () => {
    const engine = await loadSchemapbEngine();
    const schema = fromJson(SchemaSchema, fixture.schema as unknown as JsonValue);

    const result = engine.bake(schema, fixture.invalid_values);
    expect(result.baked).toBeUndefined();
    expect(result.errors.length).toBeGreaterThan(0);
    expect(fieldPaths(result.errors)).toContain(fixture.invalid_expected_field);
  });

  // Highest-value assertion in this task (per the SP-D plan): ComposeFormSchema
  // sets Strict on BOTH the root form schema and the nested "provider" object
  // (commit 056becbb, a security fix against silent config injection). If the
  // browser silently accepted an undeclared key that the server then rejects
  // (or vice versa), a user would see a green form and a failed run -- or
  // worse, a client could smuggle an unvalidated key past client-side UX
  // straight into a server bake that also happens to accept it. Both engines
  // must reject an undeclared top-level key.
  it("agrees with Go: an undeclared TOP-LEVEL key is rejected by the strict root schema on both sides", async () => {
    const engine = await loadSchemapbEngine();
    const schema = fromJson(SchemaSchema, fixture.schema as unknown as JsonValue);

    const result = engine.bake(schema, fixture.undeclared_values);
    expect(result.baked).toBeUndefined();
    expect(result.errors.length).toBeGreaterThan(0);
    expect(fieldPaths(result.errors)).toContain(fixture.undeclared_expected_field);
  });

  // Same invariant, but for the NESTED "provider" object -- ObjectOf clones
  // params into its own nested Schema with its own Strict flag, independent
  // of the root's strictness, so this must be checked separately.
  it("agrees with Go: an undeclared NESTED provider key is rejected by the strict provider schema on both sides", async () => {
    const engine = await loadSchemapbEngine();
    const schema = fromJson(SchemaSchema, fixture.schema as unknown as JsonValue);

    const result = engine.bake(schema, fixture.undeclared_provider_values);
    expect(result.baked).toBeUndefined();
    expect(result.errors.length).toBeGreaterThan(0);
    expect(fieldPaths(result.errors)).toContain(fixture.undeclared_provider_expected_field);
  });
});
