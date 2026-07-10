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

// openInIde mints a ticket for targetUrl (an "/ide/<scope>/..." path, as
// built by orgIdeUrl/instanceIdeUrl) and navigates a new browser tab to the
// ticket-bearing URL the server returns. It never opens targetUrl directly
// — doing so would 403 (no Authorization header on a plain navigation),
// which is exactly the bug this handshake exists to fix.
export async function openInIde(targetUrl: string): Promise<void> {
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
  const body = (await res.json()) as { url: string };
  window.open(body.url, "_blank", "noreferrer");
}
