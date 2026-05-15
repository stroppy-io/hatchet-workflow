# Plan 01a — IAM ratel API patch

> Supersedes Tasks 13–17 of `2026-05-15-01-foundation-iam.md`. The original plan was written against a fictional ratel API (`repo.Insert/SelectOne/Update/Delete` + `set.Eq/And/In/IsNull/Update`). Real `github.com/yaroher/ratel v0.4.25` uses generated table accessors + column-typed builders. This file replaces the broken code blocks. Task numbers and acceptance criteria from the original plan still apply; only the code listings change.

## Real ratel API cheat-sheet (komeet-derived)

**Read one row → proto:**
```go
acc, err := s.accountRepo.QueryRow(ctx,
    account.Accounts.SelectAll().Where(
        account.Accounts.Email.Eq(email),
        account.Accounts.DeletedAt.IsNull(), // soft-delete guard
    ),
)
if errors.Is(err, pgx.ErrNoRows) { /* not found */ }
```

**Read many → []proto:**
```go
rows, err := s.sessionRepo.Query(ctx, account.Sessions.SelectAll().Where(...))
```

**Insert:**
```go
scanner := user.IntoPlain()           // *UserScanner — plain go struct
scanner.PasswordHash = hash           // populate virtual / extra fields
scanner.CreatedAt = time.Now()        // explicit; the column has no DB default for inserts in this codebase pattern
scanner.UpdatedAt = time.Now()
_, err := s.userRepo.Execute(ctx,
    iampb.Users.Insert().From(scanner.AllSetters()...),
)
```

**Update:**
```go
_, err := s.refreshRepo.Execute(ctx,
    iampb.RefreshTokens.Update().
        Set(iampb.RefreshTokens.RevokedAt.Set(time.Now())).
        Where(iampb.RefreshTokens.Id.Eq(id)),
)
```

**Delete:**
```go
_, err := s.memberRepo.Execute(ctx,
    iampb.TenantMembers.Delete().Where(iampb.TenantMembers.Id.Eq(id)),
)
```

**Multiple Where conditions:** Variadic `Where(a, b, c)` ANDs them.

**Unique-violation detection:** Use the generated helpers per table — `iampb.IsUserEmailUniqueIdxError(err)`, `iampb.IsUserNicknameUniqueIdxError(err)`, etc. Constraint names in `UserConstraintEmailIdx`, `UserConstraintNicknameIdx`.

**Not-found sentinel:** `pgx.ErrNoRows` from `github.com/jackc/pgx/v5`.

**Column types — generated table struct fields:**
- `iampb.Users.Id schema.TextColumnI[UserColumnAlias]` → `.Eq(v)`, `.IsNull()`, `.Set(v)`
- `iampb.Users.DeletedAt schema.NullTimestamptzColumnI[...]` → `.IsNull()`
- Multi-value: column may expose `.In(values...)`. If absent, build OR chain via the actual generated method (check the field type — likely `clause.In` builder available; if not, query with `IN` literal or fall back to N round-trips).

**Generated scanner schema** (real field names — `IntoPlain()` returns `*XxxScanner`):

| Table | Columns (Scanner fields) |
|---|---|
| `iampb.Users` | `Id, CreatedAt, UpdatedAt, DeletedAt, Email, Nickname, PasswordHash` |
| `iampb.Tenants` | `Id, CreatedAt, UpdatedAt, DeletedAt, Name, Description, Label []string` |
| `iampb.TenantMembers` | `Id, TenantId, UserId, CreatedAt, UpdatedAt, DeletedAt, Role` (Role is text-encoded enum name) |
| `iampb.RefreshTokens` | `Id, UserId, CreatedAt, UpdatedAt, DeletedAt, TokenHash, ExpiresAt, FamilyId, ReplacedBy *string, Jti, RevokedAt *time.Time, RevocationReason, UserAgent` |
| `iampb.ApiTokens` | `Id, TenantId, CreatedBy *string, CreatedAt, UpdatedAt, DeletedAt, Name, Description *string, Scopes []string, TokenHash, ExpiresAt *time.Time, LastUsedAt *time.Time` |

