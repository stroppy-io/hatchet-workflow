//go:build integration

package integration

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gopherex/pgtx"
	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/stroppy-io/stroppy-cloud/internal/app"
	domainauth "github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/dagstore"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/services/tenancy"
)

// TestServerE2ESubmitRunIsProcessed boots the REAL assembled control plane
// (BuildServer over real postgres+valkey), submits a test run through the grpc
// API authenticated with a minted JWT, and asserts the real DagProcessor picks up
// the compiled dag and drives it off PENDING — i.e. the whole wiring works:
// grpc + auth + run service + planner + dagstore + the orchestration loop.
func TestServerE2ESubmitRunIsProcessed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const secret = "e2e-test-secret"

	pgDSN := startRawPostgres(t, ctx)
	vkAddr := startRawValkey(t, ctx)

	addr := freeAddr(t)
	cfg := &app.Config{
		GRPCAddr:           addr,
		JWTSecretRaw:       secret,
		ShareBaseURL_:      "http://localhost/share/",
		Postgres:           parsePGConfig(t, pgDSN),
		VictoriaLogsURL:    "http://localhost:9428",
		VictoriaMetricsURL: "http://localhost:8428",
		ServerAddr_:        "http://" + addr,
	}
	cfg.Valkey.Addresses = []string{vkAddr}

	logger := xlog.Default()
	srv, err := app.BuildServer(ctx, cfg, logger)
	require.NoError(t, err, "BuildServer must assemble against real infra")
	t.Cleanup(srv.Close)

	srvCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	go func() { _ = srv.Run(srvCtx) }()
	waitDial(t, addr)

	// Seed account -> tenant -> member (OWNER) via a second pool to the same DB.
	pool, err := pgxpool.New(ctx, pgDSN)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	executor := pgtx.NewTxDB(pool)
	trm, err := pgtx.NewTxManager(pool, tx.ReadCommitted())
	require.NoError(t, err)

	acctSvc := tenancy.NewAccountAdminService(logger, executor, trm)
	tenantSvc := tenancy.NewTenantAdminService(logger, executor, trm)
	acc, err := acctSvc.CreateAccount(ctx, &adminpb.CreateAccountRequest{
		Account: &models.Account{Email: "e2e@example.com", Nickname: "e2e"}, Password: "pw-12345",
	})
	require.NoError(t, err)
	accountID := &models.AccountId{Value: acc.GetEntity().GetId().GetValue()}
	tenant, err := tenantSvc.CreateTenant(ctx, &adminpb.CreateTenantRequest{
		Tenant: &models.Tenant{OwnerAccountId: accountID},
	})
	require.NoError(t, err)
	tenantID := &models.TenantId{Value: tenant.GetEntity().GetId().GetValue()}
	memberRepo := repository.NewProtoRepository(
		repository.NewScannerRepository(models.TenantMembers.Table, executor), models.TenantMemberConverter)
	_, err = memberRepo.Execute(ctx, models.TenantMembers.Insert().From(
		(&models.TenantMember{Entity: ids.NewEntity(), TenantId: tenantID, AccountId: accountID, Role: models.TenantMember_ROLE_OWNER}).IntoPlain().AllSetters()...))
	require.NoError(t, err)

	// Mint an account JWT + dial the grpc API with it.
	token, err := domainauth.NewSigner([]byte(secret)).SignAccount(accountID.GetValue(), false, time.Hour)
	require.NoError(t, err)
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
			return invoker(ctx, method, req, reply, cc, opts...)
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// Submit a real run through the grpc API.
	runClient := uipb.NewRunServiceClient(conn)
	run, err := runClient.SubmitTestRun(ctx, &uipb.SubmitTestRunRequest{
		TenantId:   tenantID,
		TestPreset: singlePostgresPreset(),
	})
	require.NoError(t, err, "SubmitTestRun over the real grpc API")
	dagID := run.GetDag().GetValue()
	require.NotEmpty(t, dagID, "run must be linked to a dag")

	// The real DagProcessor must pick the dag up and advance it off PENDING.
	store := dagstore.New(logger, executor)
	advanced := false
	for range 30 {
		dag, gerr := store.GetDag(ctx, dagID)
		if gerr == nil && dag != nil && dag.GetStatus() != primitive.Status_STATUS_PENDING &&
			dag.GetStatus() != primitive.Status_STATUS_UNSPECIFIED {
			t.Logf("dag advanced to %s", dag.GetStatus())
			advanced = true
			break
		}
		time.Sleep(time.Second)
	}
	require.True(t, advanced, "DagProcessor never advanced the submitted run's dag off PENDING")
}

func singlePostgresPreset() *domain.TestPreset {
	return &domain.TestPreset{
		Database: &domain.Database{Kind: domain.Database_KIND_POSTGRES, Version: "16"},
		Topology: &domain.Topology{Machines: []*domain.Topology_Machine{{
			Id: "m1", Cores: 2, MemoryGb: 4,
			Components: []*domain.Topology_Component{{Id: "pg", Kind: domain.Topology_Component_KIND_DATABASE}},
		}}},
	}
}

func startRawPostgres(t *testing.T, ctx context.Context) string {
	t.Helper()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			Env:          map[string]string{"POSTGRES_USER": "stroppy", "POSTGRES_PASSWORD": "stroppy", "POSTGRES_DB": "stroppy"},
			ExposedPorts: []string{"5432/tcp"},
			WaitingFor:   wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "5432/tcp")
	// give postgres a moment to accept connections after the port opens
	dsn := fmt.Sprintf("postgres://stroppy:stroppy@%s:%s/stroppy?sslmode=disable", host, port.Port())
	waitPG(t, ctx, dsn)
	return dsn
}

func startRawValkey(t *testing.T, ctx context.Context) string {
	t.Helper()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "valkey/valkey:8-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "6379/tcp")
	return net.JoinHostPort(host, port.Port())
}

func waitPG(t *testing.T, ctx context.Context, dsn string) {
	t.Helper()
	for range 30 {
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				pool.Close()
				return
			}
			pool.Close()
		}
		time.Sleep(time.Second)
	}
	t.Fatal("postgres never became ready")
}

func parsePGConfig(t *testing.T, dsn string) postgres.Config {
	t.Helper()
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	pw, _ := u.User.Password()
	port, _ := strconv.Atoi(u.Port())
	return postgres.Config{
		Host:     u.Hostname(),
		Port:     port,
		Username: u.User.Username(),
		Password: pw,
		Database: strings.TrimPrefix(u.Path, "/"),
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func waitDial(t *testing.T, addr string) {
	t.Helper()
	for range 30 {
		c, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("server never listened on %s", addr)
}
