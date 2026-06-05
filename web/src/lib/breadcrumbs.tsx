// Breadcrumb mechanism for the (non-data) `<Routes>` shell.
//
// Single source of truth for crumb labels lives in `CRUMB_REGISTRY`: an ordered
// list of path patterns (react-router-style, with `:params`) mapped to a static
// label. The active URL is matched against the registry to assemble the trail.
//
// Dynamic segments (`:slug`, `:id`, ...) resolve to a human label when a page
// supplies one via `useBreadcrumbLabel(param, label)`; otherwise they fall back
// to the raw param value. This keeps the registry declarative and lets future
// pages contribute names (suite name, run name, ...) without touching routing.
//
// Adding a page = add one `{ pattern, label }` entry here. Done.
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { matchPath, useLocation, useParams } from "react-router-dom";

/** A resolved crumb ready to render. */
export interface Crumb {
  /** Display label. */
  label: string;
  /**
   * Absolute path to link to. Undefined for the current (last) crumb and for
   * non-clickable grouping crumbs (e.g. "Admin"); render those as plain text.
   */
  href?: string;
  /**
   * "org" marks the leading tenant crumb so the renderer swaps in the inline
   * org-switcher dropdown (Variant D: the first crumb IS the org switcher).
   * Carries the active org slug so the switcher knows what's selected.
   */
  kind?: "org";
  /** Active org slug, present when `kind === "org"`. */
  orgSlug?: string;
}

interface CrumbDef {
  /** react-router path pattern, e.g. "/t/:slug/suites/:id". */
  pattern: string;
  /**
   * Crumb label. A string is static. A function receives the matched params and
   * the dynamic-label overrides and returns the label (used for the leaf when a
   * `:param` should display a resolved name rather than the segment word).
   */
  label: string | ((ctx: CrumbLabelContext) => string);
  /**
   * Link target. By default a crumb links to the portion of the URL its pattern
   * matched. `null` renders a non-clickable grouping crumb (e.g. an "Admin" root
   * that has no single landing page). A string overrides the link target.
   */
  href?: string | null;
  /**
   * Crumb rendering kind. "org" turns the leading `/t/:slug` crumb into the
   * inline org-switcher dropdown. Omitted = a normal text/link crumb.
   */
  kind?: "org";
}

interface CrumbLabelContext {
  params: Readonly<Record<string, string | undefined>>;
  /** Overrides registered by pages via `useBreadcrumbLabel`, keyed by param. */
  overrides: Readonly<Record<string, string>>;
}

// Ordered shortest-prefix-first so the assembled trail reads root -> leaf.
// Each entry is one crumb; the URL contributes a crumb for every pattern it
// matches as a prefix. `slug` resolves to the human tenant name when available.
const CRUMB_REGISTRY: CrumbDef[] = [
  // Tenant root. Variant D: the leading crumb IS the org switcher. Its label is
  // the current org name (override) else the slug; the renderer turns it into a
  // dropdown that lists the user's orgs.
  {
    pattern: "/t/:slug",
    label: ({ params, overrides }) => overrides.slug ?? params.slug ?? "",
    kind: "org",
  },
  // Work entities only — org Settings/Members/Tokens moved to /orgs/:slug.
  { pattern: "/t/:slug/suites", label: "Suites" },
  // `:id` registered BEFORE `new` so the more specific "New suite" literal wins
  // the dedup (same convention as the preset crumbs below).
  {
    pattern: "/t/:slug/suites/:id",
    label: ({ params, overrides }) => overrides.id ?? params.id ?? "",
  },
  { pattern: "/t/:slug/suites/new", label: "New Suite" },
  { pattern: "/t/:slug/runs", label: "Test Runs" },
  { pattern: "/t/:slug/runs/new", label: "New Run" },
  {
    pattern: "/t/:slug/runs/:id",
    label: ({ params, overrides }) => overrides.id ?? params.id ?? "",
  },
  { pattern: "/t/:slug/compare", label: "Compare" },
  // Library — the preset tables share a "/presets" grouping crumb (which itself
  // redirects to the Database tab) + a leaf per tab; Packages is a sibling.
  { pattern: "/t/:slug/presets", label: "Library" },
  { pattern: "/t/:slug/presets/database", label: "Database Presets" },
  // `:id` is registered BEFORE `new` so that on the /new route (where `:id`
  // would also match the literal "new") the more specific "New preset" crumb
  // wins the dedup by appearing later in registry order.
  {
    pattern: "/t/:slug/presets/database/:id",
    label: ({ params, overrides }) => overrides.id ?? params.id ?? "",
  },
  { pattern: "/t/:slug/presets/database/new", label: "New preset" },
  { pattern: "/t/:slug/presets/database/:id/edit", label: "Edit" },
  { pattern: "/t/:slug/presets/workload", label: "Workload Presets" },
  // `:id` registered BEFORE `new`/`edit` so the more specific literal crumbs win
  // dedup (same convention as the database preset crumbs above).
  {
    pattern: "/t/:slug/presets/workload/:id",
    label: ({ params, overrides }) => overrides.id ?? params.id ?? "",
  },
  { pattern: "/t/:slug/presets/workload/new", label: "New preset" },
  { pattern: "/t/:slug/presets/workload/:id/edit", label: "Edit" },
  { pattern: "/t/:slug/presets/test", label: "Test Presets" },
  {
    pattern: "/t/:slug/presets/test/:id",
    label: ({ params, overrides }) => overrides.id ?? params.id ?? "",
  },
  { pattern: "/t/:slug/presets/test/:id/edit", label: "Edit" },
  { pattern: "/t/:slug/packages", label: "Packages" },
  { pattern: "/t/:slug/packages/new", label: "Upload package" },
  {
    pattern: "/t/:slug/packages/:id",
    label: ({ params, overrides }) => overrides.id ?? params.id ?? "",
  },

  // Outside-tenant areas. The first crumb here is a plain label (no switcher).
  { pattern: "/profile", label: "Profile" },
  // Organizations area (reached from the account menu).
  { pattern: "/orgs", label: "Organizations" },
  {
    pattern: "/orgs/:slug",
    label: ({ params, overrides }) => overrides.slug ?? params.slug ?? "",
  },
  // Platform-admin area. Grouping crumb + leaves.
  { pattern: "/admin/*", label: "Admin", href: null },
  { pattern: "/admin/accounts", label: "Accounts" },
  { pattern: "/admin/registration-requests", label: "Registration requests" },
  { pattern: "/admin/identity-providers", label: "Identity providers" },
  { pattern: "/admin/system", label: "System settings" },
];

