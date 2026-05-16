package valkey

import (
	"context"
	"errors"
	"time"

	valkeygo "github.com/valkey-io/valkey-go"
)

// impl is the internal interface that both the real valkey adapter and the
// in-memory test implementation satisfy.
type impl interface {
	get(ctx context.Context, key string) (string, error)
	set(ctx context.Context, key, value string, ttl time.Duration) error
	setNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	del(ctx context.Context, key string) error
	close()
}

// Client is a thin convenience wrapper over an impl (either real valkey or in-memory).
type Client struct {
	impl impl
}

func NewClient(raw valkeygo.Client) *Client { return &Client{impl: &valkeyImpl{raw: raw}} }

// Close closes the underlying connection.
func (c *Client) Close() { c.impl.close() }

// Get returns "" + nil on cache miss (Nil reply).
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.impl.get(ctx, key)
}

func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("valkey: ttl required")
	}
	return c.impl.set(ctx, key, value, ttl)
}

// SetNX returns (acquired, error). True iff the key was newly set.
func (c *Client) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		return false, errors.New("valkey: ttl required")
	}
	return c.impl.setNX(ctx, key, value, ttl)
}

func (c *Client) Del(ctx context.Context, key string) error {
	return c.impl.del(ctx, key)
}

// valkeyImpl adapts valkeygo.Client to the impl interface.
type valkeyImpl struct {
	raw valkeygo.Client
}

func (v *valkeyImpl) get(ctx context.Context, key string) (string, error) {
	res := v.raw.Do(ctx, v.raw.B().Get().Key(key).Build())
	if err := res.Error(); err != nil {
		if valkeygo.IsValkeyNil(err) {
			return "", nil
		}
		return "", err
	}
	return res.ToString()
}

func (v *valkeyImpl) set(ctx context.Context, key, value string, ttl time.Duration) error {
	return v.raw.Do(ctx, v.raw.B().Set().Key(key).Value(value).Px(ttl).Build()).Error()
}

func (v *valkeyImpl) setNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	res := v.raw.Do(ctx, v.raw.B().Set().Key(key).Value(value).Nx().Px(ttl).Build())
	if err := res.Error(); err != nil {
		if valkeygo.IsValkeyNil(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (v *valkeyImpl) del(ctx context.Context, key string) error {
	return v.raw.Do(ctx, v.raw.B().Del().Key(key).Build()).Error()
}

func (v *valkeyImpl) close() { v.raw.Close() }
