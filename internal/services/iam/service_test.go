package iam

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// Smoke test proving the gomock-generated mocks wire into IamService correctly.
func TestGetMyAccount(t *testing.T) {
	ctrl := gomock.NewController(t)

	authn := NewMockAuthn(ctrl)
	accounts := NewMockAccountRepo(ctrl)

	authn.EXPECT().Caller(gomock.Any()).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	accounts.EXPECT().Get(gomock.Any(), "a1").Return(&iampb.Account{Id: "a1", Email: "user@example.com"}, nil)

	svc := NewIamService(IamDeps{Authn: authn, Accounts: accounts})

	resp, err := svc.GetMyAccount(context.Background(), &api.GetMyAccountRequest{})
	if err != nil {
		t.Fatalf("GetMyAccount: %v", err)
	}
	if got := resp.GetAccount().GetId(); got != "a1" {
		t.Fatalf("account id = %q, want a1", got)
	}
}
