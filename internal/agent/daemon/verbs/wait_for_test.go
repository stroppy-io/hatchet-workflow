package verbs

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

func TestWaitFor_PortOpen(t *testing.T) {
	// Start a real TCP listener.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	action := &agentpb.Action{
		Verb: &agentpb.Action_WaitFor{
			WaitFor: &agentpb.WaitForCondition{
				Check: &agentpb.WaitForCondition_PortOpen_{
					PortOpen: &agentpb.WaitForCondition_PortOpen{
						Host: "127.0.0.1",
						Port: uint32(addr.Port),
					},
				},
				MaxAttempts:  3,
				DelaySeconds: 0,
			},
		},
	}
	rep := WaitFor(context.Background(), action)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		t.Fatalf("expected SUCCEEDED, got %v; error: %s", rep.Status, rep.Error)
	}
}

func TestWaitFor_Http(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	action := &agentpb.Action{
		Verb: &agentpb.Action_WaitFor{
			WaitFor: &agentpb.WaitForCondition{
				Check: &agentpb.WaitForCondition_Http{
					Http: &agentpb.WaitForCondition_HttpProbe{
						Url:            srv.URL,
						ExpectedStatus: http.StatusOK,
					},
				},
				MaxAttempts:  3,
				DelaySeconds: 0,
			},
		},
	}
	rep := WaitFor(context.Background(), action)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		t.Fatalf("expected SUCCEEDED, got %v; error: %s", rep.Status, rep.Error)
	}
}

func TestWaitFor_ExitZero(t *testing.T) {
	action := &agentpb.Action{
		Verb: &agentpb.Action_WaitFor{
			WaitFor: &agentpb.WaitForCondition{
				Check: &agentpb.WaitForCondition_ExitZero{
					ExitZero: &agentpb.WaitForCondition_ShellExitZero{
						Argv: []string{"true"},
					},
				},
				MaxAttempts:  2,
				DelaySeconds: 0,
			},
		},
	}
	rep := WaitFor(context.Background(), action)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		t.Fatalf("expected SUCCEEDED, got %v; error: %s", rep.Status, rep.Error)
	}
}

func TestWaitFor_Exhausted(t *testing.T) {
	action := &agentpb.Action{
		Verb: &agentpb.Action_WaitFor{
			WaitFor: &agentpb.WaitForCondition{
				Check: &agentpb.WaitForCondition_ExitZero{
					ExitZero: &agentpb.WaitForCondition_ShellExitZero{
						Argv: []string{"false"},
					},
				},
				MaxAttempts:  2,
				DelaySeconds: 0,
			},
		},
	}
	rep := WaitFor(context.Background(), action)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_FAILED {
		t.Fatalf("expected FAILED when exhausted, got %v", rep.Status)
	}
}
