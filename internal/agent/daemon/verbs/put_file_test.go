package verbs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

func TestPutFile_Inline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")

	action := &agentpb.Action{
		Verb: &agentpb.Action_PutFile{
			PutFile: &agentpb.PutFile{
				Path:    path,
				Content: &agentpb.PutFile_Inline{Inline: []byte("hello world")},
			},
		},
	}
	rep := PutFile(context.Background(), action)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		t.Fatalf("expected SUCCEEDED, got %v; error: %s", rep.Status, rep.Error)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "hello world" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestPutFile_CreateParents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c.txt")

	action := &agentpb.Action{
		Verb: &agentpb.Action_PutFile{
			PutFile: &agentpb.PutFile{
				Path:          path,
				Content:       &agentpb.PutFile_Inline{Inline: []byte("data")},
				CreateParents: true,
			},
		},
	}
	rep := PutFile(context.Background(), action)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		t.Fatalf("expected SUCCEEDED, got %v; error: %s", rep.Status, rep.Error)
	}
}

func TestPutFile_NoContent(t *testing.T) {
	dir := t.TempDir()
	action := &agentpb.Action{
		Verb: &agentpb.Action_PutFile{
			PutFile: &agentpb.PutFile{Path: filepath.Join(dir, "f.txt")},
		},
	}
	rep := PutFile(context.Background(), action)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_FAILED {
		t.Fatalf("expected FAILED for missing content, got %v", rep.Status)
	}
}
