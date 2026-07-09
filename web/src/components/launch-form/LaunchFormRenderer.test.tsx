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

  it("narrows Enum options via options_expr instead of the static values map (review fix 1)", async () => {
    const onSubmit = vi.fn();
    const schema = create(SchemaSchema, {
      id: { namespace: "stroppy.test", name: "enum-options-expr", version: "v1" },
      fields: [
        { name: "kind", kind: { case: "int64", value: { default: 1n } } },
        {
          name: "engine",
          kind: {
            case: "enum",
            value: {
              values: { 1: "postgres", 2: "mysql", 3: "mongo" },
              // Narrows the option set based on `kind`: postgres/mysql are
              // relational engines (kind==1), mongo is the only choice
              // otherwise. The static `values` map above would offer all
              // three regardless of `kind` if a renderer read it directly —
              // exactly the landmine this test guards against.
              optionsExpr: "root.kind == 1 ? [1, 2] : [3]",
            },
          },
        },
      ],
    });
    render(<LaunchFormRenderer schema={schema} onSubmit={onSubmit} />);

    await waitFor(() => expect(screen.getByLabelText("engine")).toBeInTheDocument());

    // `kind` starts unset until the WASM engine's compute pass fills in the
    // declared default (same async settling the "recomputes a Computed
    // field" test above waits out) — drive it explicitly to 1 and wait for
    // that to land before trusting options_expr's evaluation of root.kind.
    const kindInput = screen.getByLabelText("kind") as HTMLInputElement;
    fireEvent.change(kindInput, { target: { value: "1" } });
    await waitFor(() => expect(kindInput.value).toBe("1"));

    const trigger = screen.getByLabelText("engine");
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
    await waitFor(() => expect(screen.getByRole("listbox")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByRole("option", { name: "postgres" })).toBeInTheDocument());
    expect(screen.getByRole("option", { name: "mysql" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "mongo" })).not.toBeInTheDocument();

    // Close the listbox, flip `kind` away from 1, and reopen: the option set
    // must be recomputed by the engine (mongo only), not stay stuck at the
    // first render's static/derived list.
    fireEvent.keyDown(trigger, { key: "Escape" });
    fireEvent.change(kindInput, { target: { value: "2" } });
    await waitFor(() => expect(kindInput.value).toBe("2"));

    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
    await waitFor(() => expect(screen.getByRole("listbox")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByRole("option", { name: "mongo" })).toBeInTheDocument());
    expect(screen.queryByRole("option", { name: "postgres" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "mysql" })).not.toBeInTheDocument();
  });

  it("honors a field's negative lower bound instead of NumField's default min=0 (review fix 2)", async () => {
    const onSubmit = vi.fn();
    const schema = create(SchemaSchema, {
      id: { namespace: "stroppy.test", name: "negative-bounds", version: "v1" },
      fields: [
        // gte(-10) is a legitimately negative range — the input must not
        // carry an HTML min="0" clamp that would silently reject/clamp
        // negative values before they ever reach the schemapb validator.
        { name: "offset", kind: { case: "int64", value: { default: 0n, gte: -10n } } },
      ],
    });
    render(<LaunchFormRenderer schema={schema} onSubmit={onSubmit} />);

    await waitFor(() => expect(screen.getByLabelText("offset")).toBeInTheDocument());
    const offsetInput = screen.getByLabelText("offset") as HTMLInputElement;
    expect(offsetInput).not.toHaveAttribute("min", "0");
    expect(offsetInput.min).toBe("-10");

    // A negative value within the declared bound must validate through the
    // real engine (no FieldError), proving the bound is honored end-to-end
    // and not just cosmetically on the <input>.
    fireEvent.change(offsetInput, { target: { value: "-5" } });
    await waitFor(() => expect(offsetInput.value).toBe("-5"));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
