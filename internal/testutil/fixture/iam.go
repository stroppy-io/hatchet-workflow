package fixture

import (
	"testing"
	"time"

	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/iam"
)

type IAMFixture struct {
	*F
	IAM *iam.Service
}

func NewIAM(t *testing.T) *IAMFixture {
	t.Helper()
	base := New(t)
	executor := base.Executor.(*sqlexec.TxExecutor)
	svc := iam.New(
		executor,
		base.TxMgr,
		base.Events,
		configurator.AuthConfig{
			AccessTTL:  15 * time.Minute,
			RefreshTTL: 720 * time.Hour,
		},
		[]byte("test-jwt-secret-32-bytes-long!!"),
	)
	return &IAMFixture{F: base, IAM: svc}
}
