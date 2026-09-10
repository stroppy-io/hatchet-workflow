package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	iamsdk "github.com/gopherex/iam/pkg/sdk"
	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/profile"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/repositories"
)

const maxIAMWebhookBody = 1 << 20

// iamWebhook receives IAM lifecycle events: session.revoked feeds the
// denylist, email.changed and user.deleted keep profiles in step.
// Delivery is at-least-once — events are deduped by id.
type iamWebhook struct {
	verifier *iamsdk.WebhookVerifier
	cfg      *IAMConfig
	iam      *repositories.IAMRepo
	profiles *profile.Service
	log      *xlog.Logger
}

func newIAMWebhook(cfg *IAMConfig, iam *repositories.IAMRepo, profiles *profile.Service, log *xlog.Logger) (http.Handler, error) {
	if !cfg.WebhookEnabled() {
		return nil, nil //nolint:nilnil // disabled optional endpoint = no handler
	}
	verifier, err := iamsdk.NewWebhookVerifier(iamsdk.WebhookVerifierConfig{SigningSecret: cfg.WebhookSigningSecret})
	if err != nil {
		return nil, fmt.Errorf("iam webhook verifier: %w", err)
	}
	return &iamWebhook{verifier: verifier, cfg: cfg, iam: iam, profiles: profiles, log: log}, nil
}

func (h *iamWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxIAMWebhookBody))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	event, err := h.verifier.Verify(r.Header, body)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, iamsdk.ErrWebhookMalformedBody) {
			status = http.StatusBadRequest
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	if event.ProjectID != h.cfg.ProjectID || event.Environment != h.cfg.Environment {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	ctx := r.Context()
	fresh, err := h.iam.RecordWebhookEvent(ctx, event.ID)
	if err != nil {
		h.fail(ctx, w, event.Type, err)
		return
	}
	if !fresh {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := h.handle(ctx, event); err != nil {
		if errors.Is(err, iamsdk.ErrWebhookMalformedBody) {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		h.fail(ctx, w, event.Type, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *iamWebhook) handle(ctx context.Context, event *iamsdk.WebhookEvent) error {
	switch event.Type {
	case "session.revoked":
		data, err := event.SessionRevokedData()
		if err != nil {
			return err
		}
		user, err := uuid.Parse(data.UserID)
		if err != nil {
			return err
		}
		return h.iam.DenySession(ctx, data.SessionID, user, time.Now().Add(h.cfg.AccessTTL))
	case "email.changed":
		var data struct {
			UserID string `json:"user_id"`
			Email  string `json:"email"`
		}
		if err := event.DecodeData(&data); err != nil {
			return err
		}
		user, err := uuid.Parse(data.UserID)
		if err != nil {
			return err
		}
		return h.profiles.EmailChanged(ctx, user, data.Email)
	case "user.deleted":
		var data struct {
			UserID string `json:"user_id"`
		}
		if err := event.DecodeData(&data); err != nil {
			return err
		}
		user, err := uuid.Parse(data.UserID)
		if err != nil {
			return err
		}
		return h.profiles.UserDeleted(ctx, user)
	default:
		return nil
	}
}

func (h *iamWebhook) fail(ctx context.Context, w http.ResponseWriter, kind string, err error) {
	h.log.Ctx().Error(ctx, "iam webhook failed", xlog.String("event", kind), xlog.ErrorCause(err))
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
