// Token storage for the real auth flow.
//
// access_token is tenant-agnostic and short-lived: kept in memory only (via
// client.ts) so it never lands in storage. refresh_token is long-lived and
// single-use/rotated, so it MUST survive reloads — persisted in localStorage
// and replaced on every Refresh.

import { setAccessToken } from "@/services/client";
import type { TokenPair } from "@/lib/proto/cloud/v1/api/iam_pb";

const REFRESH_KEY = "stroppy.refreshToken";

export function getRefreshToken(): string | null {
  try {
    return localStorage.getItem(REFRESH_KEY);
  } catch {
    return null;
  }
}

export function setRefreshToken(token: string | null): void {
  try {
    if (token) localStorage.setItem(REFRESH_KEY, token);
    else localStorage.removeItem(REFRESH_KEY);
  } catch {
    /* storage unavailable (private mode) — access token still works in-memory */
  }
}

/** Apply a freshly minted TokenPair: access in memory, rotated refresh persisted. */
export function applyTokens(tokens: TokenPair | undefined): void {
  if (!tokens) return;
  if (tokens.accessToken) setAccessToken(tokens.accessToken);
  if (tokens.refreshToken) setRefreshToken(tokens.refreshToken);
}

/** Drop both credentials (logout / unrecoverable refresh failure). */
export function clearTokens(): void {
  setAccessToken(null);
  setRefreshToken(null);
}
