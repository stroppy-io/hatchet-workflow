import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { SchemaSchema, Schema_Filed_ResultType } from "@stroppy-io/schemapb";
import { LaunchFormRenderer } from "./LaunchFormRenderer";

function fieldCoverageSchema() {
  return create(SchemaSchema, {
    id: { namespace: "stroppy.test", name: "launch", version: "v1" },
    fields: [
      { name: "db_version", kind: { case: "string", value: { default: "16" } } },
      { name: "threads", kind: { case: "int64", value: { default: 4n, gte: 1n } } },
      { name: "ratio", kind: { case: "double", value: { default: 0.5 } } },
      { name: "ssl", kind: { case: "bool", value: { default: false } } },
      {
        name: "engine",
        kind: { case: "enum", value: { values: { 1: "postgres", 2: "mysql" }, definedOnly: true } },
      },
      {
        // `when` binds only `root` (never `this` — a field's own value must
        // not gate its own existence; see schemapb/schema.proto Filed.when).
        name: "tls_key",
        when: "root.ssl == true",
        kind: { case: "string", value: {} },
      },
      {
        name: "nodes",
        kind: { case: "list", value: { items: [{ name: "node", kind: { case: "string", value: {} } }] } },
      },
      {
        name: "provider",
        kind: {
          case: "object",
          value: {
            schema: create(SchemaSchema, {
              id: { namespace: "stroppy.test", name: "provider", version: "v1" },
              fields: [{ name: "zone", kind: { case: "string", value: {} } }],
            }),
          },
        },
      },
    ],
  });
}

describe("LaunchFormRenderer", () => {
  it("renders every declared field kind and hides a when-gated field until its condition is met", async () => {
    const onSubmit = vi.fn();
    render(<LaunchFormRenderer schema={fieldCoverageSchema()} onSubmit={onSubmit} />);

    await waitFor(() => expect(screen.getByLabelText("db_version")).toBeInTheDocument());
    expect(screen.getByLabelText("threads")).toBeInTheDocument();
    expect(screen.getByLabelText("ratio")).toBeInTheDocument();
    expect(screen.getByLabelText("ssl")).toBeInTheDocument();
    expect(screen.getByLabelText("engine")).toBeInTheDocument();
    expect(screen.getByLabelText("nodes")).toBeInTheDocument();
    expect(screen.getByLabelText("provider.zone")).toBeInTheDocument();

    // tls_key is when-gated on ssl == true — hidden until toggled. Before the
    // WASM engine finishes loading, fieldActive() has no engine to ask and
    // defaults to "active" (see useSchemaForm), so wait it out rather than
    // asserting synchronously.
    await waitFor(() => expect(screen.queryByLabelText("tls_key")).not.toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("ssl"));
    await waitFor(() => expect(screen.getByLabelText("tls_key")).toBeInTheDocument());
  });

  it("surfaces a FieldError when a constraint is violated", async () => {
    const onSubmit = vi.fn();
    render(<LaunchFormRenderer schema={fieldCoverageSchema()} onSubmit={onSubmit} />);

    await waitFor(() => expect(screen.getByLabelText("threads")).toBeInTheDocument());

    // "threads" requires gte 1 — driving the underlying <input type=number>
    // below that should surface a FieldError alert next to it. (The field
    // displays "0" until touched, so use a value that actually differs —
    // otherwise React's input value tracker treats the change as a no-op.)
    const threadsInput = screen.getByLabelText("threads") as HTMLInputElement;
    fireEvent.change(threadsInput, { target: { value: "-1" } });

    await waitFor(() => expect(screen.getByRole("alert")).toBeInTheDocument());
  });

  it("recomputes a Computed field live as its inputs change", async () => {
    const onSubmit = vi.fn();
    const schema = create(SchemaSchema, {
      id: { namespace: "stroppy.test", name: "computed", version: "v1" },
      fields: [
        { name: "workers", kind: { case: "int64", value: { default: 2n } } },
        {
          name: "total_ram_mb",
          kind: { case: "computed", value: { expr: "root.workers * 1024", result: Schema_Filed_ResultType.INT64 } },
        },
      ],
    });
    render(<LaunchFormRenderer schema={schema} onSubmit={onSubmit} />);

    await waitFor(() => expect(screen.getByLabelText("workers")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByLabelText("total_ram_mb")).toHaveTextContent("2048"));

    const workersInput = screen.getByLabelText("workers") as HTMLInputElement;
    fireEvent.change(workersInput, { target: { value: "3" } });

    await waitFor(() => expect(screen.getByLabelText("total_ram_mb")).toHaveTextContent("3072"));
  });
});
