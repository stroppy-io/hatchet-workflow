package identity

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func newTicketService(t *testing.T) *IdeTicketService {
	t.Helper()
	s, err := NewIdeTicketService(Config{SigningSecret: "test-secret-value-1234567890"})
	if err != nil {
		t.Fatalf("NewIdeTicketService: %v", err)
	}
	return s
}

func TestIdeTicketService_MintAndConsume_RoundTrips(t *testing.T) {
	s := newTicketService(t)
	ticket, err := s.MintTicket("acc-1", "org:acme:provider:pg")
	if err != nil {
		t.Fatalf("MintTicket: %v", err)
	}
	accountID, scope, err := s.ConsumeTicket(context.Background(), ticket)
	if err != nil {
		t.Fatalf("ConsumeTicket: %v", err)
	}
	if accountID != "acc-1" || scope != "org:acme:provider:pg" {
		t.Fatalf("got (%q, %q)", accountID, scope)
	}
}

func TestIdeTicketService_ConsumeTicket_SingleUse(t *testing.T) {
	s := newTicketService(t)
	ticket, err := s.MintTicket("acc-1", "instance:provider:docker")
	if err != nil {
		t.Fatalf("MintTicket: %v", err)
	}
	if _, _, err := s.ConsumeTicket(context.Background(), ticket); err != nil {
		t.Fatalf("first ConsumeTicket: %v", err)
	}
	if _, _, err := s.ConsumeTicket(context.Background(), ticket); err == nil {
		t.Fatal("replaying a consumed ticket must fail — tickets are single-use")
	}
}

func TestIdeTicketService_ConsumeTicket_RejectsExpired(t *testing.T) {
	s := newTicketService(t)
	now := time.Now()
	claims := ideTokenClaims{
		Purpose: ideTicketPurpose,
		Scope:   "instance:provider:docker",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "acc-1",
			Issuer:    s.issuer,
			ID:        "expired-jti",
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Minute)),
			NotBefore: jwt.NewNumericDate(now.Add(-2 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, _, err := s.ConsumeTicket(context.Background(), tok); err == nil {
		t.Fatal("an expired ticket must be rejected")
	}
}

func TestIdeTicketService_ConsumeTicket_RejectsSessionToken(t *testing.T) {
	s := newTicketService(t)
	session, err := s.MintSession("acc-1", "instance:provider:docker")
	if err != nil {
		t.Fatalf("MintSession: %v", err)
	}
	if _, _, err := s.ConsumeTicket(context.Background(), session); err == nil {
		t.Fatal("a session token must not be consumable as a ticket — the two purposes are not interchangeable")
	}
}

func TestIdeTicketService_VerifySession_RejectsTicketToken(t *testing.T) {
	s := newTicketService(t)
	ticket, err := s.MintTicket("acc-1", "instance:provider:docker")
	if err != nil {
		t.Fatalf("MintTicket: %v", err)
	}
	if _, _, err := s.VerifySession(context.Background(), ticket); err == nil {
		t.Fatal("a ticket must not verify as a session — the two purposes are not interchangeable")
	}
}

func TestIdeTicketService_VerifySession_ReusableUntilExpiry(t *testing.T) {
	s := newTicketService(t)
	session, err := s.MintSession("acc-1", "instance:provider:docker")
	if err != nil {
		t.Fatalf("MintSession: %v", err)
	}
	for i := 0; i < 3; i++ {
		accountID, scope, err := s.VerifySession(context.Background(), session)
		if err != nil {
			t.Fatalf("VerifySession call %d: %v", i, err)
		}
		if accountID != "acc-1" || scope != "instance:provider:docker" {
			t.Fatalf("call %d: got (%q, %q)", i, accountID, scope)
		}
	}
}

func TestIdeTicketService_ConsumeTicket_RejectsForeignSignature(t *testing.T) {
	s := newTicketService(t)
	other, err := NewIdeTicketService(Config{SigningSecret: "a-completely-different-secret"})
	if err != nil {
		t.Fatalf("NewIdeTicketService: %v", err)
	}
	ticket, err := other.MintTicket("acc-1", "instance:provider:docker")
	if err != nil {
		t.Fatalf("MintTicket: %v", err)
	}
	if _, _, err := s.ConsumeTicket(context.Background(), ticket); err == nil {
		t.Fatal("a ticket signed with a different secret must be rejected")
	}
}

func TestNewIdeTicketService_RejectsEmptySecret(t *testing.T) {
	if _, err := NewIdeTicketService(Config{}); err == nil {
		t.Fatal("an empty signing secret must be rejected")
	}
}