⚠️ Plan-original called the secret column `Token` / `Hash`. Real schema = `TokenHash` on both refresh and api tokens. **Always hash before insert; never store cleartext.**

---

## Task 13 (rewritten) — IAM service skeleton + repos

**Files:**
- Create: `internal/domain/services/iam/service.go`, `passwords.go`, `jwt.go`

### Step 1 — `service.go`

```go
package iam

import (
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// Service owns IAM tables and exposes business methods consumed by Connect handlers.
type Service struct {
	*tracing.Entity

	userRepo    *repository.ProtoRepository[iampb.UserAlias, iampb.UserColumnAlias, *iampb.UserScanner, *iampb.User]
	tenantRepo  *repository.ProtoRepository[iampb.TenantAlias, iampb.TenantColumnAlias, *iampb.TenantScanner, *iampb.Tenant]
	memberRepo  *repository.ProtoRepository[iampb.TenantMemberAlias, iampb.TenantMemberColumnAlias, *iampb.TenantMemberScanner, *iampb.TenantMember]
	refreshRepo *repository.ProtoRepository[iampb.RefreshTokenAlias, iampb.RefreshTokenColumnAlias, *iampb.RefreshTokenScanner, *iampb.RefreshToken]
	tokenRepo   *repository.ProtoRepository[iampb.ApiTokenAlias, iampb.ApiTokenColumnAlias, *iampb.ApiTokenScanner, *iampb.ApiToken]

	txManager pgtx.TxManager
	events    eventing.Bus
	cfg       configurator.AuthConfig
	jwtSecret []byte
}

func New(executor exec.DB, txManager pgtx.TxManager, events eventing.Bus, cfg configurator.AuthConfig, jwtSecret []byte) *Service {
	return &Service{
		Entity: tracing.NewEntity("iam.Service"),
		userRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.Users.Table, executor),
			iampb.UserConverter,
		),
		tenantRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.Tenants.Table, executor),
			iampb.TenantConverter,
		),
		memberRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.TenantMembers.Table, executor),
			iampb.TenantMemberConverter,
		),
		refreshRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.RefreshTokens.Table, executor),
			iampb.RefreshTokenConverter,
		),
		tokenRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.ApiTokens.Table, executor),
			iampb.ApiTokenConverter,
		),
		txManager: txManager,
		events:    events,
		cfg:       cfg,
		jwtSecret: jwtSecret,
	}
}
```

### Step 2 — `passwords.go`

```go
package iam

import "golang.org/x/crypto/bcrypt"

func hashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func verifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// sha256Hex is used to hash bearer tokens (refresh + api) before storing.
// Bearer tokens are random 26-char ULIDs (or longer) — bcrypt is overkill;
// sha256 is sufficient against rainbow tables, lookup is O(1) via unique index.
import "crypto/sha256"
import "encoding/hex"
```

NOTE: the `crypto/sha256` + `encoding/hex` imports cannot live below the function block — collect all imports into a single `import ( … )` block at the top:

```go
package iam

import (
	"crypto/sha256"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

func hashPassword(plain string) (string, error) { ... }
func verifyPassword(hash, plain string) bool    { ... }

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
```

### Step 3 — `jwt.go`

```go
package iam

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type accessClaims struct {
	UserID string `json:"sub"`
	JTI    string `json:"jti"`
	jwt.RegisteredClaims
}

func (s *Service) signAccessToken(userID, jti string) (string, error) {
	now := time.Now()
	claims := accessClaims{
		UserID: userID,
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTTL)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.jwtSecret)
}

func (s *Service) VerifyAccessToken(token string) (string, string, error) {
	parsed, err := jwt.ParseWithClaims(token, &accessClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected sign method: %v", t.Method)
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return "", "", err
	}
	claims, ok := parsed.Claims.(*accessClaims)
	if !ok || !parsed.Valid {
		return "", "", fmt.Errorf("invalid token")
	}
	return claims.UserID, claims.JTI, nil
}
```

