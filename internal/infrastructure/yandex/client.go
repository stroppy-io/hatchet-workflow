// Package yandex builds Yandex Cloud SDK clients from a tenant's stored OAuth/IAM
// token (settings KEY_YANDEX_CLOUD_TOKEN). Used for live quota + network inventory
// (the cloud is the source of truth, H33/D19/D20).
package yandex

import (
	"context"
	"fmt"

	ycsdk "github.com/yandex-cloud/go-sdk"
)

// Build constructs a Yandex Cloud SDK using an OAuth token.
func Build(ctx context.Context, token string) (*ycsdk.SDK, error) {
	if token == "" {
		return nil, fmt.Errorf("yandex: empty token")
	}
	sdk, err := ycsdk.Build(ctx, ycsdk.Config{Credentials: ycsdk.OAuthToken(token)})
	if err != nil {
		return nil, fmt.Errorf("yandex: build sdk: %w", err)
	}
	return sdk, nil
}
