import { Code, ConnectError } from "@connectrpc/connect";

// Single place that turns any thrown value into a user-facing message.
// Connect RPC errors carry a typed Code; map the common ones to friendly text
// and fall back to the server message otherwise. Never show raw stack traces.
export function errorMessage(err: unknown): string {
  if (err instanceof ConnectError) {
    switch (err.code) {
      case Code.Unauthenticated:
        return "Session expired. Please sign in again.";
      case Code.PermissionDenied:
        return "You do not have permission for this action.";
      case Code.NotFound:
        return "Not found.";
      case Code.AlreadyExists:
        return err.rawMessage || "Already exists.";
      case Code.InvalidArgument:
        return err.rawMessage || "Invalid input.";
      case Code.FailedPrecondition:
        return err.rawMessage || "Action not allowed in the current state.";
      case Code.Unavailable:
        return "Server unreachable. Please try again.";
      case Code.DeadlineExceeded:
        return "Request timed out. Please try again.";
      default:
        return err.rawMessage || err.message;
    }
  }
  if (err instanceof Error) return err.message;
  return "Request failed.";
}

// Map InvalidArgument validation violations to form fields.
// The exact detail shape from connect-go validation is confirmed against the
// live backend when the first form is wired (Phase 1); until then this returns
// no field-level errors and callers rely on errorMessage() for the summary.
export function fieldErrors(_err: unknown): Record<string, string> {
  return {};
}
