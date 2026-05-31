package app

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// demoAccountID is the fixed account id every demo request is attributed to.
const demoAccountID = "demo"

// staticAuthn is a fixed Authn for the demo: it always resolves the same admin
// caller, so no real auth interceptor is needed. The tenant comes from each
// request's tenant_id, so the claims only carry the account id + is_admin so any
// authorization gate passes.
type staticAuthn struct{}

var _ utils.Authn = (*staticAuthn)(nil)

// Caller returns the fixed demo claims.
func (staticAuthn) Caller(_ context.Context) (*iam.AccessClaims, error) {
	return &iam.AccessClaims{
		AccountId: demoAccountID,
		IsAdmin:   true,
	}, nil
}
