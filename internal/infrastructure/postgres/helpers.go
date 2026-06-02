package postgres

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
)

// marshalOpts/unmarshalOpts are the canonical protojson codecs used for every
// blob: deterministic output, tolerant input. Same codecs the GORM store used.
var (
	marshalOpts   = protojson.MarshalOptions{}
	unmarshalOpts = protojson.UnmarshalOptions{DiscardUnknown: true}
)

// marshal serializes a proto record to a JSONB blob ([]byte).
func marshal(m proto.Message) ([]byte, error) {
	return marshalOpts.Marshal(m)
}

// unmarshal decodes a JSONB blob into a proto record.
func unmarshal(b []byte, m proto.Message) error {
	return unmarshalOpts.Unmarshal(b, m)
}

// translatePgErr maps pgx.ErrNoRows onto the domain not-found so services
// translate it to gRPC NotFound; everything else passes through.
func translatePgErr(resource string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return derrors.NotFound(resource, "not found")
	}
	return err
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505), used to translate a duplicate insert into the
// domain Conflict the GORM store raised on gorm.ErrDuplicatedKey.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// newUUID mints a fresh record id.
func newUUID() string { return uuid.NewString() }

// timestamppbNow is the current wall-clock time as a protobuf timestamp.
func timestamppbNow() *timestamppb.Timestamp { return timestamppb.New(time.Now()) }
