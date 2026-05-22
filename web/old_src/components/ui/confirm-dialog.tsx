import { createContext, useContext, useState, useCallback, useRef } from "react";
import type { ReactNode } from "react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { AlertTriangle } from "lucide-react";

export interface ConfirmOptions {
  title: string;
  description?: ReactNode;
  /** Button label for the confirm action. Default: "Delete" when danger, else "Confirm". */
  confirmLabel?: string;
  /** Button label for the cancel action. Default: "Cancel". */
  cancelLabel?: string;
  /** When true, paints the confirm button in destructive colours and shows a warning icon. */
  danger?: boolean;
}

type Resolver = (ok: boolean) => void;

interface ConfirmContextValue {
  confirm: (opts: ConfirmOptions) => Promise<boolean>;
}

const ConfirmContext = createContext<ConfirmContextValue | null>(null);

// ConfirmProvider mounts a single Dialog at the root and exposes a
// promise-returning `confirm()` via context. Keeps call sites flat:
//   const ok = await confirm({ title: "Delete?", danger: true });
//   if (!ok) return;
//   ...
// Replaces native window.confirm() so dialogs match the app theme.
export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const [opts, setOpts] = useState<ConfirmOptions | null>(null);
  const resolverRef = useRef<Resolver | null>(null);

  const confirm = useCallback((next: ConfirmOptions) => {
    setOpts(next);
    setOpen(true);
    return new Promise<boolean>((resolve) => {
      resolverRef.current = resolve;
    });
  }, []);

  const finish = useCallback((result: boolean) => {
    const r = resolverRef.current;
    resolverRef.current = null;
    setOpen(false);
    // Defer the resolve so the dialog close animation can run before the
    // caller does whatever it does next (often state updates that trigger
    // a re-render, which would otherwise interrupt the unmount).
    queueMicrotask(() => r?.(result));
  }, []);

  const isDanger = !!opts?.danger;
  const confirmLabel = opts?.confirmLabel ?? (isDanger ? "Delete" : "Confirm");
  const cancelLabel = opts?.cancelLabel ?? "Cancel";

  return (
    <ConfirmContext.Provider value={{ confirm }}>
      {children}
      <Dialog
        open={open}
        onOpenChange={(o) => {
          if (!o) finish(false);
        }}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <div className="flex items-start gap-3">
              {isDanger && (
                <div className="mt-0.5 shrink-0 rounded-full bg-destructive/15 p-1.5">
                  <AlertTriangle className="h-4 w-4 text-destructive" />
                </div>
              )}
              <div className="flex-1">
                <DialogTitle>{opts?.title ?? "Confirm"}</DialogTitle>
                {opts?.description && (
                  <DialogDescription className="mt-2 whitespace-pre-line">
                    {opts.description}
                  </DialogDescription>
                )}
              </div>
            </div>
          </DialogHeader>
          <div className="flex justify-end gap-2 mt-2">
            <Button variant="outline" size="sm" onClick={() => finish(false)}>
              {cancelLabel}
            </Button>
            <Button
              variant={isDanger ? "destructive" : "default"}
              size="sm"
              onClick={() => finish(true)}
              autoFocus
            >
              {confirmLabel}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </ConfirmContext.Provider>
  );
}

export function useConfirm(): (opts: ConfirmOptions) => Promise<boolean> {
  const ctx = useContext(ConfirmContext);
  if (!ctx) {
    throw new Error("useConfirm must be used inside <ConfirmProvider>");
  }
  return ctx.confirm;
}