// --- dynamic-label override context ----------------------------------------

type Overrides = Record<string, string>;
interface BreadcrumbCtx {
  overrides: Overrides;
  set: (param: string, label: string | undefined) => void;
}

const BreadcrumbContext = createContext<BreadcrumbCtx | null>(null);

/** Wraps the shell so pages can register dynamic crumb labels. */
export function BreadcrumbProvider({ children }: { children: ReactNode }) {
  const [overrides, setOverrides] = useState<Overrides>({});
  const ctx = useMemo<BreadcrumbCtx>(
    () => ({
      overrides,
      set: (param, label) =>
        setOverrides((prev) => {
          if (label === undefined) {
            if (!(param in prev)) return prev;
            const next = { ...prev };
            delete next[param];
            return next;
          }
          if (prev[param] === label) return prev;
          return { ...prev, [param]: label };
        }),
    }),
    [overrides],
  );
  return (
    <BreadcrumbContext.Provider value={ctx}>
      {children}
    </BreadcrumbContext.Provider>
  );
}

/**
 * Register a human label for a dynamic URL segment from within a page, e.g.
 * `useBreadcrumbLabel("id", suite?.name)` on a suite-detail page makes the
 * breadcrumb read `acme / Suites / My Suite` instead of the raw id. Passing
 * undefined (still loading) leaves the raw param showing. Cleared on unmount.
 */
export function useBreadcrumbLabel(param: string, label: string | undefined) {
  const ctx = useContext(BreadcrumbContext);
  const setRef = useRef(ctx?.set);
  setRef.current = ctx?.set;
  useEffect(() => {
    const set = setRef.current;
    if (!set) return;
    set(param, label);
    return () => set(param, undefined);
  }, [param, label]);
}

// --- trail assembly ----------------------------------------------------------

function resolveLabel(def: CrumbDef, ctx: CrumbLabelContext): string {
  return typeof def.label === "function" ? def.label(ctx) : def.label;
}

/**
 * Build the breadcrumb trail for the current location from the registry.
 * Every registry pattern that the current path matches as a prefix contributes
 * one crumb, in registry order; the last is marked current (no href).
 */
export function useBreadcrumbs(): Crumb[] {
  const location = useLocation();
  const params = useParams();
  const ctx = useContext(BreadcrumbContext);
  const overrides = ctx?.overrides ?? {};

  return useMemo(() => {
    const pathname = location.pathname.replace(/\/+$/, "") || "/";
    const labelCtx: CrumbLabelContext = { params, overrides };

    // Track crumbs by a dedup key so two patterns matching the same URL portion
    // (e.g. tenant root + an index page) collapse into one, most-specific label.
    const out: {
      key: string;
      label: string;
      href?: string;
      kind?: "org";
      orgSlug?: string;
    }[] = [];
    for (const def of CRUMB_REGISTRY) {
      const m = matchPath({ path: def.pattern, end: false }, pathname);
      if (!m) continue;
      const label = resolveLabel(def, labelCtx);
      if (!label) continue;
      // href === null -> non-clickable grouping crumb (keyed by its pattern so
      // it stays distinct from sibling page crumbs).
      const isLink = def.href !== null;
      const matched = (m.pathnameBase || "/").replace(/\/+$/, "") || "/";
      const href = isLink ? (def.href ?? matched) : undefined;
      const key = isLink ? href! : `__group:${def.pattern}`;
      const existing = out.find((c) => c.key === key);
      const orgSlug = def.kind === "org" ? m.params.slug : undefined;
      if (existing) {
        existing.label = label;
        if (def.kind) existing.kind = def.kind;
        if (orgSlug) existing.orgSlug = orgSlug;
      } else {
        out.push({ key, label, href, kind: def.kind, orgSlug });
      }
    }

    const crumbs: Crumb[] = out.map(({ label, href, kind, orgSlug }) => ({
      label,
      href,
      kind,
      orgSlug,
    }));
    // Last crumb is the current page: drop its href so it renders as current —
    // EXCEPT the org-switcher crumb, which stays interactive even when it is the
    // only crumb (the org dashboard root), since it is a switcher, not a link.
    if (crumbs.length > 0) {
      const last = crumbs[crumbs.length - 1];
      if (last.kind !== "org") {
        crumbs[crumbs.length - 1] = { label: last.label };
      }
    }
    return crumbs;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.pathname, JSON.stringify(params), JSON.stringify(overrides)]);
}
