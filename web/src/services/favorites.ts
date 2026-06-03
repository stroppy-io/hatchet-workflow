// Favorites data surface — wraps cloud.v1.api.FavoriteService.ListFavorites.
//
// ListFavorites returns the caller's raw favorite JOIN rows (FavoriteRecord),
// NOT the denormalized target entities. Each FavoriteRecord carries:
//   * entity   — storage envelope (id, author_id, tenant_id, timings)
//   * kind     — cloud.v1.common.FavoriteKind (which table the favorite targets)
//   * target_id — the favorited row's id, within `kind`
// There is no denormalized target name/snapshot on the join row, so the flat VM
// surfaces the kind label + target id + the favorite's own created_at, and the
// page builds an in-app link to the target's detail route from (kind, targetId).

import { toJson } from "@bufbuild/protobuf";
import {
  ListFavoritesResponseSchema,
} from "@/lib/proto/cloud/v1/api/favorite_pb";
import { FavoriteKind } from "@/lib/proto/cloud/v1/common/favorite_pb";
import { favoriteClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";

/** The favoritable kinds, as flat lower-case labels (UNSPECIFIED collapsed). */
export type FavoriteKindLabel =
  | "database_preset"
  | "workload_preset"
  | "test_preset"
  | "test_run"
  | "suite"
  | "suite_run";

/** JSON-string enum form -> flat label ("" for unspecified/unknown). */
function favoriteKindLabelFromJson(s: string | undefined): FavoriteKindLabel | "" {
  switch (s) {
    case "FAVORITE_KIND_DATABASE_PRESET":
      return "database_preset";
    case "FAVORITE_KIND_WORKLOAD_PRESET":
      return "workload_preset";
    case "FAVORITE_KIND_TEST_PRESET":
      return "test_preset";
    case "FAVORITE_KIND_TEST_RUN":
      return "test_run";
    case "FAVORITE_KIND_SUITE":
      return "suite";
    case "FAVORITE_KIND_SUITE_RUN":
      return "suite_run";
    default:
      return "";
  }
}

/** Map a flat label to the numeric proto enum (for the kind narrowing filter). */
export function favoriteKindProto(label: FavoriteKindLabel | ""): FavoriteKind {
  switch (label) {
    case "database_preset":
      return FavoriteKind.DATABASE_PRESET;
    case "workload_preset":
      return FavoriteKind.WORKLOAD_PRESET;
    case "test_preset":
      return FavoriteKind.TEST_PRESET;
    case "test_run":
      return FavoriteKind.TEST_RUN;
    case "suite":
      return FavoriteKind.SUITE;
    case "suite_run":
      return FavoriteKind.SUITE_RUN;
    default:
      return FavoriteKind.UNSPECIFIED;
  }
}

/** One favorite join row, flattened from cloud.v1.models.FavoriteRecord. */
export interface FavoriteVM {
  /** entity.id — the join row's own id. */
  id: string;
  /** favorite kind label ("" when unknown). */
  kind: FavoriteKindLabel | "";
  /** target_id — the favorited row's id within `kind`. */
  targetId: string;
  /** entity.timings.created_at (ISO) — when the favorite was added. */
  createdAt: string;
}

/** A page of favorites + cursor for the next page (empty at the end). */
export interface FavoritesPage {
  favorites: FavoriteVM[];
  nextPageToken: string;
}

/**
 * List the caller's favorites for a tenant (by slug), optionally narrowed to a
 * single kind (omit / "" = all kinds). Pagination via the opaque page token.
 */
export async function listFavorites(
  tenantSlug: string,
  opts: {
    kind?: FavoriteKindLabel | "";
    pageSize?: number;
    pageToken?: string;
  } = {},
): Promise<FavoritesPage> {
  const tenantId = await resolveTenantId(tenantSlug);
  const resp = await favoriteClient.listFavorites({
    tenantId,
    kind: favoriteKindProto(opts.kind ?? ""),
    page: { size: opts.pageSize ?? 0, token: opts.pageToken ?? "" },
  });
  const j = toJson(ListFavoritesResponseSchema, resp) as {
    favorites?: Array<{
      entity?: { id?: string; timings?: { createdAt?: string } };
      kind?: string;
      targetId?: string;
    }>;
    nextPageToken?: string;
  };
  return {
    favorites: (j.favorites ?? []).map((f) => ({
      id: f.entity?.id ?? "",
      kind: favoriteKindLabelFromJson(f.kind),
      targetId: f.targetId ?? "",
      createdAt: f.entity?.timings?.createdAt ?? "",
    })),
    nextPageToken: j.nextPageToken ?? "",
  };
}

/** Human label + in-app target route for a favorite kind. */
export const FAVORITE_KIND_LABEL: Record<FavoriteKindLabel, string> = {
  database_preset: "Database Preset",
  workload_preset: "Workload Preset",
  test_preset: "Test Preset",
  test_run: "Test Run",
  suite: "Suite",
  suite_run: "Suite Run",
};

/**
 * Build the in-app detail route for a favorited target, relative to the tenant
 * scope (the page composes it under /t/:slug). Returns undefined when the kind
 * has no detail route (suite_run has no standalone page yet).
 */
export function favoriteTargetPath(
  kind: FavoriteKindLabel | "",
  targetId: string,
): string | undefined {
  switch (kind) {
    case "database_preset":
      return `/presets/database/${targetId}`;
    case "workload_preset":
      return `/presets/workload/${targetId}`;
    case "test_preset":
      return `/presets/test/${targetId}`;
    case "test_run":
      return `/runs/${targetId}`;
    case "suite":
      return `/suites/${targetId}`;
    default:
      return undefined;
  }
}
