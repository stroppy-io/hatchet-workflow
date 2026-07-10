// EmbeddedIde — the ticket -> iframe handshake shared by every "open in the
// embedded IDE" surface (CatalogEntryEditor.tsx's provider/workflow EDIT
// branch, RecipeEditor.tsx's recipe EDIT branch). Extracted out of
// CatalogEntryEditor.tsx (which originated it) so a second kind (recipes)
// reuses the SAME component instead of a second copy — the product
// decision this task implements is "все редакторы это IDE", one mechanism,
// not a parallel one per entity kind.
//
// Sequence: mintIdeTicket(targetUrl) authenticates with the SPA's Bearer
// token (a fetch, which CAN carry it) and returns a single-use ticket-
// bearing URL; that URL is set as the iframe's src. The iframe's navigation
// carries no Authorization header (browser navigations never do — that is
// exactly why the ticket exists), but it IS same-origin with the SPA (the
// url is always a relative "/ide/..." path — see ide.ts's IdeTicket doc),
// so it is not a cross-site request. The gateway exchanges the ticket for
// an httpOnly cookie scoped to Path=/ide/<scope> with SameSite=Lax (see
// internal/ide/ticket.go's TicketExchanger) and 302s the iframe to the
// clean URL; the browser then attaches that cookie to every subsequent
// request the iframe makes into /ide/<scope>/... (assets, the code-server
// websocket). SameSite=Lax only restricts CROSS-SITE requests — a
// same-origin iframe subresource/navigation is not cross-site, so Lax does
// not block it here (Lax would only matter if the IDE were embedded from a
// different origin, which this never is).
//
// The FIRST open of a scope can take ~15s (EnsureRunning blocks the
// gateway's HTTP response until the code-server container is up — see
// internal/ide/backend.go), so the response to the iframe's navigation
// itself is slow, not just slow-to-render: the iframe's `load` event simply
// does not fire until the container is ready. The overlay below stays up
// until `load` fires and swaps its message after a few seconds so a slow
// first start reads as "working", not "hung".

import { useEffect, useState } from "react";
import { AlertCircle, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { mintIdeTicket } from "@/services/ide";

function classifyIdeError(message: string): "disabled" | "forbidden" | "other" {
  if (message.includes("404")) return "disabled";
  if (message.includes("403")) return "forbidden";
  return "other";
}

export interface EmbeddedIdeProps {
  targetUrl: string;
  entryLabel: string;
}

export function EmbeddedIde({ targetUrl, entryLabel }: EmbeddedIdeProps) {
  const [ticketUrl, setTicketUrl] = useState<string | null>(null);
  const [minting, setMinting] = useState(true);
  const [mintError, setMintError] = useState<{ kind: "disabled" | "forbidden" | "other"; message: string } | null>(
    null,
  );
  const [iframeLoaded, setIframeLoaded] = useState(false);
  const [slowStart, setSlowStart] = useState(false);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setMinting(true);
    setMintError(null);
    setTicketUrl(null);
    setIframeLoaded(false);
    setSlowStart(false);
    mintIdeTicket(targetUrl)
      .then((res) => {
        if (!cancelled) setTicketUrl(res.url);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        const message = e instanceof Error ? e.message : String(e);
        setMintError({ kind: classifyIdeError(message), message });
      })
      .finally(() => !cancelled && setMinting(false));
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [targetUrl, attempt]);

  useEffect(() => {
    if (!ticketUrl || iframeLoaded) return undefined;
    const t = setTimeout(() => setSlowStart(true), 4000);
    return () => clearTimeout(t);
  }, [ticketUrl, iframeLoaded]);

  if (minting) {
    return (
      <div className="flex h-full items-center justify-center gap-2 text-sm text-zinc-500">
        <Loader2 className="h-4 w-4 animate-spin" /> Preparing your IDE session…
      </div>
    );
  }

  if (mintError) {
    const heading =
      mintError.kind === "disabled"
        ? "The embedded IDE is not enabled on this server."
        : mintError.kind === "forbidden"
          ? "You are not authorized to open this entry in the IDE."
          : "Could not open the IDE.";
    const hint =
      mintError.kind === "disabled"
        ? "Ask an administrator to enable IDE_MANAGER_ENABLED and configure the Gitea backend."
        : mintError.kind === "forbidden"
          ? "This requires an update grant on this resource."
          : mintError.message;
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 p-8 text-center">
        <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-4 w-4 shrink-0" /> {heading}
        </div>
        <p className="max-w-md text-xs text-zinc-500">{hint}</p>
        <Button size="sm" variant="outline" onClick={() => setAttempt((n) => n + 1)}>
          Retry
        </Button>
      </div>
    );
  }

  return (
    <div className="relative h-full w-full">
      <iframe
        key={ticketUrl}
        src={ticketUrl ?? undefined}
        title={`${entryLabel} — IDE`}
        onLoad={() => setIframeLoaded(true)}
        className="h-full w-full border-0"
        sandbox="allow-scripts allow-same-origin allow-popups allow-forms allow-downloads allow-modals"
        data-testid="ide-iframe"
      />
      {!iframeLoaded && (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-background/90">
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          <span className="font-mono text-xs text-muted-foreground">
            {slowStart ? "Starting your IDE session — first open can take up to ~15s…" : "Opening IDE…"}
          </span>
        </div>
      )}
    </div>
  );
}
