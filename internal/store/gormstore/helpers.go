package gormstore

import (
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// newUUID mints a fresh record id.
func newUUID() string { return uuid.NewString() }

// timestamppbNow is the current wall-clock time as a protobuf timestamp.
func timestamppbNow() *timestamppb.Timestamp { return timestamppb.New(time.Now()) }
