package quotas

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	quotamanager "github.com/yandex-cloud/go-genproto/yandex/cloud/quotamanager/v1"
	ycsdk "github.com/yandex-cloud/go-sdk/v2"
	"github.com/yandex-cloud/go-sdk/v2/credentials"
	"github.com/yandex-cloud/go-sdk/v2/pkg/options"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

const quotaManagerListMethod = protoreflect.FullName("yandex.cloud.quotamanager.v1.QuotaLimitService.List")

type YandexSource struct {
	SnapshotTTL time.Duration
}

func NewYandexSource(snapshotTTL time.Duration) *YandexSource {
	return &YandexSource{SnapshotTTL: snapshotTTL}
}

func (s *YandexSource) ListQuotas(ctx context.Context, req SourceRequest) ([]Snapshot, error) {
	settings := req.Settings.GetYandex()
	if settings.GetToken() == "" {
		return nil, derrors.Invalid("token", "yandex token is required")
	}
	if settings.GetCloudId() == "" {
		return nil, derrors.Invalid("cloud_id", "yandex cloud_id is required")
	}

	sdk, err := ycsdk.Build(ctx,
		options.WithCredentials(credentials.IAMToken(settings.GetToken())),
		options.WithDefaultRetryOptions(),
	)
	if err != nil {
		return nil, fmt.Errorf("build yandex sdk: %w", err)
	}
	defer sdk.Shutdown(ctx)

	conn, err := sdk.GetConnection(ctx, quotaManagerListMethod)
	if err != nil {
		return nil, fmt.Errorf("yandex quota manager connection: %w", err)
	}
	client := quotamanager.NewQuotaLimitServiceClient(conn)

	services := normalizeServices(req.Services)
	if len(services) == 0 {
		services, err = listYandexQuotaServices(ctx, client)
		if err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()
	staleAfter := now.Add(s.SnapshotTTL)
	if s.SnapshotTTL <= 0 {
		staleAfter = now.Add(5 * time.Minute)
	}

	resource := &quotamanager.Resource{
		Id:   settings.GetCloudId(),
		Type: ResourceTypeYandexCloud,
	}
	out := make([]Snapshot, 0)
	for _, service := range services {
		pageToken := ""
		for {
			resp, err := client.List(ctx, &quotamanager.ListQuotaLimitsRequest{
				Resource:  resource,
				Service:   service,
				PageSize:  1000,
				PageToken: pageToken,
			})
			if err != nil {
				return nil, fmt.Errorf("list yandex quotas for service %q: %w", service, err)
			}
			for _, limit := range resp.GetQuotaLimits() {
				if limit.GetQuotaId() == "" {
					continue
				}
				raw, _ := protojson.Marshal(limit)
				used := limit.GetUsage().GetValue()
				quotaLimit := limit.GetLimit().GetValue()
				out = append(out, Snapshot{
					Scope: Scope{
						TenantID:     req.TenantID,
						Provider:     deploymentpb.Provider_PROVIDER_YANDEX,
						ResourceType: ResourceTypeYandexCloud,
						ResourceID:   settings.GetCloudId(),
						Service:      service,
					},
					QuotaName:         limit.GetQuotaId(),
					Units:             unitsOrInfer("", limit.GetQuotaId()),
					ProviderUsed:      used,
					Limit:             quotaLimit,
					ProviderAvailable: math.Max(quotaLimit-used, 0),
					ObservedAt:        now,
					StaleAfter:        staleAfter,
					Raw:               raw,
				})
			}
			pageToken = resp.GetNextPageToken()
			if pageToken == "" {
				break
			}
		}
	}
	return out, nil
}

func listYandexQuotaServices(ctx context.Context, client quotamanager.QuotaLimitServiceClient) ([]string, error) {
	seen := map[string]bool{}
	pageToken := ""
	for {
		resp, err := client.ListServices(ctx, &quotamanager.ListServicesRequest{
			ResourceType: ResourceTypeYandexCloud,
			PageSize:     1000,
			PageToken:    pageToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list yandex quota services: %w", err)
		}
		for _, service := range resp.GetServices() {
			if service.GetId() != "" {
				seen[service.GetId()] = true
			}
		}
		pageToken = resp.GetNextPageToken()
		if pageToken == "" {
			break
		}
	}
	out := make([]string, 0, len(seen))
	for service := range seen {
		out = append(out, service)
	}
	sort.Strings(out)
	return out, nil
}

func normalizeServices(services []string) []string {
	seen := map[string]bool{}
	for _, service := range services {
		if service != "" {
			seen[service] = true
		}
	}
	out := make([]string, 0, len(seen))
	for service := range seen {
		out = append(out, service)
	}
	sort.Strings(out)
	return out
}
