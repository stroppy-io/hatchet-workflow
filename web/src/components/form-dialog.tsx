import type { ReactNode } from "react";
import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

type FormDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: ReactNode;
  submitLabel?: string;
  cancelLabel?: string;
  onSubmit: () => void;
  loading?: boolean;
  error?: string | null;
  submitDisabled?: boolean;
  danger?: boolean;
  children: ReactNode;
};

// Reusable create/edit modal: header + form body (children = fields) + inline
// error + cancel/submit footer with in-flight spinner. Closing is blocked while
// loading. Every CRUD dialog uses this instead of re-building Dialog plumbing.
export function FormDialog({
  open,
  onOpenChange,
  title,
  description,
  submitLabel = "Save",
  cancelLabel = "Cancel",
  onSubmit,
  loading = false,
  error,
  submitDisabled = false,
  danger = false,
  children,
}: FormDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!loading) onOpenChange(next);
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description ? <DialogDescription>{description}</DialogDescription> : null}
        </DialogHeader>
        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            onSubmit();
          }}
        >
          <div className="space-y-4">{children}</div>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={loading}
            >
              {cancelLabel}
            </Button>
            <Button
              type="submit"
              variant={danger ? "destructive" : "default"}
              disabled={loading || submitDisabled}
            >
              {loading ? <Loader2 className="size-4 animate-spin" /> : null}
              {submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
