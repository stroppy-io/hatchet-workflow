package postgres

import (
	"context"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
)

/*
	===== RegistrationRequestRepo =====

	Access requests captured while self-signup is closed. Persisted as a
	protojson blob keyed by id, with email (unique) and status mirrored into
	indexed columns. email is the natural key — Upsert collapses repeat
	submissions onto one row.
*/

// RegistrationRequests returns the iam.RegistrationRequestRepo.
func (s *Store) RegistrationRequests() *RegistrationRequestRepo {
	return &RegistrationRequestRepo{db: s.db}
}

type RegistrationRequestRepo struct{ db *DB }

var _ iamsvc.RegistrationRequestRepo = (*RegistrationRequestRepo)(nil)

func (r *RegistrationRequestRepo) Upsert(ctx context.Context, req *api.RegistrationRequest) error {
	data, err := marshal(req)
	if err != nil {
		return err
	}
	return r.db.q().UpsertRegistrationRequest(ctx, dbgen.UpsertRegistrationRequestParams{
		ID:     req.GetId(),
		Email:  req.GetEmail(),
		Status: req.GetStatus().String(),
		Data:   data,
	})
}

func (r *RegistrationRequestRepo) Get(ctx context.Context, id string) (*api.RegistrationRequest, error) {
	row, err := r.db.q().GetRegistrationRequest(ctx, id)
	if err != nil {
		return nil, translatePgErr("registration_request", err)
	}
	return decodeRegistrationRequest(row.Data)
}

func (r *RegistrationRequestRepo) GetByEmail(ctx context.Context, email string) (*api.RegistrationRequest, error) {
	row, err := r.db.q().GetRegistrationRequestByEmail(ctx, email)
	if err != nil {
		return nil, translatePgErr("registration_request", err)
	}
	return decodeRegistrationRequest(row.Data)
}

func (r *RegistrationRequestRepo) List(ctx context.Context) ([]*api.RegistrationRequest, error) {
	rows, err := r.db.q().ListRegistrationRequests(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*api.RegistrationRequest, 0, len(rows))
	for _, row := range rows {
		rec, err := decodeRegistrationRequest(row.Data)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

func (r *RegistrationRequestRepo) Update(ctx context.Context, req *api.RegistrationRequest) error {
	data, err := marshal(req)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateRegistrationRequest(ctx, dbgen.UpdateRegistrationRequestParams{
		Status: req.GetStatus().String(),
		Data:   data,
		ID:     req.GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("registration_request", "registration request not found")
	}
	return nil
}

func decodeRegistrationRequest(data []byte) (*api.RegistrationRequest, error) {
	rec := &api.RegistrationRequest{}
	if err := unmarshal(data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}
