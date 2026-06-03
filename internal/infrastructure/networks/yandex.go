package networks

import (
	"context"
	"fmt"
	"sort"

	vpc "github.com/yandex-cloud/go-genproto/yandex/cloud/vpc/v1"
	ycsdk "github.com/yandex-cloud/go-sdk/v2"
	"github.com/yandex-cloud/go-sdk/v2/credentials"
	"github.com/yandex-cloud/go-sdk/v2/pkg/options"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
)

const yandexNetworkListSubnetsMethod = protoreflect.FullName("yandex.cloud.vpc.v1.NetworkService.ListSubnets")

type YandexSource struct{}

func NewYandexSource() *YandexSource {
	return &YandexSource{}
}

func (s *YandexSource) ListSubnets(ctx context.Context, req SourceRequest) ([]ProviderSubnet, error) {
	settings := req.Settings.GetYandex()
	if settings.GetToken() == "" {
		return nil, derrors.Invalid("token", "yandex token is required")
	}
	if settings.GetNetworkId() == "" {
		return nil, derrors.Invalid("network_id", "yandex network_id is required")
	}

	sdk, err := ycsdk.Build(ctx,
		options.WithCredentials(credentials.IAMToken(settings.GetToken())),
		options.WithDefaultRetryOptions(),
	)
	if err != nil {
		return nil, fmt.Errorf("build yandex sdk: %w", err)
	}
	defer sdk.Shutdown(ctx)

	conn, err := sdk.GetConnection(ctx, yandexNetworkListSubnetsMethod)
	if err != nil {
		return nil, fmt.Errorf("yandex vpc connection: %w", err)
	}
	client := vpc.NewNetworkServiceClient(conn)

	out := make([]ProviderSubnet, 0)
	pageToken := ""
	for {
		resp, err := client.ListSubnets(ctx, &vpc.ListNetworkSubnetsRequest{
			NetworkId: settings.GetNetworkId(),
			PageSize:  1000,
			PageToken: pageToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list yandex subnets for network %q: %w", settings.GetNetworkId(), err)
		}
		for _, subnet := range resp.GetSubnets() {
			raw, _ := protojson.Marshal(subnet)
			out = append(out, ProviderSubnet{
				ID:    subnet.GetId(),
				Name:  subnet.GetName(),
				Zone:  subnet.GetZoneId(),
				CIDRs: append([]string{}, subnet.GetV4CidrBlocks()...),
				Raw:   raw,
			})
		}
		pageToken = resp.GetNextPageToken()
		if pageToken == "" {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Zone != out[j].Zone {
			return out[i].Zone < out[j].Zone
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}
