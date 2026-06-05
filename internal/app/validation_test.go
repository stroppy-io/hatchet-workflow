package app

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
)

func TestRequestValidationRejectsInvalidUnaryBeforeHandler(t *testing.T) {
	called := false
	next := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		return connect.NewResponse(&api.GetTestRunResponse{}), nil
	}

	_, err := (requestValidationInterceptor{}).WrapUnary(next)(
		context.Background(),
		connect.NewRequest(&api.GetTestRunRequest{}),
	)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want %s (err %v)", status.Code(err), codes.InvalidArgument, err)
	}
	if called {
		t.Fatal("handler was called for invalid request")
	}
}

func TestRequestValidationRejectsInvalidStreamingMessage(t *testing.T) {
	called := false
	next := func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		called = true
		var req api.GetTestRunRequest
		return conn.Receive(&req)
	}

	err := (requestValidationInterceptor{}).WrapStreamingHandler(next)(
		context.Background(),
		&fakeStreamingConn{msg: &api.GetTestRunRequest{}},
	)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want %s (err %v)", status.Code(err), codes.InvalidArgument, err)
	}
	if !called {
		t.Fatal("stream handler was not called")
	}
}

type fakeStreamingConn struct {
	msg any
}

func (c *fakeStreamingConn) Spec() connect.Spec {
	return connect.Spec{StreamType: connect.StreamTypeClient}
}
func (c *fakeStreamingConn) Peer() connect.Peer { return connect.Peer{} }

func (c *fakeStreamingConn) Receive(dst any) error {
	reflect.ValueOf(dst).Elem().Set(reflect.ValueOf(c.msg).Elem())
	return nil
}

func (c *fakeStreamingConn) RequestHeader() http.Header  { return http.Header{} }
func (c *fakeStreamingConn) Send(any) error              { return nil }
func (c *fakeStreamingConn) ResponseHeader() http.Header { return http.Header{} }
func (c *fakeStreamingConn) ResponseTrailer() http.Header {
	return http.Header{}
}
