package valkey

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type JSONCache[T any] struct {
	cli *Client
	ns  string
}

func NewJSONCache[T any](cli *Client, namespace string) *JSONCache[T] {
	return &JSONCache[T]{cli: cli, ns: namespace}
}

func (c *JSONCache[T]) key(k string) string { return c.ns + ":" + k }

func (c *JSONCache[T]) Get(ctx context.Context, k string) (T, bool, error) {
	var zero T
	raw, err := c.cli.Get(ctx, c.key(k))
	if err != nil {
		return zero, false, fmt.Errorf("valkey cache get: %w", err)
	}
	if raw == "" {
		return zero, false, nil
	}
	var v T
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return zero, false, fmt.Errorf("valkey cache decode: %w", err)
	}
	return v, true, nil
}

func (c *JSONCache[T]) Set(ctx context.Context, k string, v T, ttl time.Duration) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.cli.Set(ctx, c.key(k), string(raw), ttl)
}
