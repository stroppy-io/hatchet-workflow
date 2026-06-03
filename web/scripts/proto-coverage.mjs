#!/usr/bin/env node
// proto-coverage.mjs — frontend/backend integration starting point.
//
// Parses the generated protobuf TS (web/src/lib/proto) to enumerate every
// service RPC and every message field, then scans the ACTIVE frontend source
// (web/src, excluding lib/proto and old/) to estimate which RPCs we call and
// which fields we actually use. Output is a coverage report: implemented vs
// total handlers, used vs total fields (grouped by proto package), and the
// gap lists (unused RPCs / unused fields) to drive integration.
//
// Heuristics (intentionally simple, documented):
//  - RPC "implemented" = the camelCase method name is called as `.method(` in
//    app source (the connect client method).
//  - Field "used" = the camelCase field name is accessed as `.field` anywhere
//    in app source. Field names collide across messages (id/name/status/...),
//    so a field is counted used if its name appears used ANYWHERE — this
//    over-counts common names; treat per-message numbers as an upper bound and
//    the UNUSED lists as the reliable signal.
//
// Usage:  node scripts/proto-coverage.mjs [--json] [--unused]
//   (run from web/, or from repo root: node web/scripts/proto-coverage.mjs)

import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const WEB = resolve(__dirname, "..");
const PROTO_DIR = join(WEB, "src", "lib", "proto");
const APP_DIR = join(WEB, "src");
const APP_EXCLUDE = [join(WEB, "src", "lib", "proto"), join(WEB, "src", "old")];

const args = new Set(process.argv.slice(2));
const asJson = args.has("--json");
const showUnused = args.has("--unused");

// ---- file walking ----------------------------------------------------------
function walk(dir, exclude = []) {
  const out = [];
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (exclude.some((e) => p === e || p.startsWith(e + "/"))) continue;
    const st = statSync(p);
    if (st.isDirectory()) out.push(...walk(p, exclude));
    else if (/\.(ts|tsx)$/.test(p) && !p.endsWith(".d.ts")) out.push(p);
  }
  return out;
}

const pkgOf = (fqmn) => {
  // cloud.v1.api.ListTestRunsRequest -> api ; cloud.v1.models.X -> models
  const m = /^cloud\.v1\.([a-z_]+)\./.exec(fqmn);
  return m ? m[1] : "other";
};
// classify api messages as request/response/other for the integration view
function apiRole(typeName) {
  if (/Request$/.test(typeName)) return "request";
  if (/Response$/.test(typeName)) return "response";
  return "message";
}

// ---- parse generated proto -------------------------------------------------
const protoFiles = walk(PROTO_DIR);
const services = new Map(); // serviceFqn -> Set(methodCamel)
const messages = []; // { type, fqmn, pkg, role, fields:Set<camel> }

const RPC_RE = /@generated from rpc ([\w.]+)\.(\w+)/g;

for (const f of protoFiles) {
  const src = readFileSync(f, "utf8");

  let m;
  while ((m = RPC_RE.exec(src))) {
    const svc = m[1];
    const method = m[2][0].toLowerCase() + m[2].slice(1);
    if (!services.has(svc)) services.set(svc, new Set());
    services.get(svc).add(method);
  }

  // messages + their fields. We read the object body that follows the header.
  const lines = src.split("\n");
  for (let i = 0; i < lines.length; i++) {
    const h = /export type (\w+) = Message<"([\w.]+)">/.exec(lines[i]);
    if (!h) continue;
    const type = h[1];
    const fqmn = h[2];
    // skip the *Json / *Valid mirror types (only the canonical Message type)
    if (/Json$/.test(type) || /Valid$/.test(type)) continue;
    // collect body until a line that is just "};"
    const fields = new Set();
    let depth = 0;
    let started = false;
    for (let j = i; j < lines.length; j++) {
      const ln = lines[j];
      for (const ch of ln) {
        if (ch === "{") {
          depth++;
          started = true;
        } else if (ch === "}") depth--;
      }
      // a field declaration: leading ws + ident + optional ? + ':'
      const fm = /^\s{2,}([a-z][A-Za-z0-9]*)\??:/.exec(ln);
      if (started && fm && depth >= 1) fields.add(fm[1]);
      if (started && depth <= 0) break;
    }
    fields.delete("$typeName");
    messages.push({ type, fqmn, pkg: pkgOf(fqmn), role: apiRole(type), fields });
  }
}

// Scope: count ONLY the frontend-facing API surface — the cloud.v1.api services
// + the product data packages their requests/responses carry. Drop the internal
// orchestration (workflow, agent) and the non-product noise (validate rules /
// schemapb, bucketed as "other").
const EXCLUDE_PKGS = new Set(["workflow", "agent", "other"]);
for (const svc of [...services.keys()]) {
  if (!svc.startsWith("cloud.v1.api.")) services.delete(svc);
}

// dedupe messages by fqmn (a type may appear once; nested types are distinct),
// then drop the excluded packages.
const seen = new Set();
const msgs = messages
  .filter((x) => (seen.has(x.fqmn) ? false : seen.add(x.fqmn)))
  .filter((x) => !EXCLUDE_PKGS.has(x.pkg));

// ---- scan app source -------------------------------------------------------
const appFiles = walk(APP_DIR, APP_EXCLUDE);
const appText = appFiles.map((f) => readFileSync(f, "utf8")).join("\n");