### Step 4 — Build + Commit

```
go build ./internal/domain/services/iam/...
git add internal/domain/services/iam/
git commit -m "feat(iam): service skeleton with repos + password/JWT helpers"
```

---

## Task 14 (rewritten) — IAM users CRUD

**Files:**
- Create: `internal/domain/services/iam/users.go`, `internal/domain/services/iam/integration_test.go`
- Update: `internal/testutil/fixture/iam.go` (already stub)

### Step 1 — Failing test `internal/domain/services/iam/integration_test.go`

```go
package iam_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestCreateUser(t *testing.T) {
	f := fixture.NewIAM(t)

	user := &iampb.User{
		Email:    "alice@example.com",
		Nickname: "alice",
	}
	created, err := f.IAM.CreateUser(context.Background(), user, "P@ssw0rd!")
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())

	got, err := f.IAM.GetUserByID(context.Background(), created.GetId())
	require.NoError(t, err)
	require.Equal(t, "alice@example.com", got.GetEmail())
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	f := fixture.NewIAM(t)
	_, err := f.IAM.CreateUser(context.Background(), &iampb.User{Email: "dup@e.com", Nickname: "dup1"}, "P@ssw0rd!")
	require.NoError(t, err)
	_, err = f.IAM.CreateUser(context.Background(), &iampb.User{Email: "dup@e.com", Nickname: "dup2"}, "P@ssw0rd!")
	require.Error(t, err)
}
```

### Step 2 — `users.go`

```go
package iam

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateUser persists a new user with a hashed password. Returns the user
// proto with id + timestamps populated.
func (s *Service) CreateUser(ctx context.Context, user *iampb.User, password string) (*iampb.User, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateUser",
		func(ctx context.Context, _ trace.Span) (*iampb.User, error) {
			return pgtx.WithSerializableRet(ctx, s.txManager,
				func(ctx context.Context) (*iampb.User, error) {
					hash, err := hashPassword(password)
					if err != nil {
						return nil, err
					}
					now := time.Now()
					user.Id = &iampb.UserId{Value: ids.New()}
					user.Timestamps = &commonpb.Timestamps{
						CreatedAt: timestamppb.New(now),
						UpdatedAt: timestamppb.New(now),
					}
					scanner := user.IntoPlain()
					scanner.PasswordHash = hash

					if _, err := s.userRepo.Execute(ctx,
						iampb.Users.Insert().From(scanner.AllSetters()...),
					); err != nil {
						if iampb.IsUserEmailUniqueIdxError(err) {
							return nil, domainerr.AlreadyExists(domainerr.ResourceInfo("user", user.GetEmail()))
						}
						if iampb.IsUserNicknameUniqueIdxError(err) {
							return nil, domainerr.AlreadyExists(domainerr.ResourceInfo("user", user.GetNickname()))
						}
						return nil, err
					}
					s.events.Publish(ctx, eventing.UserCreated{UserID: user.GetId().GetValue()})
					return user, nil
				})
		})
}

// GetUserByID retrieves a user, returning domainerr.NotFound if absent.
func (s *Service) GetUserByID(ctx context.Context, id *iampb.UserId) (*iampb.User, error) {
	u, err := s.userRepo.QueryRow(ctx,
		iampb.Users.SelectAll().Where(
			iampb.Users.Id.Eq(id.GetValue()),
			iampb.Users.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("user", id.GetValue()))
		}
		return nil, err
	}
	return u, nil
}

// GetUserByEmail looks up by email.
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*iampb.User, error) {
	u, err := s.userRepo.QueryRow(ctx,
		iampb.Users.SelectAll().Where(
			iampb.Users.Email.Eq(email),
			iampb.Users.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("user", email))
		}
		return nil, err
	}
	return u, nil
}

// ListUsers returns all non-deleted users (caller is platform admin).
func (s *Service) ListUsers(ctx context.Context) ([]*iampb.User, error) {
	return s.userRepo.Query(ctx,
		iampb.Users.SelectAll().Where(iampb.Users.DeletedAt.IsNull()),
	)
}
```

