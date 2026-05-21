package logger //nolint:testpackage // test package

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestNewFromCtx(t *testing.T) { //nolint:paralleltest // global logger
	// Test with nil context
	if got := NewFromCtx(context.TODO()); got != globalLogger {
		t.Errorf("NewFromCtx(nil) = %v; want %v", got, globalLogger)
	}

	// Test with context containing logger
	logger := zap.NewNop()

	ctx := context.WithValue(context.Background(), ctxLoggerKey, logger)
	if got := NewFromCtx(ctx); got != logger {
		t.Errorf("NewFromCtx(ctx) = %v; want %v", got, logger)
	}
}

func TestWrapInCtx(t *testing.T) { //nolint:paralleltest // global logger
	logger := zap.NewNop()

	ctx := WrapInCtx(context.Background(), logger)
	if got := ctx.Value(ctxLoggerKey); got != logger {
		t.Errorf("WrapInCtx(ctx) = %v; want %v", got, logger)
	}
}

func TestCtxWithAttrs(t *testing.T) { //nolint:paralleltest // global logger
	ctx := CtxWithAttrs(context.Background(), zap.String("key", "value"))
	fields := GetCtxFields(ctx)

	if len(fields) != 1 {
		t.Errorf("Expected 1 field, got %d", len(fields))
	}

	if fields[0].Key != "key" || fields[0].String != "value" {
		t.Errorf(
			"Expected field key 'key' with value 'value', got key '%s' and value '%s'",
			fields[0].Key,
			fields[0].String,
		)
	}
}

func TestSetCtxFields(t *testing.T) { //nolint:paralleltest // global logger
	ctx := CtxWithAttrs(context.Background(), zap.String("initial", "value"))
	SetCtxFields(ctx, zap.String("new", "value"))
	fields := GetCtxFields(ctx)

	if len(fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(fields))
	}

	if fields[0].Key != "initial" || fields[0].String != "value" {
		t.Errorf(
			"Expected field key 'initial' with value 'value', got key '%s' and value '%s'",
			fields[0].Key,
			fields[0].String,
		)
	}

	if fields[1].Key != "new" || fields[1].String != "value" {
		t.Errorf(
			"Expected field key 'new' with value 'value', got key '%s' and value '%s'",
			fields[1].Key,
			fields[1].String,
		)
	}
}

func TestGetCtxFields(t *testing.T) { //nolint:paralleltest // global logger
	ctx := CtxWithAttrs(context.Background(), zap.String("key", "value"))
	fields := GetCtxFields(ctx)

	if len(fields) != 1 {
		t.Errorf("Expected 1 field, got %d", len(fields))
	}

	if fields[0].Key != "key" || fields[0].String != "value" {
		t.Errorf(
			"Expected field key 'key' with value 'value', got key '%s' and value '%s'",
			fields[0].Key,
			fields[0].String,
		)
	}
}

func TestWithCtxFields(t *testing.T) { //nolint:paralleltest // global logger
	ctx := CtxWithAttrs(context.Background(), zap.String("initial", "value"))
	newFields := WithCtxFields(ctx, zap.String("additional", "value"))

	if len(newFields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(newFields))
	}

	if newFields[0].Key != "initial" || newFields[0].String != "value" {
		t.Errorf(
			"Expected field key 'initial' with value 'value', got key '%s' and value '%s'",
			newFields[0].Key,
			newFields[0].String,
		)
	}

	if newFields[1].Key != "additional" || newFields[1].String != "value" {
		t.Errorf(
			"Expected field key 'additional' with value 'value', got key '%s' and value '%s'",
			newFields[1].Key,
			newFields[1].String,
		)
	}
}
