package run

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
)

type capturedEvent struct {
	tenantID string
	event    models.Webhook_Event
	id       string
}

type fakeDeliverer struct{ got []capturedEvent }

func (f *fakeDeliverer) DeliverRunEvent(_ context.Context, tenantID string, event models.Webhook_Event, id, _ string) error {
	f.got = append(f.got, capturedEvent{tenantID, event, id})
	return nil
}

func dag(id, tenant string, st primitive.Status) *primitive.Dag {
	d := &primitive.Dag{Id: id, Status: st}
	if tenant != "" {
		d.Metadata = map[string]string{metadataTenantID: tenant}
	}
	return d
}

func TestWebhookTerminalHook(t *testing.T) {
	cases := []struct {
		name string
		dag  *primitive.Dag
		want *capturedEvent
	}{
		{"completed fires RUN_COMPLETED", dag("r1", "t1", primitive.Status_STATUS_COMPLETED),
			&capturedEvent{"t1", models.Webhook_EVENT_RUN_COMPLETED, "r1"}},
		{"failed fires RUN_FAILED", dag("r2", "t1", primitive.Status_STATUS_FAILED),
			&capturedEvent{"t1", models.Webhook_EVENT_RUN_FAILED, "r2"}},
		{"cancelled fires nothing", dag("r3", "t1", primitive.Status_STATUS_CANCELLED), nil},
		{"running fires nothing", dag("r4", "t1", primitive.Status_STATUS_RUNNING), nil},
		{"no tenant fires nothing", dag("r5", "", primitive.Status_STATUS_COMPLETED), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeDeliverer{}
			WebhookTerminalHook(f)(context.Background(), tc.dag)
			if tc.want == nil {
				require.Empty(t, f.got)
				return
			}
			require.Len(t, f.got, 1)
			require.Equal(t, *tc.want, f.got[0])
		})
	}
}