Add `"go.opentelemetry.io/otel/trace"` to imports (for `trace.Span` in `WithTraceRet` callback).

### Step 3 — `internal/testutil/fixture/iam.go`

Replace stub with:

```go
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
```

### Step 4 — Run tests (needs Docker)

```
go test ./internal/domain/services/iam/... -v -run TestCreateUser
```
Expected: PASS (uses real Postgres testcontainer).

### Step 5 — Commit

```
git add internal/domain/services/iam/users.go internal/domain/services/iam/integration_test.go internal/testutil/fixture/iam.go
git commit -m "feat(iam): create/get/list users + duplicate email handling"
```

---

## Task 15 (rewritten) — Tenants + Members CRUD

**Files:** create `tenants.go`, `members.go`; append tests to `integration_test.go`.

### Step 1 — Append test

```go
func TestCreateTenantAndMember(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	user, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "owner@e.com", Nickname: "owner"}, "P@ssw0rd!")
	require.NoError(t, err)

	tenant, err := f.IAM.CreateTenant(ctx, &iampb.Tenant{Name: "Acme"}, user.GetId())
	require.NoError(t, err)
	require.NotEmpty(t, tenant.GetId().GetValue())

	ok, err := f.IAM.HasTenantRole(ctx, user.GetId(), tenant.GetId(), iampb.TenantRole_TENANT_ROLE_OWNER)
	require.NoError(t, err)
	require.True(t, ok)
}
```

### Step 2 — `tenants.go`

```go
package iam

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// CreateTenant creates a tenant and adds the owner as TENANT_ROLE_OWNER in one tx.
func (s *Service) CreateTenant(ctx context.Context, tenant *iampb.Tenant, ownerID *iampb.UserId) (*iampb.Tenant, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateTenant",
		func(ctx context.Context, _ trace.Span) (*iampb.Tenant, error) {
			return pgtx.WithSerializableRet(ctx, s.txManager,
				func(ctx context.Context) (*iampb.Tenant, error) {
					now := time.Now()
					tenant.Id = &iampb.TenantId{Value: ids.New()}
					tenant.Timestamps = &commonpb.Timestamps{
						CreatedAt: timestamppb.New(now),
						UpdatedAt: timestamppb.New(now),
					}
					tScanner := tenant.IntoPlain()
					if _, err := s.tenantRepo.Execute(ctx,
						iampb.Tenants.Insert().From(tScanner.AllSetters()...),
					); err != nil {
						return nil, err
					}

					member := &iampb.TenantMember{
						Id:       &iampb.TenantMemberId{Value: ids.New()},
						TenantId: tenant.GetId(),
						UserId:   ownerID,
						Role:     iampb.TenantRole_TENANT_ROLE_OWNER,
						Timestamps: &commonpb.Timestamps{
							CreatedAt: timestamppb.New(now),
							UpdatedAt: timestamppb.New(now),
						},
					}
					mScanner := member.IntoPlain()
					if _, err := s.memberRepo.Execute(ctx,
						iampb.TenantMembers.Insert().From(mScanner.AllSetters()...),
					); err != nil {
						return nil, err
					}

					s.events.Publish(ctx, eventing.TenantCreated{TenantID: tenant.GetId().GetValue()})
					s.events.Publish(ctx, eventing.MemberAdded{
						UserID:   ownerID.GetValue(),
						TenantID: tenant.GetId().GetValue(),
						Role:     iampb.TenantRole_TENANT_ROLE_OWNER.String(),
					})
					return tenant, nil
				})
		})
}

func (s *Service) GetTenantByID(ctx context.Context, id *iampb.TenantId) (*iampb.Tenant, error) {
	t, err := s.tenantRepo.QueryRow(ctx,
		iampb.Tenants.SelectAll().Where(
			iampb.Tenants.Id.Eq(id.GetValue()),
			iampb.Tenants.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("tenant", id.GetValue()))
		}
		return nil, err
	}
	return t, nil
}

// ListTenantsForUser returns tenants where user is a member.
// Implementation: load member rows for user, then load each tenant via repeated
// QueryRow. Acceptable since memberships per user are bounded (<100 typically).
// If ratel exposes `.In()` on TextColumn the query can be collapsed into one.
func (s *Service) ListTenantsForUser(ctx context.Context, userID *iampb.UserId) ([]*iampb.Tenant, error) {
	members, err := s.memberRepo.Query(ctx,
		iampb.TenantMembers.SelectAll().Where(
			iampb.TenantMembers.UserId.Eq(userID.GetValue()),
			iampb.TenantMembers.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		return nil, err
	}
	out := make([]*iampb.Tenant, 0, len(members))
	for _, m := range members {
		t, err := s.GetTenantByID(ctx, m.GetTenantId())
		if err != nil {
			if errors.Is(err, domainerr.Codeful(domainerr.NotFound().Code())) {
				continue // tenant deleted; skip
			}
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}
```

