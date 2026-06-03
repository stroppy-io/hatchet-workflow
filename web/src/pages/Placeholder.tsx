import type { ReactNode } from "react";
import { useParams } from "react-router-dom";
import { useBreadcrumbLabel } from "@/lib/breadcrumbs";

// Minimal titled content placeholder. Real pages get rebuilt against the new
// API in later tasks; for now these just render the page title so navigation
// is viewable in the shell.
//
// `crumbParam` opts a dynamic page (e.g. /runs/:id) into the breadcrumb trail:
// the placeholder declares a label for that URL param so the crumb reads a
// human name instead of the raw id. Real pages will pass the loaded entity's
// name here (or call `useBreadcrumbLabel` directly).
export function Placeholder({
  title,
  subtitle,
  children,
  crumbParam,
}: {
  title: string;
  subtitle?: string;
  children?: ReactNode;
  crumbParam?: string;
}) {
  const params = useParams();
  // While there is no real entity yet, surface "<Title> <id>" as the crumb so
  // the dynamic-resolution path is exercised; pages later pass the real name.
  const paramValue = crumbParam ? params[crumbParam] : undefined;
  useBreadcrumbLabel(
    crumbParam ?? "__none",
    crumbParam && paramValue ? `${title} ${paramValue}` : undefined,
  );

  return (
    <div className="p-8">
      <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
        Page
      </div>
      <h1 className="text-xl font-semibold tracking-tight text-foreground">
        {title}
      </h1>
      {subtitle && (
        <p className="mt-1 text-sm text-muted-foreground">{subtitle}</p>
      )}
      {children}
    </div>
  );
}
