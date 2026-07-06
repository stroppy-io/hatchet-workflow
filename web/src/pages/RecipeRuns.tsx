// RecipeRuns — the list page for RecipeService.ListRuns: runs launched from
// (or alongside) recipe bundles for the current tenant. Mirrors the dark-theme
// "tenant working page" chrome shared by Recipes.tsx (h1 header row, p-5 page
// padding, Card + shadcn Table body) since this list — like Recipes.tsx — has
// no filter/sort/paging surface to justify the heavier tanstack-table +
// LibraryTable machinery Runs.tsx uses.
//
// Row -> RunVM (services/recipe.ts, NOT services/runs.ts's richer RunVM): id,
// name, status, recipeId, startedAt. Name links to RunDetail (/runs/:id) which
// renders the FULL overview via testRunOverviewClient — unrelated to this
// list's data source. Actions: Cancel (running/pending only, confirm) and
// Delete (anything not running, confirm) via recipe.ts's cancelRun/deleteRun.
//
// listRuns(slug) with no recipeId returns EVERY run for the tenant (recipe.ts
// RecipeService.ListRuns falls back to the tenant's full run list when
// recipe_id is unset — see recipe.ts's doc comment) — v1 scope per the task
// brief; a future pass can add a recipe-id filter / search box.

import { useCallback, useEffect, useState } from "react";
import { AlertCircle, Loader2, Square, Trash2 } from "lucide-react";
import { Link, useNavigate, useTenantSlug } from "@/lib/router";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useConfirm } from "@/components/ui/confirm-dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cancelRun, deleteRun, listRuns, type RunVM } from "@/services/recipe";
import type { RunStatus } from "@/services/dashboard";

const STATUS_VARIANT: Record<RunStatus, "default" | "success" | "destructive" | "warning" | "pending"> = {
  pending: "pending",
  running: "default",
  cancelling: "warning",
  completed: "success",
  failed: "destructive",
  cancelled: "warning",
};

// Cancel is only meaningful while a run can still accept a cancel request;
// delete is blocked only while the run is actively in flight (mirrors
// services/runs.ts's actionsForStatus gating, without importing it — that
// helper's action set (rerun/clone/...) belongs to a wider surface this page
// does not use).
function canCancel(status: RunStatus): boolean {
  return status === "running" || status === "pending";
}
function canDelete(status: RunStatus): boolean {
  return status !== "running" && status !== "cancelling";
}

function fmtStarted(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(d);
}

export function RecipeRuns() {
  const slug = useTenantSlug() ?? "";
  const navigate = useNavigate();
  const confirm = useConfirm();

  const [runs, setRuns] = useState<RunVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  const fetchRuns = useCallback(async () => {
    if (!slug) return;
    setLoading(true);
    setError(null);
    try {
      const rows = await listRuns(slug);
      setRuns(rows);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load runs");
      setRuns([]);
    } finally {
      setLoading(false);
    }
  }, [slug]);

  useEffect(() => {
    void fetchRuns();
  }, [fetchRuns]);

  const handleCancel = useCallback(
    async (run: RunVM) => {
      if (!slug) return;
      const ok = await confirm({
        title: "Cancel run?",
        description: "The run will be stopped. Already-provisioned resources are torn down during teardown.",
        danger: true,
        confirmLabel: "Cancel run",
      });
      if (!ok) return;
      setBusyId(run.id);
      setError(null);
      try {
        await cancelRun(slug, run.id);
        await fetchRuns();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to cancel run");
      } finally {
        setBusyId(null);
      }
    },
    [slug, confirm, fetchRuns],
  );

  const handleDelete = useCallback(
    async (run: RunVM) => {
      if (!slug) return;
      const ok = await confirm({
        title: "Delete run?",
        description: `"${run.name || run.id}" will be permanently removed. This cannot be undone.`,
        danger: true,
        confirmLabel: "Delete",
      });
      if (!ok) return;
      setBusyId(run.id);
      setError(null);
      try {
        await deleteRun(slug, run.id);
        await fetchRuns();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to delete run");
      } finally {
        setBusyId(null);
      }
    },
    [slug, confirm, fetchRuns],
  );

  return (
    <div className="p-5 flex flex-col gap-4 h-full min-h-0">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-base font-semibold font-mono tracking-tight">Runs</h1>
      </div>

      {error && (
        <div className="flex items-center gap-2 text-xs p-2.5 border border-destructive/30 text-destructive font-mono">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" />
          {error}
        </div>
      )}

      <Card className="flex-1 min-h-0 overflow-auto">
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Started</TableHead>
                <TableHead className="w-[136px] text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={4} className="h-32 text-center">
                    <span className="inline-flex items-center gap-2 text-xs text-muted-foreground font-mono">
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      Loading runs...
                    </span>
                  </TableCell>
                </TableRow>
              ) : runs.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className="h-32 text-center">
                    <span className="text-xs text-muted-foreground font-mono">
                      No runs yet — start one from a recipe
                    </span>
                  </TableCell>
                </TableRow>
              ) : (
                runs.map((run) => (
                  <TableRow
                    key={run.id}
                    className="cursor-pointer"
                    onClick={() => navigate(`/runs/${run.id}`)}
                  >
                    <TableCell>
                      <Link
                        to={`/runs/${run.id}`}
                        onClick={(e) => e.stopPropagation()}
                        className="text-xs text-primary hover:underline underline-offset-2"
                      >
                        {run.name || run.id}
                      </Link>
                    </TableCell>
                    <TableCell>
                      <Badge variant={STATUS_VARIANT[run.status]}>{run.status}</Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {fmtStarted(run.startedAt)}
                    </TableCell>
                    <TableCell onClick={(e) => e.stopPropagation()}>
                      <div className="flex justify-end gap-1">
                        {canCancel(run.status) && (
                          <Button
                            variant="ghost"
                            size="icon"
                            disabled={busyId === run.id}
                            onClick={() => void handleCancel(run)}
                            title="Cancel run"
                            className="text-warning hover:text-warning"
                          >
                            {busyId === run.id ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : (
                              <Square className="h-4 w-4" />
                            )}
                          </Button>
                        )}
                        {canDelete(run.status) && (
                          <Button
                            variant="ghost"
                            size="icon"
                            disabled={busyId === run.id}
                            onClick={() => void handleDelete(run)}
                            title="Delete run"
                            className="text-destructive hover:text-destructive"
                          >
                            {busyId === run.id ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : (
                              <Trash2 className="h-4 w-4" />
                            )}
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
