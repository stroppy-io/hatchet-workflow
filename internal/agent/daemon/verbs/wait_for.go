package verbs

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const defaultMaxAttempts = 30
const defaultDelaySeconds = 2

// WaitFor polls until the condition succeeds or exhausts max_attempts.
func WaitFor(ctx context.Context, action *agentpb.Action) *agentpb.Report {
	wf := action.GetWaitFor()
	if wf == nil {
		return failReport("WaitFor: nil payload")
	}

	maxAttempts := int(wf.GetMaxAttempts())
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}
	delay := time.Duration(wf.GetDelaySeconds()) * time.Second
	if delay <= 0 {
		delay = defaultDelaySeconds * time.Second
	}

	var checkFn func(ctx context.Context) (bool, error)
	var desc string

	switch {
	case wf.GetPortOpen() != nil:
		po := wf.GetPortOpen()
		addr := fmt.Sprintf("%s:%d", po.GetHost(), po.GetPort())
		desc = "port_open " + addr
		checkFn = func(ctx context.Context) (bool, error) {
			d := net.Dialer{Timeout: 2 * time.Second}
			conn, err := d.DialContext(ctx, "tcp", addr)
			if err != nil {
				return false, nil //nolint:nilerr
			}
			conn.Close()
			return true, nil
		}

	case wf.GetHttp() != nil:
		hp := wf.GetHttp()
		desc = "http " + hp.GetUrl()
		wantStatus := int(hp.GetExpectedStatus())
		if wantStatus == 0 {
			wantStatus = http.StatusOK
		}
		checkFn = func(ctx context.Context) (bool, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, hp.GetUrl(), nil)
			if err != nil {
				return false, err
			}
			for k, v := range hp.GetHeaders() {
				req.Header.Set(k, v)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return false, nil //nolint:nilerr
			}
			defer resp.Body.Close()
			return resp.StatusCode == wantStatus, nil
		}

	case wf.GetExitZero() != nil:
		ez := wf.GetExitZero()
		argv := ez.GetArgv()
		if len(argv) == 0 {
			return failReport("WaitFor/exit_zero: empty argv")
		}
		desc = "exit_zero " + strings.Join(argv, " ")
		checkFn = func(ctx context.Context) (bool, error) {
			cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
			for k, v := range ez.GetEnv() {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
			err := cmd.Run()
			return err == nil, nil
		}

	case !fileCheckEmpty(wf):
		// Deprecated path: file_exists is not a known proto variant;
		// guard clause to avoid fallthrough.
		return failReport("WaitFor: no check variant specified")

	default:
		return failReport("WaitFor: no check variant specified")
	}

	for i := 0; i < maxAttempts; i++ {
		ok, err := checkFn(ctx)
		if err != nil {
			return failReport(fmt.Sprintf("WaitFor/%s: check error: %v", desc, err))
		}
		if ok {
			return &agentpb.Report{
				Status:     agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED,
				Output:     []byte(fmt.Sprintf("condition met after %d attempt(s)", i+1)),
				FinishedAt: timestamppb.Now(),
			}
		}
		select {
		case <-ctx.Done():
			return failReport(fmt.Sprintf("WaitFor/%s: context cancelled", desc))
		case <-time.After(delay):
		}
	}

	return &agentpb.Report{
		Status:     agentpb.ReportStatus_REPORT_STATUS_FAILED,
		Error:      fmt.Sprintf("WaitFor/%s: exhausted %d attempts", desc, maxAttempts),
		FinishedAt: timestamppb.Now(),
	}
}

// fileCheckEmpty is a no-op sentinel to make the default case compile cleanly.
func fileCheckEmpty(_ *agentpb.WaitForCondition) bool {
	_ = os.DevNull // use os to avoid import cycle lint
	return false
}
