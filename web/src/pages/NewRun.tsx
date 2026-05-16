import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { clients } from "@/api/clients";
import { getTenantId } from "@/api/transport";
import type { DatabasePreset } from "@/lib/proto/cloud/v1/catalog/database_pb";
import type { WorkloadPreset } from "@/lib/proto/cloud/v1/catalog/workload_pb";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { AlertCircle, Rocket } from "lucide-react";

export function NewRun() {
  const navigate = useNavigate();
  const tid = getTenantId();

  const [name, setName] = useState("");
  const [dbPresets, setDbPresets] = useState<DatabasePreset[]>([]);
  const [wlPresets, setWlPresets] = useState<WorkloadPreset[]>([]);
  const [selectedDbPreset, setSelectedDbPreset] = useState("");
  const [selectedWlPreset, setSelectedWlPreset] = useState("");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!tid) return;
    Promise.all([
      clients.databasePreset.listDatabasePresets({ value: tid }),
      clients.workloadPreset.listWorkloadPresets({ value: tid }),
    ])
      .then(([dbr, wlr]) => {
        const dps = dbr.databasePresets ?? [];
        const wps = wlr.workloadPresets ?? [];
        setDbPresets(dps);
        setWlPresets(wps);
        if (dps.length > 0) setSelectedDbPreset(dps[0].id?.value ?? "");
        if (wps.length > 0) setSelectedWlPreset(wps[0].id?.value ?? "");
      })
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Failed to load presets")
      )
      .finally(() => setLoading(false));
  }, [tid]);

  if (!tid) {
    return (
      <div className="p-6 text-sm text-muted-foreground">
        No tenant selected. Please select a tenant first.
      </div>
    );
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!tid) return;
    setSubmitting(true);
    setError(null);

    try {
      // Build TestRun with discriminated union oneofs
      const testRun = {
        identity: { name: name.trim() || undefined },
        ...(selectedDbPreset
          ? {
              database: {
                databaseVariant: {
                  case: "databasePresetId" as const,
                  value: { value: selectedDbPreset },
                },
              },
            }
          : {}),
        ...(selectedWlPreset
          ? {
              workload: {
                workloadVariant: {
                  case: "workloadPresetId" as const,
                  value: { value: selectedWlPreset },
                },
              },
            }
          : {}),
      };

      const created = await clients.testRun.createTestRun({ testRun });
      const createdId = created.id?.value;
      if (!createdId) throw new Error("No run ID returned from createTestRun");

      await clients.testRun.launchTestRun({ value: createdId });
      navigate(`/runs/${createdId}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create run");
      setSubmitting(false);
    }
  }

  return (
    <div className="p-6 max-w-lg space-y-6">
      <div>
        <h1 className="text-lg font-semibold">New Test Run</h1>
        <p className="text-sm text-muted-foreground">
          Configure and launch a new benchmark run.
        </p>
      </div>

      {error && (
        <div className="flex items-center gap-2 text-sm p-3 border border-destructive/30 text-destructive">
          <AlertCircle className="h-4 w-4" />
          {error}
        </div>
      )}

      {loading ? (
        <div className="text-sm text-muted-foreground">Loading presets...</div>
      ) : (
        <form onSubmit={handleSubmit} className="space-y-5">
          {/* Name */}
          <div className="space-y-1.5">
            <Label htmlFor="run-name">Run Name (optional)</Label>
            <Input
              id="run-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. tpcc-pg17-baseline"
            />
          </div>

          {/* Database Preset */}
          <div className="space-y-1.5">
            <Label htmlFor="db-preset">Database Preset</Label>
            {dbPresets.length === 0 ? (
              <div className="text-sm text-zinc-500">
                No database presets available.{" "}
                <a href="/presets" className="text-primary underline">
                  Create one first.
                </a>
              </div>
            ) : (
              <select
                id="db-preset"
                value={selectedDbPreset}
                onChange={(e) => setSelectedDbPreset(e.target.value)}
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                {dbPresets.map((p) => (
                  <option key={p.id?.value} value={p.id?.value ?? ""}>
                    {p.identity?.name ?? p.id?.value}
                  </option>
                ))}
              </select>
            )}
          </div>

          {/* Workload Preset */}
          <div className="space-y-1.5">
            <Label htmlFor="wl-preset">Workload Preset</Label>
            {wlPresets.length === 0 ? (
              <div className="text-sm text-zinc-500">
                No workload presets available.{" "}
                <a href="/presets" className="text-primary underline">
                  Create one first.
                </a>
              </div>
            ) : (
              <select
                id="wl-preset"
                value={selectedWlPreset}
                onChange={(e) => setSelectedWlPreset(e.target.value)}
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                {wlPresets.map((p) => (
                  <option key={p.id?.value} value={p.id?.value ?? ""}>
                    {p.identity?.name ?? p.id?.value}
                  </option>
                ))}
              </select>
            )}
          </div>

          <div className="flex gap-3 pt-2">
            <Button
              type="submit"
              disabled={submitting || (dbPresets.length === 0 && wlPresets.length === 0)}
              className="gap-2"
            >
              <Rocket className="h-4 w-4" />
              {submitting ? "Launching..." : "Launch Run"}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={() => navigate("/runs")}
            >
              Cancel
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}
