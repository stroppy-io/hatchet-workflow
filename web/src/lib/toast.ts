import { toast } from "sonner";

import { errorMessage } from "@/lib/errors";

// Thin wrappers over sonner so call sites stay consistent and error toasts
// always run through errorMessage() (no raw error objects on screen).
export function notifySuccess(message: string, description?: string) {
  toast.success(message, { description });
}

export function notifyError(err: unknown, message = "Something went wrong") {
  toast.error(message, { description: errorMessage(err) });
}

export function notifyInfo(message: string, description?: string) {
  toast(message, { description });
}
