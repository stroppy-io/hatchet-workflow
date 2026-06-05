import { useEffect, useMemo, useState } from "react";

import {
  fallbackAuthorDisplay,
  resolveAuthorDisplay,
  type AuthorDisplay,
} from "@/lib/author-display";

const cache: Record<string, AuthorDisplay> = {};
const inflight = new Map<string, Promise<AuthorDisplay>>();

export function useAuthorDisplays(
  authorIds: readonly (string | null | undefined)[],
): Record<string, AuthorDisplay> {
  const key = useMemo(() => normalizeIds(authorIds).join("\0"), [authorIds]);
  const [resolved, setResolved] = useState<Record<string, AuthorDisplay>>(() =>
    pickCached(authorIds),
  );

  useEffect(() => {
    const ids = key ? key.split("\0") : [];
    const cached = pickCached(ids);
    if (Object.keys(cached).length > 0) {
      setResolved((current) => ({ ...current, ...cached }));
    }

    const missing = ids.filter((id) => !cache[id]);
    if (missing.length === 0) return;

    let cancelled = false;
    void Promise.all(missing.map(resolveCachedAuthor)).then((entries) => {
      if (cancelled) return;
      const next: Record<string, AuthorDisplay> = {};
      for (const [id, display] of entries) next[id] = display;
      setResolved((current) => ({ ...current, ...next }));
    });

    return () => {
      cancelled = true;
    };
  }, [key]);

  return resolved;
}

export function useAuthorDisplay(authorId?: string | null): AuthorDisplay | null {
  const authorIds = useMemo(() => [authorId], [authorId]);
  const displays = useAuthorDisplays(authorIds);
  if (!authorId) return null;
  return displays[authorId] ?? fallbackAuthorDisplay(authorId);
}

function normalizeIds(authorIds: readonly (string | null | undefined)[]): string[] {
  return Array.from(
    new Set(authorIds.map((id) => id?.trim()).filter(Boolean) as string[]),
  ).sort();
}

function pickCached(
  authorIds: readonly (string | null | undefined)[],
): Record<string, AuthorDisplay> {
  const picked: Record<string, AuthorDisplay> = {};
  for (const id of normalizeIds(authorIds)) {
    if (cache[id]) picked[id] = cache[id];
  }
  return picked;
}

async function resolveCachedAuthor(id: string): Promise<[string, AuthorDisplay]> {
  const cached = cache[id];
  if (cached) return [id, cached];

  let request = inflight.get(id);
  if (!request) {
    request = resolveAuthorDisplay(id).then((display) => {
      cache[id] = display;
      return display;
    });
    inflight.set(id, request);
  }

  try {
    return [id, await request];
  } finally {
    inflight.delete(id);
  }
}
