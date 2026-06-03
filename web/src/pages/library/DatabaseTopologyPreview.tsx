// Read-only topology preview for a database preset's typed domain.Database.
//
// Renders the node groups engineNodes(db) implies (mirroring the server topology
// builder) as compact role cards — the same derivation the New Run wizard shows,
// shared by the preset create/edit form and the detail page.

import {
  Box,
  Cpu,
  Database,
  Globe,
  Layers,
  Server,
} from "lucide-react";
import { ENGINES, type DatabaseVM } from "@/services/wizard";
import { engineNodes } from "@/components/database/DatabaseParamsForm";

const ROLE_ICON: Record<string, typeof Database> = {
  master: Database,
  primary: Server,
  replica: Database,
  instance: Cpu,
  storage: Database,
  node: Database,
  database: Database,
  haproxy: Globe,
  proxysql: Globe,
  etcd: Layers,
  managed: Globe,
  external: Globe,
};

export function TopologyPreview({ database }: { database: DatabaseVM }) {
  const groups = engineNodes(database);
  const total = groups.reduce((n, g) => n + g.count, 0);

  return (
    <div className="flex flex-col border border-zinc-800/60 bg-[#070707] p-3">
      <div className="mb-2 flex items-center justify-between">
        <span className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
          {total > 0 ? `${total} node${total > 1 ? "s" : ""}` : "no self-deployed nodes"}
        </span>
      </div>
      {total === 0 ? (
        <p className="text-[11px] leading-snug text-zinc-600">
          {database.kind === "external"
            ? "External endpoint — connects via DSN, nothing is deployed."
            : "Managed service — no self-deployed nodes; only the stroppy client runs."}
        </p>
      ) : (
        <div className="grid auto-rows-min grid-cols-2 gap-2">
          {groups.map((g) => {
            const Icon = ROLE_ICON[g.role] ?? Box;
            const meta = ENGINES.find((e) => e.kind === g.engine);
            const color = meta?.hex ?? "#71717a";
            return (
              <div key={`${g.role}-${g.engine}`} className="flex flex-col gap-1 border border-zinc-800 bg-[#0a0a0a] p-2.5">
                <div className="flex items-center gap-2">
                  <Icon className="h-4 w-4 shrink-0" style={{ color }} />
                  <span className="truncate font-mono text-[11px] text-zinc-300">{g.role}</span>
                  {g.count > 1 && (
                    <span className="ml-auto font-mono text-[10px] text-zinc-500">×{g.count}</span>
                  )}
                </div>
                <div className="font-mono text-[10px] text-zinc-600">{g.engine}</div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
