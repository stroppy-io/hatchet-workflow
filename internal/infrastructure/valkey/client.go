package valkey

import (
	"context"
	"errors"
	"time"

	valkeygo "github.com/valkey-io/valkey-go"
)

// Client is a thin convenience wrapper over valkey-go for the middleware use case.
type Client struct {
	raw valkeygo.Client
}

func NewClient(raw valkeygo.Client) *Client { return &Client{raw: raw} }

// Get returns "" + nil on cache miss (Nil reply).
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	res := c.raw.Do(ctx, c.raw.B().Get().Key(key).Build())
	if err := res.Error(); err != nil {
		if valkeygo.IsValkeyNil(err) {
			return "", nil
		}
		return "", err
	}
	return res.ToString()
}

func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("valkey: ttl required")
	}
	return c.raw.Do(ctx, c.raw.B().Set().Key(key).Value(value).Px(ttl).Build()).Error()
}

// SetNX returns (acquired, error). True iff the key was newly set.
func (c *Client) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		return false, errors.New("valkey: ttl required")
	}
	res := c.raw.Do(ctx, c.raw.B().Set().Key(key).Value(value).Nx().Px(ttl).Build())
	if err := res.Error(); err != nil {
		if valkeygo.IsValkeyNil(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) Del(ctx context.Context, key string) error {
	return c.raw.Do(ctx, c.raw.B().Del().Key(key).Build()).Error()
}
