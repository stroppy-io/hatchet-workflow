import type { ReactNode } from "react";
import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";

type StateBlockProps = {
  loading: boolean;
  error?: string | null;
  empty?: boolean;
  emptyMessage?: string;
  onRetry?: () => void;
  children: ReactNode;
};

// Standard read tri-state: loading spinner → error + retry → empty → content.
// Reused wherever a page renders the result of useListQuery so loading/error/
// empty look the same everywhere (EntityDataTable handles its own internally).
export function StateBlock({
  loading,
  error,
  empty,
  emptyMessage = "Nothing here yet.",
  onRetry,
  children,
}: StateBlockProps) {
  if (loading) {
    return (
      <div className="flex items-center justify-center gap-2 p-8 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" />
        Loading…
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center gap-3 p-8 text-center text-sm">
        <p className="text-destructive">{error}</p>
        {onRetry ? (
          <Button variant="outline" size="sm" onClick={onRetry}>
            Retry
          </Button>
        ) : null}
      </div>
    );
  }

  if (empty) {
    return (
      <div className="p-8 text-center text-sm text-muted-foreground">
        {emptyMessage}
      </div>
    );
  }

  return <>{children}</>;
}
