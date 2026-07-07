// Recipes — the list page for RecipeService.ListRecipes (persisted DSL recipe
// bundles). Mirrors the dark-theme "tenant working page" chrome shared by
// Suites.tsx / Runs.tsx (h1 + New button header row, p-5 page padding) and the
// Card + shadcn Table body used by the account-scope list pages (Orgs.tsx /
// AdminAccounts.tsx), since this list has no filter/sort/paging surface to
// justify the heavier tanstack-table + LibraryTable machinery those pages use.
//
// Row -> RecipeVM (services/recipe.ts): name links to the detail page,
// version/provider/compiles/machine-group+service counts are plain columns,
// Actions carries Open (navigate) + Delete (confirm-dialog -> deleteRecipe ->
// refetch). "New Recipe" is gated to operator+ (roleLevel), matching the
// authoring-affordance gate Packages.tsx / TenantSidebar apply to create/mutate
// actions.

import { useCallback, useEffect, useState } from "react";
import { AlertCircle, Loader2, Plus, Star, Trash2 } from "lucide-react";
import { Link, useNavigate, useTenantSlug } from "@/lib/router";
import { useAuth } from "@/hooks/useAuth";
import { roleLevel } from "@/lib/roles";
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
import { deleteRecipe, listRecipes, type RecipeVM } from "@/services/recipe";
import { addFavorite, listFavorites, removeFavorite } from "@/services/favorites";
import { cn } from "@/lib/utils";

export function Recipes() {
  const slug = useTenantSlug() ?? "";
  const navigate = useNavigate();
  const { user } = useAuth();
  const confirm = useConfirm();

  const [recipes, setRecipes] = useState<RecipeVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [favoriteIds, setFavoriteIds] = useState<Set<string>>(new Set());
  const [togglingId, setTogglingId] = useState<string | null>(null);

  // operator+ (>=2) on the active tenant, or platform admin, may create/delete.
  const tenant = user?.tenants.find((t) => t.slug === slug);
  const level = user?.isAdmin ? 99 : tenant ? roleLevel[tenant.role] : 0;
  const canMutate = level >= roleLevel.operator;

  const fetchRecipes = useCallback(async () => {
    if (!slug) return;
    setLoading(true);
    setError(null);
    try {
      const rows = await listRecipes(slug);
      setRecipes(rows);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load recipes");
      setRecipes([]);
    } finally {
      setLoading(false);
    }
  }, [slug]);

  useEffect(() => {
    if (!slug) return;
    let cancelled = false;
    void listFavorites(slug, { kind: "recipe" })
      .then((page) => {
        if (!cancelled) setFavoriteIds(new Set(page.favorites.map((f) => f.targetId)));
      })
      .catch(() => {
        // favorites are a non-critical affordance — leave the star row empty on error.
      });
    return () => {
      cancelled = true;
    };
  }, [slug]);

  const handleToggleFavorite = useCallback(
    async (recipe: RecipeVM) => {
      if (!slug) return;
      const isFavorite = favoriteIds.has(recipe.id);
      setTogglingId(recipe.id);
      try {
        if (isFavorite) {
          await removeFavorite(slug, "recipe", recipe.id);
          setFavoriteIds((prev) => {
            const next = new Set(prev);
            next.delete(recipe.id);
            return next;
          });
        } else {
          await addFavorite(slug, "recipe", recipe.id);
          setFavoriteIds((prev) => new Set(prev).add(recipe.id));
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to update favorite");
      } finally {
        setTogglingId(null);
      }
    },
    [slug, favoriteIds],
  );

  useEffect(() => {
    void fetchRecipes();
  }, [fetchRecipes]);

  const handleDelete = useCallback(
    async (recipe: RecipeVM) => {
      if (!slug) return;
      const ok = await confirm({
        title: "Delete recipe?",
        description: `"${recipe.name || recipe.id}" will be permanently removed. This cannot be undone.`,
        danger: true,
        confirmLabel: "Delete",
      });
      if (!ok) return;
      setDeletingId(recipe.id);
      setError(null);
      try {
        await deleteRecipe(slug, recipe.id);
        await fetchRecipes();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to delete recipe");
      } finally {
        setDeletingId(null);
      }
    },
    [slug, confirm, fetchRecipes],
  );

  return (
    <div className="p-5 flex flex-col gap-4 h-full min-h-0">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-base font-semibold font-mono tracking-tight">
          Recipes
        </h1>
        {canMutate && (
          <Button size="sm" onClick={() => navigate("/recipes/new")}>
            <Plus className="h-3.5 w-3.5" />
            New Recipe
          </Button>
        )}
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
                <TableHead>Version</TableHead>
                <TableHead>Provider</TableHead>
                <TableHead className="text-center">Compiles</TableHead>
                <TableHead className="text-center">Machine groups</TableHead>
                <TableHead className="text-center">Services</TableHead>
                <TableHead className="w-[112px] text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={7} className="h-32 text-center">
                    <span className="inline-flex items-center gap-2 text-xs text-muted-foreground font-mono">
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      Loading recipes...
                    </span>
                  </TableCell>
                </TableRow>
              ) : recipes.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} className="h-32 text-center">
                    <span className="text-xs text-muted-foreground font-mono">
                      No recipes yet — create one
                    </span>
                  </TableCell>
                </TableRow>
              ) : (
                recipes.map((recipe) => (
                  <TableRow
                    key={recipe.id}
                    className="cursor-pointer"
                    onClick={() => navigate(`/recipes/${recipe.id}`)}
                  >
                    <TableCell>
                      <Link
                        to={`/recipes/${recipe.id}`}
                        onClick={(e) => e.stopPropagation()}
                        className="text-xs text-primary hover:underline underline-offset-2"
                      >
                        {recipe.name || recipe.id}
                      </Link>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      v{recipe.version}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {recipe.provider || "—"}
                    </TableCell>
                    <TableCell className="text-center">
                      {recipe.compiles ? (
                        <Badge variant="success">ok</Badge>
                      ) : (
                        <Badge variant="destructive">errors</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-center font-mono text-xs text-muted-foreground tabular-nums">
                      {recipe.machineGroupCount}
                    </TableCell>
                    <TableCell className="text-center font-mono text-xs text-muted-foreground tabular-nums">
                      {recipe.serviceCount}
                    </TableCell>
                    <TableCell onClick={(e) => e.stopPropagation()}>
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          disabled={togglingId === recipe.id}
                          onClick={() => void handleToggleFavorite(recipe)}
                          title={
                            favoriteIds.has(recipe.id)
                              ? "Remove from favorites"
                              : "Add to favorites"
                          }
                        >
                          {togglingId === recipe.id ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : (
                            <Star
                              className={cn(
                                "h-4 w-4",
                                favoriteIds.has(recipe.id) &&
                                  "fill-yellow-400 text-yellow-400",
                              )}
                            />
                          )}
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => navigate(`/recipes/${recipe.id}`)}
                        >
                          Open
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          disabled={deletingId === recipe.id}
                          onClick={() => void handleDelete(recipe)}
                          title="Delete recipe"
                          className="text-destructive hover:text-destructive"
                        >
                          {deletingId === recipe.id ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : (
                            <Trash2 className="h-4 w-4" />
                          )}
                        </Button>
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
