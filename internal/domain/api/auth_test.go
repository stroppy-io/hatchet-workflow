package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func TestAuthMiddleware_MissingHeaderRejects(t *testing.T) {
	jwt := auth.NewJWTIssuer("test-secret")
	mw := auth.NewAuthMiddleware(jwt, nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/run/123/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_AgentPathRequiresAuth(t *testing.T) {
	jwtIssuer := auth.NewJWTIssuer("test-secret")
	mw := auth.NewAuthMiddleware(jwtIssuer, nil)
	handler := mw(okHandler())

	// Without token — should fail.
	req := httptest.NewRequest(http.MethodPost, "/api/agent/register", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for agent path without token, got %d", rec.Code)
	}

	// With valid JWT — should pass.
	token, _ := jwtIssuer.Issue(auth.Claims{UserID: "machine-1", TenantID: "t1", Role: "operator"}, time.Hour)
	req2 := httptest.NewRequest(http.MethodPost, "/api/agent/register", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for agent path with token, got %d", rec2.Code)
	}
}

func TestAuthMiddleware_HealthBypassesAuth(t *testing.T) {
	jwt := auth.NewJWTIssuer("test-secret")
	mw := auth.NewAuthMiddleware(jwt, nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /health, got %d", rec.Code)
	}
}

func TestAuthMiddleware_LoginBypassesAuth(t *testing.T) {
	jwt := auth.NewJWTIssuer("test-secret")
	mw := auth.NewAuthMiddleware(jwt, nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for login path, got %d", rec.Code)
	}
}
