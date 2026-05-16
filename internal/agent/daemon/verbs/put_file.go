package verbs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PutFile writes the file described by the PutFile action atomically via
// temp-file + rename.  Fetch variant downloads from the given URL.
func PutFile(ctx context.Context, action *agentpb.Action) *agentpb.Report {
	pf := action.GetPutFile()
	if pf == nil {
		return failReport("PutFile: nil payload")
	}

	path := pf.GetPath()
	if path == "" {
		return failReport("PutFile: empty path")
	}

	if pf.GetCreateParents() {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return failReport(fmt.Sprintf("PutFile: mkdir -p %s: %v", filepath.Dir(path), err))
		}
	}

	var content []byte
	switch {
	case pf.GetInline() != nil:
		content = pf.GetInline()
	case pf.GetFetch() != nil:
		data, err := fetchURL(ctx, pf.GetFetch().GetUrl())
		if err != nil {
			return failReport(fmt.Sprintf("PutFile: fetch %s: %v", pf.GetFetch().GetUrl(), err))
		}
		content = data
	default:
		return failReport("PutFile: no content (inline or fetch required)")
	}

	mode := os.FileMode(pf.GetMode())
	if mode == 0 {
		mode = 0o600
	}

	// Atomic write via temp + rename.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, mode); err != nil {
		return failReport(fmt.Sprintf("PutFile: write tmp %s: %v", tmp, err))
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return failReport(fmt.Sprintf("PutFile: rename %s → %s: %v", tmp, path, err))
	}

	return &agentpb.Report{
		Status:     agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED,
		FinishedAt: timestamppb.Now(),
	}
}
