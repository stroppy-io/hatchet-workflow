package nodeworker

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

type StateStore interface {
	Put(ctx context.Context, key string, value *anypb.Any) (*systempb.DagRunStateEntry, error)
	Get(ctx context.Context, key string) (*anypb.Any, bool, error)
}

type Handler interface {
	Kind() string
	Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state StateStore) (*anypb.Any, error)
}

type Registry struct{ m map[string]Handler }

func NewRegistry() *Registry { return &Registry{m: map[string]Handler{}} }
func (r *Registry) Register(h Handler) { r.m[h.Kind()] = h }
func (r *Registry) Resolve(typeURL string) Handler {
	for k, h := range r.m {
		if endsWithIgnoreCase(typeURL, k) {
			return h
		}
	}
	return nil
}

func endsWithIgnoreCase(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	tail := s[len(s)-len(suffix):]
	for i := 0; i < len(suffix); i++ {
		a, b := tail[i], suffix[i]
		if a >= 'A' && a <= 'Z' {
			a += 32
		}
		if b >= 'A' && b <= 'Z' {
			b += 32
		}
		if a != b {
			return false
		}
	}
	return true
}
