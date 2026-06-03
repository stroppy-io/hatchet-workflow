// Shared per-row wiring for the three preset Library tables (Database / Workload
// / Test). Each page differs only in its PresetKind, so the favorite toggle, the
// Actions (⋯) menu items + gating, and the handler all live here, parameterised
// by kind. Mirrors the Test Runs table's favorite + actions UX.
//
// GATING (per the proto + the sidebar's role-gating in lib/roles.ts):
//   * Edit / Delete   — system presets (is_system) are platform-seeded and
//                       read-only, so both are DISABLED with a reason. Delete
//                       also requires operator+ (destructive).
//   * Duplicate / Use — always available (Clone works on any preset; Use just
//                       seeds the wizard).
//   * View            — always (pure navigation).

import { useCallback, useMemo, useState } from "react";
import { Copy, Eye, PlayCircle, Pencil, Trash2 } from "lucide-react";
import type { ColumnDef } from "@tanstack/react-table";
import { useNavigate } from "@/lib/router";
import { useAuth } from "@/hooks/useAuth";
import { roleLevel } from "@/lib/roles";
import { useConfirm } from "@/components/ui/confirm-dialog";
import {
  getPresetProvider,
  type PresetAction,
  type PresetKind,
} from "@/services/preset";
import {
  FavoriteStar,
  RowActionsMenu,
  type RowActionItem,
} from "@/components/library-table/RowActions";

/** Minimal row shape the shared actions/favorite wiring needs. */
export interface PresetRowLike {
  id: string;
  name: string;
  isSystem: boolean;
  isFavorite: boolean;
}

/** Per-kind in-app route the "View" action navigates to. */
const KIND_ROUTE: Record<PresetKind, string> = {
  database: "/presets/database",
  workload: "/presets/workload",
  test: "/presets/test",
};

export function usePresetRowActions<T extends PresetRowLike>({
  slug,
  kind,
  favoritesOnly,
  rows,
  setRows,
  refetch,
  setError,
}: {
  slug: string | undefined;
  kind: PresetKind;
  favoritesOnly: boolean;
  rows: T[];
  setRows: React.Dispatch<React.SetStateAction<T[]>>;
  /** Background refetch to reconcile after a mutation (clone / delete). */
  refetch: () => Promise<void>;
  setError: (msg: string | null) => void;
}) {
  const navigate = useNavigate();
  const confirm = useConfirm();
  const { user } = useAuth();

  // operator+ (>=2) on the active tenant, or platform admin, may mutate.
  const tenant = user?.tenants.find((t) => t.slug === slug);
  const level = user?.isAdmin ? 99 : tenant ? roleLevel[tenant.role] : 0;
  const canMutate = level >= roleLevel.operator;

  // Which row's Actions (⋯) menu is open (null = none) — lifted here so a
  // refetch can't reset it; the page pauses auto-refresh (if any) on this.
  const [openActionId, setOpenActionId] = useState<string | null>(null);

  const toggleFavorite = useCallback(
    async (row: T) => {
      if (!slug) return;
      const next = !row.isFavorite;
      try {
        await getPresetProvider().setPresetFavorite(slug, kind, row.id, next);
        setRows((current) => {
          const mapped = current.map((r) =>
            r.id === row.id ? { ...r, isFavorite: next } : r,
          );
          // When the favorites-only filter is on, un-favoriting drops the row.
          return favoritesOnly && !next
            ? mapped.filter((r) => r.id !== row.id)
            : mapped;
        });
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to update favorite");
      }
    },
    [slug, kind, favoritesOnly, setRows, setError],
  );

  const runAction = useCallback(
    async (action: PresetAction, row: T) => {
      if (!slug) return;
      setOpenActionId(null);
      const provider = getPresetProvider();
      try {
        switch (action) {
          case "view":
            navigate(KIND_ROUTE[kind]);
            return;
          case "use":
            navigate(`/runs/new?preset=${encodeURIComponent(row.id)}&kind=${kind}`);
            return;
          case "edit":
            // Update<Kind>Preset — route to the authoring surface seeded with
            // this preset (the editor consumes ?edit=:id). System presets are
            // gated out before we get here.
            navigate(`${KIND_ROUTE[kind]}?edit=${encodeURIComponent(row.id)}`);
            return;
          case "duplicate": {
            await provider.clonePreset(slug, kind, row.id, `${row.name} (copy)`);
            await refetch();
            return;
          }
          case "delete": {
            const ok = await confirm({
              title: "Delete preset?",
              description: `“${row.name || row.id}” will be permanently removed. This cannot be undone.`,
              danger: true,
              confirmLabel: "Delete",
            });
            if (!ok) return;
            await provider.deletePreset(slug, kind, row.id);
            setRows((current) => current.filter((r) => r.id !== row.id));
            await refetch();
            return;
          }
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : `Failed to ${action} preset`);
      }
    },
    [slug, kind, navigate, confirm, refetch, setRows, setError],
  );

  // The Actions menu items for one row, with system + role gating applied.
  const itemsFor = useCallback(
    (row: T): RowActionItem<PresetAction>[] => {
      const systemReason = "System presets are platform-seeded and read-only";
      const roleReason = "Requires the operator role or higher";
      return [
        { action: "view", label: "View detail", icon: Eye },
        { action: "use", label: "Use in new run", icon: PlayCircle },
        {
          action: "edit",
          label: "Edit",
          icon: Pencil,
          disabled: row.isSystem || !canMutate,
          disabledReason: row.isSystem ? systemReason : roleReason,
        },
        { action: "duplicate", label: "Duplicate", icon: Copy },
        {
          action: "delete",
          label: "Delete",
          icon: Trash2,
          danger: true,
          disabled: row.isSystem || !canMutate,
          disabledReason: row.isSystem ? systemReason : roleReason,
        },
      ];
    },
    [canMutate],
  );

  // The trailing Actions column (⋯ menu + favorite star), shared by all three
  // preset pages. Place it last in each page's column list.
  const actionsColumn = useMemo<ColumnDef<T>>(
    () => ({
      id: "actions",
      enableSorting: false,
      meta: { className: "px-1", disableRowNavigation: true },
      header: () => (
        <span className="block text-center whitespace-nowrap leading-none">
          Actions
        </span>
      ),
      cell: ({ row }) => {
        const r = row.original;
        return (
          <div className="flex h-full items-center justify-center gap-2">
            <RowActionsMenu
              title={r.name || r.id}
              items={itemsFor(r)}
              open={openActionId === r.id}
              onOpenChange={(o) => setOpenActionId(o ? r.id : null)}
              onAction={(action) => void runAction(action, r)}
              ariaLabel="Preset actions"
            />
            <FavoriteStar
              favorite={r.isFavorite}
              onToggle={() => void toggleFavorite(r)}
            />
          </div>
        );
      },
    }),
    [openActionId, itemsFor, runAction, toggleFavorite],
  );

  // True while a menu is open — the page uses this to know an interaction is in
  // flight (parity with the Runs "pause auto-refresh while open" behaviour).
  const actionMenuOpen = openActionId !== null;

  return { actionsColumn, actionMenuOpen };
}
