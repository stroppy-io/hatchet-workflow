// Favorites — the caller's personal favorite join rows across every kind
// (FavoriteService.ListFavorites). Favorites are raw (kind, target_id) join
// rows with no denormalized target snapshot, so the page lists them grouped by
// kind, each row linking to the target's detail route (when one exists).

import { useCallback, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { Star, ArrowRight, RefreshCw } from "lucide-react";

import { Link, useTenantSlug } from "@/lib/router";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import {
  listFavorites,
  favoriteTargetPath,
  FAVORITE_KIND_LABEL,
  type FavoriteVM,
  type FavoriteKindLabel,
} from "@/services/favorites";

// Display order of kind groups.
const KIND_ORDER: FavoriteKindLabel[] = [
  "test_run",
  "suite",
  "suite_run",
  "test_preset",
  "database_preset",
  "workload_preset",
];

export function Favorites() {
  const tenantSlug = useTenantSlug() ?? "";

  const [favorites, setFavorites] = useState<FavoriteVM[]>([]);
  const [nextPageToken, setNextPageToken] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(
    async (pageToken?: string) => {
      if (!tenantSlug) return;
      setLoading(true);
      setError(null);
      try {
        const page = await listFavorites(tenantSlug, { pageToken });
        setFavorites((prev) =>
          pageToken ? [...prev, ...page.favorites] : page.favorites,
        );
        setNextPageToken(page.nextPageToken);
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      } finally {
        setLoading(false);
      }
    },
    [tenantSlug],
  );

  useEffect(() => {
    void load();
  }, [load]);

  // Group favorites by kind, preserving the declared display order.
  const grouped = useMemo(() => {
    const byKind = new Map<FavoriteKindLabel | "", FavoriteVM[]>();
    for (const f of favorites) {
      const arr = byKind.get(f.kind) ?? [];
      arr.push(f);
      byKind.set(f.kind, arr);
    }
    const out: { kind: FavoriteKindLabel | ""; rows: FavoriteVM[] }[] = [];
    for (const k of KIND_ORDER) {
      const rows = byKind.get(k);
      if (rows && rows.length) out.push({ kind: k, rows });
    }
    // Unknown-kind rows (shouldn't happen) trail at the end.
    const unknown = byKind.get("");
    if (unknown && unknown.length) out.push({ kind: "", rows: unknown });
    return out;
  }, [favorites]);

  return (
    <div className="min-h-full bg-background text-foreground">
      <div className="mx-auto flex max-w-[1100px] flex-col gap-4 p-4 md:p-6">
        <div className="flex flex-col gap-3 border-b border-border pb-4 md:flex-row md:items-center md:justify-between">
          <div className="flex items-center gap-2 text-lg font-semibold">
            <Star className="h-5 w-5 text-primary" />
            <h1>Favorites</h1>
            <span className="text-xs text-muted-foreground">
              ({favorites.length})
            </span>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => void load()}
            disabled={loading}
          >
            <RefreshCw className={cn("h-4 w-4", loading && "animate-spin")} />{" "}
            Refresh
          </Button>
        </div>

        {error && (
          <div className="border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </div>
        )}

        {grouped.length === 0 && (
          <div className="border border-border bg-muted/20 px-4 py-12 text-center text-sm text-muted-foreground">
            {loading
              ? "Loading favorites..."
              : "No favorites yet. Star a run, suite, or preset to pin it here."}
          </div>
        )}

        {grouped.map((g) => (
          <Section
            key={g.kind || "unknown"}
            title={g.kind ? FAVORITE_KIND_LABEL[g.kind] : "Other"}
            count={g.rows.length}
          >
            <ul className="divide-y divide-border/70 border border-border">
              {g.rows.map((f) => {
                const path = favoriteTargetPath(f.kind, f.targetId);
                const inner = (
                  <div className="flex items-center justify-between gap-3 px-3 py-2.5">
                    <div className="min-w-0">
                      <div className="font-mono text-xs text-foreground">
                        {f.targetId || "—"}
                      </div>
                      <div className="mt-0.5 text-[11px] text-muted-foreground">
                        Favorited {fmtTime(f.createdAt)}
                      </div>
                    </div>
                    <div className="flex shrink-0 items-center gap-2">
                      <Badge variant="default">
                        {f.kind ? FAVORITE_KIND_LABEL[f.kind] : "unknown"}
                      </Badge>
                      {path && (
                        <ArrowRight className="h-4 w-4 text-muted-foreground" />
                      )}
                    </div>
                  </div>
                );
                return (
                  <li key={f.id || `${f.kind}-${f.targetId}`}>
                    {path ? (
                      <Link
                        to={path}
                        className="block hover:bg-muted/40"
                      >
                        {inner}
                      </Link>
                    ) : (
                      <div className="opacity-70">{inner}</div>
                    )}
                  </li>
                );
              })}
            </ul>
          </Section>
        ))}

        {nextPageToken && (
          <div className="flex justify-center">
            <Button
              variant="outline"
              size="sm"
              onClick={() => void load(nextPageToken)}
              disabled={loading}
            >
              Load more
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}

function Section({
  title,
  count,
  children,
}: {
  title: string;
  count: number;
  children: ReactNode;
}) {
  return (
    <div>
      <h2 className="mb-2 text-sm font-semibold">
        {title} <span className="text-xs text-muted-foreground">({count})</span>
      </h2>
      {children}
    </div>
  );
}

function fmtTime(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(d);
}
