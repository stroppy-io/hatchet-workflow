package pgcontainer

import (
	"database/sql/driver"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
)

// anypbCodec is a pgx codec that encodes/decodes *anypb.Any as a JSON text
// string in the PostgreSQL text format. This is needed because the ratel-
// generated NodeRunScanner stores Output as *anypb.Any in a text NOT NULL
// column, but pgx v5 has no built-in encoding/decoding for proto.Message
// types in text columns.
type anypbCodec struct{}

func (anypbCodec) FormatSupported(format int16) bool {
	return format == pgtype.TextFormatCode
}

func (anypbCodec) PreferredFormat() int16 { return pgtype.TextFormatCode }

func (anypbCodec) PlanEncode(_ *pgtype.Map, _ uint32, format int16, value any) pgtype.EncodePlan {
	if format != pgtype.TextFormatCode {
		return nil
	}
	switch value.(type) {
	case *anypb.Any:
		return encodePlanAnyPb{}
	}
	return nil
}

func (anypbCodec) PlanScan(_ *pgtype.Map, _ uint32, format int16, target any) pgtype.ScanPlan {
	if format != pgtype.TextFormatCode {
		return nil
	}
	switch target.(type) {
	case **anypb.Any:
		return scanPlanAnyPb{}
	}
	return nil
}

func (anypbCodec) DecodeDatabaseSQLValue(_ *pgtype.Map, _ uint32, _ int16, src []byte) (driver.Value, error) {
	if src == nil {
		return nil, nil
	}
	if len(src) == 0 {
		return "", nil
	}
	return string(src), nil
}

func (anypbCodec) DecodeValue(_ *pgtype.Map, _ uint32, _ int16, src []byte) (any, error) {
	if src == nil {
		return nil, nil
	}
	var a anypb.Any
	if err := protojson.Unmarshal(src, &a); err != nil {
		return nil, fmt.Errorf("anypbCodec: decode value: %w", err)
	}
	return &a, nil
}

type encodePlanAnyPb struct{}

func (encodePlanAnyPb) Encode(value any, buf []byte) ([]byte, error) {
	a, ok := value.(*anypb.Any)
	if !ok {
		return nil, fmt.Errorf("anypbCodec: expected *anypb.Any, got %T", value)
	}
	if a == nil {
		return buf, nil
	}
	data, err := protojson.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("anypbCodec: marshal: %w", err)
	}
	return append(buf, data...), nil
}

type scanPlanAnyPb struct{}

func (scanPlanAnyPb) Scan(src []byte, dst any) error {
	target, ok := dst.(**anypb.Any)
	if !ok {
		return fmt.Errorf("anypbCodec: expected **anypb.Any, got %T", dst)
	}
	if src == nil || len(src) == 0 {
		*target = nil
		return nil
	}
	var a anypb.Any
	if err := protojson.Unmarshal(src, &a); err != nil {
		// If unmarshal fails (e.g. invalid JSON or empty string), return nil
		*target = nil
		return nil
	}
	*target = &a
	return nil
}
