package agent

import (
	"context"

	"go.temporal.io/sdk/activity"
)

// recordHeartbeat is the default heartbeater. It delegates to the Temporal SDK,
// which is a no-op when invoked outside an activity context (e.g. unit tests),
// so it is safe to call with context.Background().
func recordHeartbeat(ctx context.Context, details ...any) {
	activity.RecordHeartbeat(ctx, details...)
}
