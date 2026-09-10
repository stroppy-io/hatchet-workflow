// Package tenant is the organization: the tenant record, membership with
// fixed roles, invites and ownership. It owns the Graphene namespace of the
// tenant (created with the record, retired with it) but nothing that runs
// inside it.
package tenant

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Role is one of the four fixed roles.
type Role string

// Roles, strongest first.
const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

var roleRank = map[Role]int{RoleOwner: 4, RoleAdmin: 3, RoleMember: 2, RoleViewer: 1}

// Valid reports a known role.
func (r Role) Valid() bool { return roleRank[r] > 0 }

// AtLeast reports r >= required.
func (r Role) AtLeast(required Role) bool { return roleRank[r] >= roleRank[required] }

// Min returns the weaker of two roles.
func (r Role) Min(o Role) Role {
	if roleRank[o] < roleRank[r] {
		return o
	}
	return r
}

// Status of a tenant.
type Status string

// Statuses.
const (
	StatusActive    Status = "active"
	StatusOrphaned  Status = "orphaned"
	StatusSuspended Status = "suspended"
)

// Tenant is the record.
type Tenant struct {
	ID                uuid.UUID
	Slug              string
	Name              string
	Description       string
	PublicName        *string
	Status            Status
	OwnerID           uuid.UUID
	GrapheneNamespace string
	MemberCount       int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Membership is a tenant seen from one member.
type Membership struct {
	Tenant   Tenant
	Role     Role
	JoinedAt time.Time
}

// Member is one person in a tenant.
type Member struct {
	UserID      uuid.UUID
	DisplayName string
	Avatar      string
	Email       string
	Role        Role
	JoinedAt    time.Time
	LastSeenAt  *time.Time
}

// InviteStatus of an invite.
type InviteStatus string

// Invite statuses.
const (
	InvitePending  InviteStatus = "pending"
	InviteAccepted InviteStatus = "accepted"
	InviteDeclined InviteStatus = "declined"
	InviteExpired  InviteStatus = "expired"
	InviteRevoked  InviteStatus = "revoked"
)

// Invite is an offer to join by email.
type Invite struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	Email         string
	Role          Role
	Status        InviteStatus
	InvitedBy     *uuid.UUID
	InviterName   string
	InviterAvatar string
	Message       string
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

// Create is the creation request.
type Create struct {
	Name        string
	Slug        string // empty = derived from Name
	Description string
}

// Patch is a partial update; nil = keep.
type Patch struct {
	Name            *string
	Description     *string
	PublicName      *string
	ClearPublicName bool
}

// Repository is the storage port.
type Repository interface {
	Insert(ctx context.Context, t Tenant) error
	BySlug(ctx context.Context, slug string) (Tenant, error)
	ByID(ctx context.Context, id uuid.UUID) (Tenant, error)
	OfUser(ctx context.Context, userID uuid.UUID) ([]Membership, error)
	OwnedBy(ctx context.Context, userID uuid.UUID) (uuid.UUID, string, bool, error)
	SlugTaken(ctx context.Context, slug string) (bool, error)
	Update(ctx context.Context, id uuid.UUID, p Patch) error
	SetOwner(ctx context.Context, id, ownerID uuid.UUID) error
	SoftDelete(ctx context.Context, id uuid.UUID) error

	UpsertMember(ctx context.Context, tenantID, userID uuid.UUID, role Role) error
	MemberRole(ctx context.Context, tenantID, userID uuid.UUID) (Role, bool, error)
	Members(ctx context.Context, tenantID uuid.UUID) ([]Member, error)
	SetMemberRole(ctx context.Context, tenantID, userID uuid.UUID, role Role) error
	TouchMember(ctx context.Context, tenantID, userID uuid.UUID) error
	DeleteMember(ctx context.Context, tenantID, userID uuid.UUID) error

	InsertInvite(ctx context.Context, inv Invite) error
	InviteByID(ctx context.Context, id uuid.UUID) (Invite, error)
	PendingInvites(ctx context.Context, tenantID uuid.UUID) ([]Invite, error)
	PendingInvitesForEmail(ctx context.Context, email string) ([]Invite, error)
	ResolveInvite(ctx context.Context, id uuid.UUID, status InviteStatus) (bool, error)
}

// Namespaces is the Graphene side of a tenant.
type Namespaces interface {
	EnsureNamespace(ctx context.Context, name string, labels map[string]string) error
	DeleteNamespace(ctx context.Context, name string) error
}

// Pipelines publishes the pipeline binaries into a fresh namespace (§7).
type Pipelines interface {
	Sync(ctx context.Context, namespace string) bool
	Forget(ctx context.Context, namespace string)
}

// Tokens is what membership changes must do to personal tokens.
type Tokens interface {
	RevokeOfMember(ctx context.Context, tenantID, userID uuid.UUID) error
}

// Mailer sends the invite e-mail. Nil = no mail.
type Mailer interface {
	SendInvite(ctx context.Context, to string, t Tenant, inv Invite) error
}

// Transactor runs fn inside one storage transaction.
type Transactor interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// LiveRuns answers "does the tenant have runs in flight" — a delete guard.
// Nil-safe: without an implementation nothing is live.
type LiveRuns interface {
	HasLiveRuns(ctx context.Context, tenantID uuid.UUID) (bool, error)
}
