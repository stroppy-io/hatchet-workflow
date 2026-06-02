package identity

import (
	"context"
	"log/slog"

	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
)

// SlogNotifier implements iamsvc.Notifier by logging the minted one-time tokens
// through slog. It is a real (non-fake) logging delivery channel: useful for
// dev/test and as a baseline before a transactional outbox+relay is wired. It
// does no synchronous network IO, so it is safe to call inside the ambient
// transaction.
type SlogNotifier struct {
	log *slog.Logger
}

var _ iamsvc.Notifier = (*SlogNotifier)(nil)

// NewSlogNotifier builds the notifier. A nil logger falls back to slog.Default.
func NewSlogNotifier(log *slog.Logger) *SlogNotifier {
	if log == nil {
		log = slog.Default()
	}
	return &SlogNotifier{log: log.With(slog.String("component", "iam.notifier"))}
}

func (n *SlogNotifier) SendEmailVerification(ctx context.Context, email, token string) error {
	n.log.InfoContext(ctx, "email verification token issued",
		slog.String("flow", "email_verification"),
		slog.String("email", email),
		slog.String("token", token),
	)
	return nil
}

func (n *SlogNotifier) SendPasswordReset(ctx context.Context, email, token string) error {
	n.log.InfoContext(ctx, "password reset token issued",
		slog.String("flow", "password_reset"),
		slog.String("email", email),
		slog.String("token", token),
	)
	return nil
}
