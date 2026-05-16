package ops

import (
	"context"

	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
)

// QuotaService is a stub that returns empty quota lists.
// Real implementation would query counter tables / cloud APIs.
type QuotaService struct{}

// NewQuotaService constructs the stub QuotaService.
func NewQuotaService() *QuotaService { return &QuotaService{} }

// GetQuotas returns an empty QuotaList.
func (s *QuotaService) GetQuotas(_ context.Context, _ *opspb.GetQuotasRequest) (*opspb.QuotaList, error) {
	return &opspb.QuotaList{}, nil
}

// RefreshQuotas returns an empty QuotaList.
func (s *QuotaService) RefreshQuotas(_ context.Context, _ *opspb.RefreshQuotasRequest) (*opspb.QuotaList, error) {
	return &opspb.QuotaList{}, nil
}