const usedMethod = (name) => new RegExp("\\." + name + "\\s*\\(").test(appText);
// precompute the set of camel field names that appear as a property access
const accessRe = /\.([a-z][A-Za-z0-9]*)\b/g;
const accessed = new Set();
for (let m; (m = accessRe.exec(appText)); ) accessed.add(m[1]);
const usedField = (name) => accessed.has(name);

// ---- compute coverage ------------------------------------------------------
const svcReport = [];
let rpcTotal = 0;
let rpcUsed = 0;
for (const [svc, methods] of [...services].sort()) {
  const impl = [...methods].filter(usedMethod);
  rpcTotal += methods.size;
  rpcUsed += impl.length;
  svcReport.push({
    service: svc.replace(/^cloud\.v1\.api\./, ""),
    total: methods.size,
    used: impl.length,
    unused: [...methods].filter((x) => !usedMethod(x)).sort(),
  });
}

// fields grouped by proto package
const pkgAgg = new Map(); // pkg -> { fieldsTotal, fieldsUsed, msgs, msgsTouched }
const allFieldNames = new Set();
const usedFieldNames = new Set();
const msgReport = [];
for (const msg of msgs) {
  let used = 0;
  for (const fld of msg.fields) {
    allFieldNames.add(fld);
    if (usedField(fld)) {
      used++;
      usedFieldNames.add(fld);
    }
  }
  const agg = pkgAgg.get(msg.pkg) || {
    fieldsTotal: 0,
    fieldsUsed: 0,
    msgs: 0,
    msgsTouched: 0,
  };
  agg.fieldsTotal += msg.fields.size;
  agg.fieldsUsed += used;
  agg.msgs += 1;
  if (used > 0) agg.msgsTouched += 1;
  pkgAgg.set(msg.pkg, agg);
  msgReport.push({
    type: msg.type,
    pkg: msg.pkg,
    role: msg.role,
    total: msg.fields.size,
    used,
    unused: [...msg.fields].filter((f) => !usedField(f)).sort(),
  });
}

const pct = (a, b) => (b === 0 ? "—" : ((100 * a) / b).toFixed(0) + "%");

// ---- output ----------------------------------------------------------------
if (asJson) {
  console.log(
    JSON.stringify(
      {
        rpc: { total: rpcTotal, used: rpcUsed, services: svcReport },
        fields: {
          distinctTotal: allFieldNames.size,
          distinctUsed: usedFieldNames.size,
          packages: [...pkgAgg].map(([pkg, a]) => ({ pkg, ...a })),
          messages: msgReport,
        },
      },
      null,
      2,
    ),
  );
  process.exit(0);
}

const bar = "=".repeat(72);
console.log(bar);
console.log("PROTO COVERAGE — frontend integration starting point");
console.log(bar);
console.log(`proto files: ${protoFiles.length}   app files scanned: ${appFiles.length} (excl. lib/proto, old/)`);
console.log("");
console.log(`API HANDLERS (RPC):  ${rpcUsed}/${rpcTotal}  (${pct(rpcUsed, rpcTotal)})  across ${services.size} services`);
console.log(`MESSAGES:            ${msgs.length}`);
console.log(`DISTINCT FIELDS used: ${usedFieldNames.size}/${allFieldNames.size}  (${pct(usedFieldNames.size, allFieldNames.size)})  [name-based, upper bound]`);
console.log("");

console.log("── RPC per service ──────────────────────────────────────────────");
for (const s of svcReport) {
  console.log(`  ${s.used}/${s.total}  ${pct(s.used, s.total).padStart(4)}  ${s.service}`);
}
console.log("");

console.log("── Fields per proto package ─────────────────────────────────────");
console.log("   used/total  %    msgs(touched/total)  package");
for (const [pkg, a] of [...pkgAgg].sort((x, y) => y[1].fieldsTotal - x[1].fieldsTotal)) {
  console.log(
    `   ${String(a.fieldsUsed).padStart(4)}/${String(a.fieldsTotal).padEnd(4)} ${pct(a.fieldsUsed, a.fieldsTotal).padStart(4)}  ${String(a.msgsTouched).padStart(3)}/${String(a.msgs).padEnd(3)}             ${pkg}`,
  );
}
console.log("");

// messages with zero used fields = not integrated at all
const untouched = msgReport.filter((m) => m.used === 0).sort((a, b) => a.type.localeCompare(b.type));
console.log(`── Messages with NO used fields (${untouched.length}) — integration TODO ──`);
console.log("   " + untouched.map((m) => `${m.pkg}.${m.type}`).join(", "));
console.log("");

if (showUnused) {
  console.log("── Unused RPCs ──────────────────────────────────────────────────");
  for (const s of svcReport) {
    if (s.unused.length) console.log(`  ${s.service}: ${s.unused.join(", ")}`);
  }
  console.log("");
  console.log("── Per-message unused fields (touched messages only) ────────────");
  for (const m of msgReport) {
    if (m.used > 0 && m.unused.length) {
      console.log(`  ${m.pkg}.${m.type} (${m.used}/${m.total}): ${m.unused.join(", ")}`);
    }
  }
}

console.log(bar);
console.log("Run with --unused for the full gap lists, --json for machine output.");
console.log(bar);
