import { Fragment } from "react";
import { Link } from "react-router-dom";
import {
  Breadcrumb,
  BreadcrumbList,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { useBreadcrumbs } from "@/lib/breadcrumbs";
import { OrgSwitcherCrumb } from "@/components/header/OrgSwitcherCrumb";

// Header breadcrumb trail. Labels come from the route-meta registry in
// `@/lib/breadcrumbs`; pages may contribute dynamic labels for `:param`
// segments via `useBreadcrumbLabel`. Crumbs carry full absolute hrefs, so we
// use the raw react-router Link (no tenant-slug prefixing needed).
//
// Variant D: the leading tenant crumb (kind === "org") renders as the inline
// org-switcher dropdown rather than a plain link/label. Because that crumb is
// itself interactive (it switches orgs), the trail renders even when it is the
// only crumb — on a non-org single crumb (e.g. a bare top-level page) there is
// no wayfinding to add, so we still hide it.
export function Breadcrumbs() {
  const crumbs = useBreadcrumbs();

  const hasSwitcher = crumbs.some((c) => c.kind === "org");
  // A single, non-switcher crumb adds nothing the header doesn't already convey.
  if (crumbs.length < 2 && !hasSwitcher) return null;

  return (
    <>
      <span className="shrink-0 text-border">/</span>
      <Breadcrumb className="min-w-0 overflow-hidden">
        <BreadcrumbList className="flex-nowrap">
          {crumbs.map((crumb, i) => {
            const isLast = i === crumbs.length - 1;
            return (
              <Fragment key={`${crumb.href ?? crumb.label}-${i}`}>
                <BreadcrumbItem>
                  {crumb.kind === "org" && crumb.orgSlug ? (
                    <OrgSwitcherCrumb activeSlug={crumb.orgSlug} />
                  ) : isLast ? (
                    <BreadcrumbPage className="max-w-[16rem] truncate">
                      {crumb.label}
                    </BreadcrumbPage>
                  ) : crumb.href ? (
                    <BreadcrumbLink asChild>
                      <Link to={crumb.href} className="max-w-[16rem] truncate">
                        {crumb.label}
                      </Link>
                    </BreadcrumbLink>
                  ) : (
                    // Non-clickable grouping crumb (e.g. "Platform").
                    <span className="max-w-[16rem] truncate">{crumb.label}</span>
                  )}
                </BreadcrumbItem>
                {!isLast && <BreadcrumbSeparator />}
              </Fragment>
            );
          })}
        </BreadcrumbList>
      </Breadcrumb>
    </>
  );
}
