package managers

import (
	"context"
	"fmt"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"github.com/valkey-io/valkey-go"
)

type QuotasConfig struct {
	AvailableQuotas []*crossplane.Quota `mapstructure:"quotas"`
}

func DefaultQuotasConfig() *QuotasConfig {
	return &QuotasConfig{
		AvailableQuotas: []*crossplane.Quota{
			{
				Cloud:   crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX,
				Kind:    crossplane.Quota_KIND_VM,
				Maximum: 20,
			},
			{
				Cloud:   crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX,
				Kind:    crossplane.Quota_KIND_SUBNET,
				Maximum: 10,
			},
			{
				Cloud:   crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX,
				Kind:    crossplane.Quota_KIND_PUBLIC_IP_ADDRESS,
				Maximum: 1,
			},
		},
	}
}

type QuotaManager struct {
	valkeyClient valkey.Client
	quotasConfig *QuotasConfig
}

func createQuotasIfNotExists(valkeyClient valkey.Client, quotas []*crossplane.Quota) error {
	ctx := context.Background()
	for _, quota := range quotas {
		key := quotaKey(quota)
		cmd := valkeyClient.B().Set().Key(key).Value(fmt.Sprintf("%d", quota.GetMaximum())).Nx().Build()
		if err := valkeyClient.Do(ctx, cmd).Error(); err != nil {
			return fmt.Errorf("failed to create quota %s: %w", key, err)
		}
	}
	return nil
}

func NewQuotaManager(valkeyClient valkey.Client, quotasConfig *QuotasConfig) (*QuotaManager, error) {
	if quotasConfig == nil {
		return nil, fmt.Errorf("quotas config is nil")
	}
	if err := createQuotasIfNotExists(valkeyClient, quotasConfig.AvailableQuotas); err != nil {
		return nil, err
	}
	return &QuotaManager{
		valkeyClient: valkeyClient,
		quotasConfig: quotasConfig,
	}, nil
}

type errQuotaNotAvailable struct {
	Quota *crossplane.Quota
}

func (e *errQuotaNotAvailable) Error() string {
	return fmt.Sprintf("quota %s:%s is not available", e.Quota.GetCloud().String(), e.Quota.GetKind())
}

func quotaKey(quota *crossplane.Quota) string {
	return fmt.Sprintf("%s:%s", quota.GetCloud().String(), quota.GetKind())
}

func (q QuotaManager) ReserveQuotas(ctx context.Context, quotas []*crossplane.Quota) error {
	if len(quotas) == 0 {
		return nil
	}

	keys := make([]string, len(quotas))
	args := make([]string, len(quotas))
	for i, quota := range quotas {
		keys[i] = quotaKey(quota)
		args[i] = fmt.Sprintf("%d", quota.GetMaximum())
	}

	script := `
	for i, key in ipairs(KEYS) do
		local val = redis.call('GET', key)
		local available = tonumber(val) or 0
		local requested = tonumber(ARGV[i])
		if available < requested then
			return i
		end
	end
	for i, key in ipairs(KEYS) do
		redis.call('DECRBY', key, ARGV[i])
	end
	return 0
	`

	cmd := q.valkeyClient.B().Eval().Script(script).Numkeys(int64(len(keys))).Key(keys...).Arg(args...).Build()
	res, err := q.valkeyClient.Do(ctx, cmd).ToInt64()
	if err != nil {
		return fmt.Errorf("failed to reserve quotas: %w", err)
	}

	if res > 0 {
		return &errQuotaNotAvailable{Quota: quotas[res-1]}
	}

	return nil
}

func (q QuotaManager) FreeQuotas(ctx context.Context, quotas []*crossplane.Quota) error {
	if len(quotas) == 0 {
		return nil
	}

	keys := make([]string, len(quotas))
	args := make([]string, len(quotas))
	for i, quota := range quotas {
		keys[i] = quotaKey(quota)
		args[i] = fmt.Sprintf("%d", quota.GetMaximum())
	}

	script := `
	for i, key in ipairs(KEYS) do
		redis.call('INCRBY', key, ARGV[i])
	end
	return 0
	`

	cmd := q.valkeyClient.B().Eval().Script(script).Numkeys(int64(len(keys))).Key(keys...).Arg(args...).Build()
	if err := q.valkeyClient.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("failed to free quotas: %w", err)
	}

	return nil
}
