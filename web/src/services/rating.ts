// Leaderboard data surface — wraps cloud.v1.api.RatingService (system + tenant
// boards) and the public cloud.v1.api.PublicRatingService. Proto -> flat VM.

import { toJson } from "@bufbuild/protobuf";
import {
  GetSystemRatingResponseSchema,
  GetTenantRatingResponseSchema,
} from "@/lib/proto/cloud/v1/api/rating_pb";
import { GetPublicRatingResponseSchema } from "@/lib/proto/cloud/v1/api/public_rating_pb";
import { ratingClient, publicRatingClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";
import { dbKindLabelFromJson, providerLabelFromJson } from "@/services/enums";

/** One ranked benchmark, flattened from api.RatingEntry / PublicRatingEntry. */
export interface RatingEntryVM {
  rank: number;
  metricValue: number;
  metricUnit: string;
  dbKind: string;
  workloadName: string;
  stroppyVersion: string;
  provider: string;
  topologyLabel: string;
  nodeCount: number;
  runAt?: string;
  /** Present only on authenticated boards. */
  runId: string;
  authorName: string;
  tenantName: string;
}

export interface RatingPageVM {
  entries: RatingEntryVM[];
  nextPageToken: string;
}

const num = (v: number | "NaN" | "Infinity" | "-Infinity" | undefined): number =>
  typeof v === "number" ? v : 0;

type RawEntry = {
  rank?: number;
  metricValue?: number | "NaN" | "Infinity" | "-Infinity";
  metricUnit?: string;
  dbKind?: string;
  workloadName?: string;
  stroppyVersion?: string;
  provider?: string;
  topologyLabel?: string;
  nodeCount?: number;
  runAt?: string;
  runId?: string;
  authorName?: string;
  tenantName?: string;
};

function entryToVM(e: RawEntry): RatingEntryVM {
  return {
    rank: e.rank ?? 0,
    metricValue: num(e.metricValue),
    metricUnit: e.metricUnit ?? "",
    dbKind: dbKindLabelFromJson(e.dbKind),
    workloadName: e.workloadName ?? "",
    stroppyVersion: e.stroppyVersion ?? "",
    provider: providerLabelFromJson(e.provider),
    topologyLabel: e.topologyLabel ?? "",
    nodeCount: e.nodeCount ?? 0,
    runAt: e.runAt,
    runId: e.runId ?? "",
    authorName: e.authorName ?? "",
    tenantName: e.tenantName ?? "",
  };
}

export interface RatingQuery {
  metricKey: string;
  dbKinds?: never[]; // facets not surfaced yet; metricKey drives the board
  limit?: number;
  pageToken?: string;
}

export async function getSystemRating(query: RatingQuery): Promise<RatingPageVM> {
  const resp = await ratingClient.getSystemRating({
    filter: { metricKey: query.metricKey },
    limit: query.limit ?? 0,
    pageToken: query.pageToken ?? "",
  });
  const j = toJson(GetSystemRatingResponseSchema, resp) as {
    entries?: RawEntry[];
    nextPageToken?: string;
  };
  return {
    entries: (j.entries ?? []).map(entryToVM),
    nextPageToken: j.nextPageToken ?? "",
  };
}

export async function getTenantRating(
  tenantSlug: string,
  query: RatingQuery,
): Promise<RatingPageVM> {
  const tenantId = await resolveTenantId(tenantSlug);
  const resp = await ratingClient.getTenantRating({
    tenantId,
    filter: { metricKey: query.metricKey },
    limit: query.limit ?? 0,
    pageToken: query.pageToken ?? "",
  });
  const j = toJson(GetTenantRatingResponseSchema, resp) as {
    entries?: RawEntry[];
    nextPageToken?: string;
  };
  return {
    entries: (j.entries ?? []).map(entryToVM),
    nextPageToken: j.nextPageToken ?? "",
  };
}

export async function getPublicRating(query: RatingQuery): Promise<RatingPageVM> {
  const resp = await publicRatingClient.getPublicRating({
    filter: { metricKey: query.metricKey },
    limit: query.limit ?? 0,
    pageToken: query.pageToken ?? "",
  });
  const j = toJson(GetPublicRatingResponseSchema, resp) as {
    entries?: RawEntry[];
    nextPageToken?: string;
  };
  return {
    entries: (j.entries ?? []).map(entryToVM),
    nextPageToken: j.nextPageToken ?? "",
  };
}
