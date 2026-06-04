package iam

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	===== Registration requests (closed-signup access requests) =====

	The counterpart to Register for when self-signup is closed: a prospective
	user leaves an email + optional note, admins triage it. See iam.proto.
*/

// SubmitRegistrationRequest is the PUBLIC access-request submission. It is the
// inverse of Register's gate: accepted only while self-registration is DISABLED
// (when it is enabled the visitor should just Register). Idempotent on email —
// a repeat submission refreshes the message on the existing PENDING row.
func (s *IamService) SubmitRegistrationRequest(ctx context.Context, req *api.SubmitRegistrationRequestRequest) (*api.SubmitRegistrationRequestResponse, error) {
	allowed, err := s.d.Gates.SelfRegistrationAllowed(ctx)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if allowed {
		return nil, status.Error(codes.FailedPrecondition, "self-registration is enabled; use Register")
	}

	email := strings.TrimSpace(req.GetEmail())

	rec, err := s.d.RegistrationRequests.GetByEmail(ctx, email)
	switch {
	case err == nil:
		// Existing row: refresh the note, keep it PENDING, leave id/created_at.
		rec.Message = req.GetMessage()
		rec.Status = api.RegistrationRequestStatus_REGISTRATION_REQUEST_STATUS_PENDING
		rec.UpdatedAt = s.now()
	case errors.Is(err, derrors.ErrNotFound):
		rec = &api.RegistrationRequest{
			Id:        uuid.NewString(),
			Email:     email,
			Message:   req.GetMessage(),
			Status:    api.RegistrationRequestStatus_REGISTRATION_REQUEST_STATUS_PENDING,
			CreatedAt: s.now(),
			UpdatedAt: s.now(),
		}
	default:
		return nil, utils.MapErr(err)
	}

	if err := s.doTx(ctx, func(ctx context.Context) error {
		return utils.MapErr(s.d.RegistrationRequests.Upsert(ctx, rec))
	}); err != nil {
		return nil, err
	}
	return &api.SubmitRegistrationRequestResponse{}, nil
}

// ListRegistrationRequests returns the access requests for admin triage,
// newest first. admin_only (enforced by the interceptor). An UNSPECIFIED
// status filter returns every request.
func (s *IamService) ListRegistrationRequests(ctx context.Context, req *api.ListRegistrationRequestsRequest) (*api.ListRegistrationRequestsResponse, error) {
	all, err := s.d.RegistrationRequests.List(ctx)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	filter := req.GetStatus()
	out := make([]*api.RegistrationRequest, 0, len(all))
	for _, r := range all {
		if filter != api.RegistrationRequestStatus_REGISTRATION_REQUEST_STATUS_UNSPECIFIED && r.GetStatus() != filter {
			continue
		}
		out = append(out, r)
	}
	return &api.ListRegistrationRequestsResponse{Requests: out}, nil
}

// MarkRegistrationRequestHandled flips a request to HANDLED and stamps the
// acting admin. admin_only. Idempotent: re-marking is a no-op.
func (s *IamService) MarkRegistrationRequestHandled(ctx context.Context, req *api.MarkRegistrationRequestHandledRequest) (*api.MarkRegistrationRequestHandledResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	rec, err := doTxRet(ctx, s, func(ctx context.Context) (*api.RegistrationRequest, error) {
		rec, err := s.d.RegistrationRequests.Get(ctx, req.GetId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		if rec.GetStatus() == api.RegistrationRequestStatus_REGISTRATION_REQUEST_STATUS_HANDLED {
			return rec, nil
		}
		rec.Status = api.RegistrationRequestStatus_REGISTRATION_REQUEST_STATUS_HANDLED
		rec.HandledByAccountId = caller.GetAccountId()
		rec.UpdatedAt = s.now()
		if err := s.d.RegistrationRequests.Update(ctx, rec); err != nil {
			return nil, utils.MapErr(err)
		}
		return rec, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.MarkRegistrationRequestHandledResponse{Request: rec}, nil
}