### Step 3 — `members.go`

```go
package iam

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func (s *Service) AddMember(ctx context.Context, userID *iampb.UserId, tenantID *iampb.TenantId, role iampb.TenantRole) (*iampb.TenantMember, error) {
	now := time.Now()
	m := &iampb.TenantMember{
		Id:       &iampb.TenantMemberId{Value: ids.New()},
		TenantId: tenantID,
		UserId:   userID,
		Role:     role,
		Timestamps: &commonpb.Timestamps{
			CreatedAt: timestamppb.New(now),
			UpdatedAt: timestamppb.New(now),
		},
	}
	scanner := m.IntoPlain()
	if _, err := s.memberRepo.Execute(ctx,
		iampb.TenantMembers.Insert().From(scanner.AllSetters()...),
	); err != nil {
		return nil, err
	}
	s.events.Publish(ctx, eventing.MemberAdded{
		UserID:   userID.GetValue(),
		TenantID: tenantID.GetValue(),
		Role:     role.String(),
	})
	return m, nil
}

// HasTenantRole returns true iff user has a (non-deleted) membership in tenant with role >= min.
// TenantMember.Role is stored as text (proto enum name); compare on the enum value side after lookup.
func (s *Service) HasTenantRole(ctx context.Context, userID *iampb.UserId, tenantID *iampb.TenantId, min iampb.TenantRole) (bool, error) {
	m, err := s.memberRepo.QueryRow(ctx,
		iampb.TenantMembers.SelectAll().Where(
			iampb.TenantMembers.UserId.Eq(userID.GetValue()),
			iampb.TenantMembers.TenantId.Eq(tenantID.GetValue()),
			iampb.TenantMembers.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return m.GetRole() >= min, nil
}

func (s *Service) HasTenantMember(ctx context.Context, userID *iampb.UserId, tenantID *iampb.TenantId) (bool, error) {
	return s.HasTenantRole(ctx, userID, tenantID, iampb.TenantRole_TENANT_ROLE_VIEWER)
}

func (s *Service) ListMembers(ctx context.Context, tenantID *iampb.TenantId) ([]*iampb.TenantMember, error) {
	return s.memberRepo.Query(ctx,
		iampb.TenantMembers.SelectAll().Where(
			iampb.TenantMembers.TenantId.Eq(tenantID.GetValue()),
			iampb.TenantMembers.DeletedAt.IsNull(),
		),
	)
}

func (s *Service) RemoveMember(ctx context.Context, id *iampb.TenantMemberId) error {
	_, err := s.memberRepo.Execute(ctx,
		iampb.TenantMembers.Update().
			Set(iampb.TenantMembers.DeletedAt.Set(time.Now())).
			Where(iampb.TenantMembers.Id.Eq(id.GetValue())),
	)
	return err
}
```

### Step 4 — Run + commit

```
go test ./internal/domain/services/iam/... -v -run TestCreateTenant
git add internal/domain/services/iam/{tenants,members}.go internal/domain/services/iam/integration_test.go
git commit -m "feat(iam): tenant create + member CRUD + HasTenantRole"
```

---

