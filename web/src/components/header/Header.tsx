import { Activity } from "lucide-react";
import { Link } from "react-router-dom";
import { UserMenu } from "@/components/header/UserMenu";
import { Breadcrumbs } from "@/components/header/Breadcrumbs";

// Top header bar. Left: brand + the slug-aware breadcrumb trail, whose FIRST
// crumb is the inline org switcher (Variant D). Right: the account menu (which
// also hosts Organizations and, for platform admins, the admin group).
export function Header() {
  return (
    <header className="flex h-12 shrink-0 items-center justify-between border-b border-border bg-[#080808] px-3">
      <div className="flex min-w-0 items-center gap-3">
        <Link
          to="/"
          className="flex shrink-0 items-center gap-2 text-foreground transition-opacity hover:opacity-80"
        >
          <Activity className="h-4 w-4 text-primary" />
          <span className="text-sm font-semibold tracking-tight">
            stroppy-cloud
          </span>
        </Link>
        <Breadcrumbs />
      </div>

      <div className="flex items-center gap-1.5">
        <UserMenu />
      </div>
    </header>
  );
}
