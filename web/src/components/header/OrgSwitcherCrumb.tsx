import { ChevronDown, Check, Plus } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { cn } from "@/lib/utils";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";

// Variant D org switcher — lives INSIDE the breadcrumb trail as its first crumb.
// Renders the active org slug as a dropdown trigger (`acme ▾`); the menu lists
// every org the user belongs to (slug + name, current one checked). Selecting an
// org performs a real route push to /t/<slug> so browser Back works. A footer
// link opens the full Organizations area.
export function OrgSwitcherCrumb({ activeSlug }: { activeSlug: string }) {
  const { user } = useAuth();
  const navigate = useNavigate();
  const tenants = user?.tenants ?? [];
  const active = tenants.find((t) => t.slug === activeSlug);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className={cn(
          "group inline-flex h-6 items-center gap-1 rounded-sm px-1.5 font-mono text-xs",
          "text-foreground outline-none transition-colors",
          "hover:bg-muted focus-visible:bg-muted focus-visible:ring-1 focus-visible:ring-ring",
          "data-[state=open]:bg-muted",
        )}
        aria-label="Switch organization"
      >
        <span className="max-w-[12rem] truncate">{active?.slug ?? activeSlug}</span>
        <ChevronDown className="h-3 w-3 text-muted-foreground transition-transform group-data-[state=open]:rotate-180" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-[16rem]">
        <DropdownMenuLabel>Organizations</DropdownMenuLabel>
        {tenants.map((t) => {
          const isActive = t.slug === activeSlug;
          return (
            <DropdownMenuItem
              key={t.id}
              onSelect={() => navigate(`/t/${t.slug}`)}
              className={cn(isActive && "text-foreground")}
            >
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="truncate font-mono text-xs">{t.slug}</span>
                <span className="truncate text-[10px] text-muted-foreground">
                  {t.name}
                </span>
              </span>
              {isActive && <Check className="h-3.5 w-3.5 shrink-0 text-primary" />}
            </DropdownMenuItem>
          );
        })}
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => navigate("/orgs")}>
          <Plus className="h-4 w-4" />
          <span className="text-xs">All organizations</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
