// ide.ts — the browser-auth half of the "Open in IDE" handshake (see
// internal/ide/ticket.go's package doc). A plain browser navigation to
// /ide/<scope>/... cannot carry the SPA's Bearer access token (navigation
// cannot set custom headers), so instead: an authenticated fetch (which CAN
// carry the header) mints a short-lived single-use ticket bound to the
// target scope, and the browser then navigates with that ticket in the
// query string. The gateway exchanges the ticket for an httpOnly
// scope-bound session cookie and redirects to the clean URL — see
// internal/app/ide_ticket_handler.go for the server side of this call.

import { getAccessToken } from "@/services/client";

const rawBaseUrl = (import.meta.env.VITE_API_BASE_URL as string | undefined) || "/";
const baseUrl = rawBaseUrl.endsWith("/") ? rawBaseUrl : `${rawBaseUrl}/`;

export interface IdeTicket {
  ticket: string;
  /** The clean "/ide/<scope>/...?ticket=..." URL to navigate (a new tab, or
   * an iframe's src) to. Relative — never carries a scheme/host, so it is
   * always same-origin with the SPA (see internal/app/ide_ticket_handler.go). */
  url: string;
}

// mintIdeTicket calls GET /api/ide/ticket?target=<targetUrl> with the SPA's
// Bearer access token (the authenticated half of the browser-auth handshake
// — see internal/ide/ticket.go's package doc) and returns the single-use
// ticket-bearing URL the gateway will exchange for an httpOnly session
// cookie on the next navigation to it. Throws on: not signed in, 400 (bad
// target), 403 (not authorized for this scope), or a disabled IDE manager
// (the endpoint 404s when IDE_MANAGER_ENABLED is off — see
// internal/app/run.go's ideTicketHandler wiring). Callers distinguish these
// by inspecting the thrown Error's message for the status code, mirroring
// the server's own status-code-is-the-contract shape.
export async function mintIdeTicket(targetUrl: string): Promise<IdeTicket> {
  const token = getAccessToken();
  if (!token) {
    throw new Error("not signed in");
  }
  const res = await fetch(`${baseUrl}api/ide/ticket?target=${encodeURIComponent(targetUrl)}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!res.ok) {
    throw new Error(`could not open IDE: ${res.status} ${res.statusText}`);
  }
  return (await res.json()) as IdeTicket;
}

// openInIde mints a ticket for targetUrl (an "/ide/<scope>/..." path, as
// built by orgIdeUrl/instanceIdeUrl) and navigates a new browser tab to the
// ticket-bearing URL the server returns. It never opens targetUrl directly
// — doing so would 403 (no Authorization header on a plain navigation),
// which is exactly the bug this handshake exists to fix.
export async function openInIde(targetUrl: string): Promise<void> {
  const { url } = await mintIdeTicket(targetUrl);
  window.open(url, "_blank", "noreferrer");
}

/** The catalog-entry/recipe list tabs — matches AdminCatalog/OrgCatalog's
 * tab param, always plural (it also names the on-disk providers/<slug> |
 * workflows/<slug> | recipes/<name> folder path below). "recipes" is
 * ORG-ONLY (see internal/ide.EntryKindRecipe's doc — a recipe has no
 * LEVEL_INSTANCE equivalent): instanceIdeUrl's tab param is typed to
 * exclude it, so a "recipes" scope can never even be constructed as an
 * instance URL — the singular/plural bug this file already guards against
 * (see entryKindSegment's doc) plus a compile-time guard against the newer
 * "recipe has no instance level" invariant. */
export type IdeEntryTab = "providers" | "workflows" | "recipes";

// entryKindSegment maps the plural list-tab name to the singular scope
// segment internal/ide.ParseScope requires (EntryKindProvider/
// EntryKindWorkflow/EntryKindRecipe — "provider"/"workflow"/"recipe", never
// "providers"/"workflows"/"recipes"). This is the one place that
// translation happens; every URL builder below funnels through it so a
// future kind can't reintroduce the singular/plural mismatch bug fixed in
// commit 09f390e4 (a prior version of these builders passed the plural tab
// name straight into the scope segment and 404'd at the gateway).
function entryKindSegment(tab: IdeEntryTab): "provider" | "workflow" | "recipe" {
  if (tab === "providers") return "provider";
  if (tab === "workflows") return "workflow";
  return "recipe";
}

// workspaceRoot is where Manager mounts a WORKSPACE (the instance catalog,
// or one tenant) inside its single code-server container — one container per
// workspace, not per entry (internal/ide.Scope.WorkspaceKey). Each entry's
// own git repo is cloned side by side underneath it, so the folder deep link
// below is what actually opens the entry the user asked for.
const workspaceRoot = "/home/coder/project";

// entryFolder is the workspace-relative path an entry's repo is cloned into
// (internal/ide.Scope.EntryDir) — the plural, human-facing form.
function entryFolder(tab: IdeEntryTab, slug: string): string {
  return `${workspaceRoot}/${tab}/${slug}`;
}

// instanceIdeUrl builds the gateway /ide/instance/* URL for an entry's
// on-disk location in the instance repo, opened as a code-server "folder"
// deep link so the IDE lands on the entry's own files rather than the whole
// repo root. Requires the instance-admin IdeAuthorizer grant
// (internal/ide.Authorizer.CanAuthor) — a non-admin's request 403s at the
// gateway, same as every other LEVEL_INSTANCE write path. tab excludes
// "recipes" (see IdeEntryTab's doc — no LEVEL_INSTANCE recipe exists; the
// server-side Authorizer/Manager reject that scope shape unconditionally
// even if a caller somehow still constructed it).
export function instanceIdeUrl(tab: Exclude<IdeEntryTab, "recipes">, slug: string): string {
  const entryKind = entryKindSegment(tab);
  const folder = encodeURIComponent(entryFolder(tab, slug));
  return `/ide/instance/${entryKind}/${slug}?folder=${folder}`;
}

// orgIdeUrl builds the gateway /ide/org/<slug>/* URL for an entry's on-disk
// location in this org's own repo. Only meaningful for a NATIVE/FORKED
// catalog entry — a LINKED entry has no files of its own yet (org repos
// start empty, spec's read-through model; see internal/services/catalog.
// GitBundleStore's package doc) until it is forked, so the caller must gate
// this behind origin !== "ORIGIN_LINKED". A recipe is never LINKED (that
// origin concept does not apply to recipes at all), so recipeIdeUrl below
// reuses this same builder unconditionally.
export function orgIdeUrl(orgSlug: string, tab: IdeEntryTab, slug: string): string {
  const entryKind = entryKindSegment(tab);
  const folder = encodeURIComponent(entryFolder(tab, slug));
  return `/ide/org/${orgSlug}/${entryKind}/${slug}?folder=${folder}`;
}

// recipeIdeUrl builds the gateway /ide/org/<slug>/recipe/<name>* URL for a
// recipe's own repo — the recipe analogue of orgIdeUrl(orgSlug,
// "recipes", name). A recipe is identified by NAME (not a slug field — see
// internal/services/recipe.recipeBundleIdentity's doc: "name" plays the
// role "slug" plays for a catalog item), and is always org-scoped (there is
// no instanceIdeUrl equivalent — see EntryKindRecipe's doc).
export function recipeIdeUrl(orgSlug: string, name: string): string {
  return orgIdeUrl(orgSlug, "recipes", name);
}