## Task 16 (rewritten) — Login + Logout + Refresh rotation

**Files:** create `auth.go`; append tests.

### Step 1 — Append tests (no change from original)

```go
func TestLoginAndRefreshRotation(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	_, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "user@e.com", Nickname: "user1"}, "P@ssw0rd!")
	require.NoError(t, err)

	pair1, err := f.IAM.Login(ctx, "user@e.com", "P@ssw0rd!")
	require.NoError(t, err)
	require.NotEmpty(t, pair1.GetAccessToken())
	require.NotEmpty(t, pair1.GetRefreshToken())

	pair2, err := f.IAM.RefreshTokens(ctx, pair1.GetRefreshToken())
	require.NoError(t, err)
	require.NotEqual(t, pair1.GetAccessToken(), pair2.GetAccessToken())
	require.NotEqual(t, pair1.GetRefreshToken(), pair2.GetRefreshToken())

	// Reuse old token → triggers family revoke
	_, err = f.IAM.RefreshTokens(ctx, pair1.GetRefreshToken())
	require.Error(t, err)

	// New token also invalid (family revoked)
	_, err = f.IAM.RefreshTokens(ctx, pair2.GetRefreshToken())
	require.Error(t, err)
}

func TestLoginWrongPassword(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	_, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "u@e.com", Nickname: "u"}, "Correct123!")
	require.NoError(t, err)

	_, err = f.IAM.Login(ctx, "u@e.com", "wrong")
	require.Error(t, err)
}
```

### Step 2 — `auth.go`

```go
package iam

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	errorspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// Login validates credentials and returns a fresh TokenPair starting a new
// rotation family.
func (s *Service) Login(ctx context.Context, email, password string) (*iampb.TokenPair, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "Login",
		func(ctx context.Context, _ trace.Span) (*iampb.TokenPair, error) {
			return pgtx.WithSerializableRet(ctx, s.txManager,
				func(ctx context.Context) (*iampb.TokenPair, error) {
					user, err := s.GetUserByEmail(ctx, email)
					if err != nil {
						if errors.Is(err, domainerr.Codeful(errorspb.Code_CODE_NOT_FOUND)) {
							return nil, domainerr.Unauthenticated()
						}
						return nil, err
					}
					if !verifyPassword(user.GetPasswordHash(), password) {
						return nil, domainerr.Unauthenticated()
					}
					return s.issuePair(ctx, user.GetId(), ids.New() /* new family */)
				})
		})
}

// RefreshTokens rotates a refresh token; reuse of a revoked token revokes the whole family.
func (s *Service) RefreshTokens(ctx context.Context, refreshTokenValue string) (*iampb.TokenPair, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "RefreshTokens",
		func(ctx context.Context, _ trace.Span) (*iampb.TokenPair, error) {
			return pgtx.WithSerializableRet(ctx, s.txManager,
				func(ctx context.Context) (*iampb.TokenPair, error) {
					tokHash := sha256Hex(refreshTokenValue)
					tok, err := s.refreshRepo.QueryRow(ctx,
						iampb.RefreshTokens.SelectAll().Where(
							iampb.RefreshTokens.TokenHash.Eq(tokHash),
						),
					)
					if err != nil {
						if errors.Is(err, pgx.ErrNoRows) {
							return nil, domainerr.Unauthenticated()
						}
						return nil, err
					}
					if tok.GetRevokedAt() != nil {
						// Reuse detection: revoke whole family.
						_ = s.revokeFamily(ctx, tok.GetFamilyId())
						return nil, domainerr.Unauthenticated()
					}
					if tok.GetExpiresAt().AsTime().Before(time.Now()) {
						return nil, domainerr.Unauthenticated()
					}

					// Revoke this token.
					if _, err := s.refreshRepo.Execute(ctx,
						iampb.RefreshTokens.Update().
							Set(iampb.RefreshTokens.RevokedAt.Set(time.Now())).
							Where(iampb.RefreshTokens.Id.Eq(tok.GetId().GetValue())),
					); err != nil {
						return nil, err
					}

					return s.issuePair(ctx, tok.GetUserId(), tok.GetFamilyId())
				})
		})
}

// Logout revokes the refresh token.
func (s *Service) Logout(ctx context.Context, refreshTokenValue string) error {
	_, err := s.refreshRepo.Execute(ctx,
		iampb.RefreshTokens.Update().
			Set(iampb.RefreshTokens.RevokedAt.Set(time.Now())).
			Where(iampb.RefreshTokens.TokenHash.Eq(sha256Hex(refreshTokenValue))),
	)
	return err
}

// issuePair issues access + refresh tokens; refresh is stored hashed.
func (s *Service) issuePair(ctx context.Context, userID *iampb.UserId, familyID string) (*iampb.TokenPair, error) {
	now := time.Now()
	jti := ids.New()
	access, err := s.signAccessToken(userID.GetValue(), jti)
	if err != nil {
		return nil, err
	}
	refreshValue := ids.New() + ids.New() // 52 chars opaque
	row := &iampb.RefreshToken{
		Id:        &iampb.RefreshTokenId{Value: ids.New()},
		UserId:    userID,
		FamilyId:  familyID,
		Jti:       jti,
		TokenHash: sha256Hex(refreshValue),
		ExpiresAt: timestamppb.New(now.Add(s.cfg.RefreshTTL)),
		Timestamps: &commonpb.Timestamps{
			CreatedAt: timestamppb.New(now),
			UpdatedAt: timestamppb.New(now),
		},
	}
	scanner := row.IntoPlain()
	if _, err := s.refreshRepo.Execute(ctx,
		iampb.RefreshTokens.Insert().From(scanner.AllSetters()...),
	); err != nil {
		return nil, err
	}
	return &iampb.TokenPair{
		AccessToken:           access,
		RefreshToken:          refreshValue,
		AccessTokenExpiresIn:  durationpb.New(s.cfg.AccessTTL),
		RefreshTokenExpiresIn: durationpb.New(s.cfg.RefreshTTL),
	}, nil
}

func (s *Service) revokeFamily(ctx context.Context, familyID string) error {
	_, err := s.refreshRepo.Execute(ctx,
		iampb.RefreshTokens.Update().
			Set(iampb.RefreshTokens.RevokedAt.Set(time.Now())).
			Where(
				iampb.RefreshTokens.FamilyId.Eq(familyID),
				iampb.RefreshTokens.RevokedAt.IsNull(),
			),
	)
	return err
}
```

