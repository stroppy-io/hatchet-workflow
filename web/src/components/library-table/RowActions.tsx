// Per-row controls shared by the Library tables — the kebab Actions (⋯) menu
// and the favorite star — mirroring the Test Runs table (src/pages/Runs.tsx)
// one-to-one. Both stopPropagation so they never trigger row navigation, and
// the menu's open-state is CONTROLLED by the page (keyed by row id) so a
// refetch / re-render can't close it mid-interaction.

import type { LucideIcon } from "lucide-react";
import { MoreHorizontal, Star } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

/** One entry in a row's Actions menu, generic over the page's action union. */
export interface RowActionItem<A extends string> {
  action: A;
  label: string;
  icon: LucideIcon;
  /** Destructive items get the red treatment. */
  danger?: boolean;
  /** Disabled items render greyed with this reason as their title. */
  disabled?: boolean;
  disabledReason?: string;
}

/**
 * RowActionsMenu — the compact "⋯" kebab cell. Renders EVERY action; invalid
 * ones are DISABLED (greyed, with a reason) rather than hidden, exactly like the
 * Runs table. Controlled-open so the parent can pause auto-refresh while open.
 */
export function RowActionsMenu<A extends string>({
  title,
  items,
  open,
  onOpenChange,
  onAction,
  ariaLabel = "Row actions",
}: {
  /** Menu header label (the row name / id). */
  title: string;
  items: RowActionItem<A>[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAction: (action: A) => void;
  ariaLabel?: string;
}) {
  return (
    <DropdownMenu open={open} onOpenChange={onOpenChange}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          onClick={(e) => e.stopPropagation()}
          className={cn(
            "flex h-6 w-6 items-center justify-center rounded transition-colors cursor-pointer",
            open
              ? "text-zinc-200 bg-zinc-800"
              : "text-zinc-600 hover:text-zinc-300 hover:bg-zinc-800/60",
          )}
          title={ariaLabel}
          aria-label={ariaLabel}
        >
          <MoreHorizontal className="h-4 w-4" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        onClick={(e) => e.stopPropagation()}
        className="min-w-[12rem] bg-zinc-950 border-zinc-800"
      >
        <DropdownMenuLabel className="truncate" title={title}>
          {title}
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        {items.map((item) => (
          <DropdownMenuItem
            key={item.action}
            disabled={item.disabled}
            title={item.disabled ? item.disabledReason : undefined}
            onSelect={(e) => {
              e.preventDefault();
              if (!item.disabled) onAction(item.action);
            }}
            className={cn(
              item.danger &&
                "text-destructive focus:text-destructive focus:bg-destructive/10",
            )}
          >
            <item.icon className="h-3.5 w-3.5 shrink-0" />
            {item.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/**
 * FavoriteStar — a clickable star that toggles the caller's favorite relation
 * (via FavoriteService through the provider). Filled + warning-tinted when
 * favorited, dim otherwise. stopPropagation so it never triggers row navigation.
 */
export function FavoriteStar({
  favorite,
  onToggle,
}: {
  favorite: boolean;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      onClick={(e) => {
        e.stopPropagation();
        onToggle();
      }}
      className={cn(
        "flex h-6 w-6 items-center justify-center rounded transition-colors cursor-pointer",
        favorite
          ? "text-warning"
          : "text-zinc-600 hover:bg-zinc-800/60 hover:text-warning",
      )}
      title={favorite ? "Remove from favorites" : "Add to favorites"}
      aria-label={favorite ? "Remove from favorites" : "Add to favorites"}
      aria-pressed={favorite}
    >
      <Star className="h-3.5 w-3.5" fill={favorite ? "currentColor" : "none"} />
    </button>
  );
}

/**
 * FavoritesOnlyToggle — the inline checkbox above the table that drives the
 * `filter.favorites_only` server-side filter (URL-state ?fav=1), matching the
 * Runs toolbar checkbox styling.
 */
export function FavoritesOnlyToggle({
  checked,
  onChange,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onChange(!checked)}
      title="Show only presets you've favorited"
      aria-pressed={checked}
      className={cn(
        "flex h-5 items-center gap-1.5 text-[10px] font-mono transition-colors cursor-pointer",
        checked ? "text-warning" : "text-zinc-500 hover:text-zinc-300",
      )}
    >
      <Star className="h-3.5 w-3.5" fill={checked ? "currentColor" : "none"} />
      Favorites only
    </button>
  );
}
