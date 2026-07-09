// Thin, memoized loader over @stroppy-io/schemapb so the rest of web/ never
// touches the WASM engine's globalThis functions directly (D1 — see
// docs/superpowers/specs/2026-07-08-sp-d-launch-form.md §3 D1). The vendor
// package (packages/schemapb inside github.com/stroppy-io/schemapb) already
// owns the WASM instantiation, caching, and JSON<->protojson bridging.
import { schemapb, type Schemapb } from "@stroppy-io/schemapb";

export type { Schemapb } from "@stroppy-io/schemapb";

/**
 * Returns the shared, lazily-loaded schemapb WASM engine. Safe to call from
 * multiple components: @stroppy-io/schemapb's own `schemapb()` helper
 * memoizes the load, this wrapper exists purely so callers depend on
 * services/schemapbEngine (this app's own module boundary) rather than the
 * vendor package directly.
 */
export function loadSchemapbEngine(): Promise<Schemapb> {
  return schemapb();
}