⚠️ Compile-time gotcha: the actual `iampb.RefreshToken` proto may NOT have `TokenHash` / `FamilyId` / `Jti` / `RevokedAt` / `ExpiresAt` as top-level fields if the proto uses different field names than the ratel-generated columns. **Before implementing**, run:

```bash
grep "protobuf:" internal/proto/cloud/v1/iam/refresh_token.pb.go | head -15
```

…to verify the proto fields. If they diverge from the ratel table columns, populate the scanner directly instead of the proto:

```go
scanner := &iampb.RefreshTokenScanner{
    Id:        ids.New(),
    UserId:    userID.GetValue(),
    CreatedAt: now,
    UpdatedAt: now,
    TokenHash: sha256Hex(refreshValue),
    ExpiresAt: now.Add(s.cfg.RefreshTTL),
    FamilyId:  familyID,
    Jti:       jti,
}
_, err := s.refreshRepo.Scanner().Execute(ctx, iampb.RefreshTokens.Insert().From(scanner.AllSetters()...))
```

`ProtoRepository.Scanner()` returns the underlying `ScannerRepository` which can take a `*XxxScanner` directly. Use whichever shape compiles. Document the choice in the commit message.

### Step 3 — Run + commit

```
go test ./internal/domain/services/iam/... -v -run "TestLogin|TestRefresh"
git add internal/domain/services/iam/auth.go internal/domain/services/iam/integration_test.go
git commit -m "feat(iam): Login + RefreshTokens rotation + family reuse-detection + Logout"
```

---

## Task 17 (rewritten) — API tokens CRUD

### Step 1 — Append test

```go
func TestApiTokenCreateAndVerify(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	u, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "t@e.com", Nickname: "t"}, "P@ss123!")
	tn, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Name: "T"}, u.GetId())

	token, plain, err := f.IAM.CreateApiToken(ctx, tn.GetId(), u.GetId(), "ci-pipeline")
	require.NoError(t, err)
	require.NotEmpty(t, plain)
	require.NotEqual(t, plain, token.GetTokenHash()) // field is TokenHash on the api_token proto

	resolved, err := f.IAM.VerifyApiToken(ctx, plain)
	require.NoError(t, err)
	require.Equal(t, tn.GetId().GetValue(), resolved.GetTenantId().GetValue())
}
```

### Step 2 — `api_tokens.go`

```go
package iam

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// CreateApiToken creates a token tied to (tenant, user). Returns the proto + cleartext.
func (s *Service) CreateApiToken(ctx context.Context, tenantID *iampb.TenantId, userID *iampb.UserId, name string) (*iampb.ApiToken, string, error) {
	now := time.Now()
	plain := "sct_" + ids.New() + ids.New()
	row := &iampb.ApiToken{
		Id:        &iampb.ApiTokenId{Value: ids.New()},
		TenantId:  tenantID,
		CreatedBy: &iampb.UserId{Value: userID.GetValue()},
		Name:      name,
		TokenHash: sha256Hex(plain),
		Timestamps: &commonpb.Timestamps{
			CreatedAt: timestamppb.New(now),
			UpdatedAt: timestamppb.New(now),
		},
	}
	scanner := row.IntoPlain()
	if _, err := s.tokenRepo.Execute(ctx,
		iampb.ApiTokens.Insert().From(scanner.AllSetters()...),
	); err != nil {
		return nil, "", err
	}
	return row, plain, nil
}

// VerifyApiToken hashes plaintext, looks up by token_hash, rejects revoked / soft-deleted.
func (s *Service) VerifyApiToken(ctx context.Context, plain string) (*iampb.ApiToken, error) {
	row, err := s.tokenRepo.QueryRow(ctx,
		iampb.ApiTokens.SelectAll().Where(
			iampb.ApiTokens.TokenHash.Eq(sha256Hex(plain)),
			iampb.ApiTokens.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.Unauthenticated()
		}
		return nil, err
	}
	return row, nil
}

// RevokeApiToken soft-deletes a token.
func (s *Service) RevokeApiToken(ctx context.Context, id *iampb.ApiTokenId) error {
	_, err := s.tokenRepo.Execute(ctx,
		iampb.ApiTokens.Update().
			Set(iampb.ApiTokens.DeletedAt.Set(time.Now())).
			Where(iampb.ApiTokens.Id.Eq(id.GetValue())),
	)
	return err
}
```

⚠️ Verify ApiToken proto has `TokenHash` field (lowercase `token_hash` in proto). If proto field is named differently, fall back to the Scanner-direct pattern as in Task 16's gotcha. Same applies to `CreatedBy` field shape (could be `*UserId` or `string`).

### Step 3 — Run + commit

```
go test ./internal/domain/services/iam/... -v -run TestApiToken
git add internal/domain/services/iam/api_tokens.go internal/domain/services/iam/integration_test.go
git commit -m "feat(iam): API tokens with sha256 storage + verify/revoke"
```

---

## Cascading fixes (Tasks 18-27)

When those tasks reference iam service methods, the signatures match the patch above. The middleware tests (Task 20, 21) and Connect handlers (Task 22) call:

- `iam.Service.VerifyAccessToken(token) (userID, jti string, error)` — unchanged
- `iam.Service.VerifyApiToken(ctx, plain) (*iampb.ApiToken, error)` — unchanged
- `iam.Service.GetUserByID(ctx, id) (*iampb.User, error)` — unchanged
- `iam.Service.HasTenantMember(ctx, userID, tenantID) (bool, error)` — unchanged

No further patches expected. If a downstream task hits another ratel mismatch, escalate again.
