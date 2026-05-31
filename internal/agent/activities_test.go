package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

func newTestActivities(t *testing.T) *Activities {
	t.Helper()
	return NewActivities(
		WithCacheDir(filepath.Join(t.TempDir(), "cache")),
		WithHeartbeater(func(context.Context, ...any) {}),
	)
}

func TestCallCmd_EchoArgv(t *testing.T) {
	a := newTestActivities(t)
	res, err := a.CallCmdActivity(context.Background(), &common.Cmd{
		Spec: &common.Cmd_Spec{
			Command: &common.Cmd_Spec_Argv{
				Argv: &common.Cmd_Argv{Args: []string{"echo", "-n", "hello world"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallCmd error: %v", err)
	}
	if res.GetExitCode() != 0 {
		t.Fatalf("exit_code = %d, want 0", res.GetExitCode())
	}
	if got := string(res.GetStdout()); got != "hello world" {
		t.Fatalf("stdout = %q, want %q", got, "hello world")
	}
	if res.GetTimedOut() {
		t.Fatalf("timed_out = true, want false")
	}
	if res.GetElapsed() == nil {
		t.Fatalf("elapsed is nil")
	}
}

func TestCallCmd_Script_NonZeroExit(t *testing.T) {
	a := newTestActivities(t)
	res, err := a.CallCmdActivity(context.Background(), &common.Cmd{
		Spec: &common.Cmd_Spec{
			Command: &common.Cmd_Spec_Script{
				Script: &common.Cmd_Script{Text: "echo out; echo err 1>&2; exit 7"},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallCmd error: %v", err)
	}
	if res.GetExitCode() != 7 {
		t.Fatalf("exit_code = %d, want 7", res.GetExitCode())
	}
	if string(res.GetStdout()) != "out\n" {
		t.Fatalf("stdout = %q, want %q", res.GetStdout(), "out\n")
	}
	if string(res.GetStderr()) != "err\n" {
		t.Fatalf("stderr = %q, want %q", res.GetStderr(), "err\n")
	}
}

func TestCallCmd_Timeout(t *testing.T) {
	a := newTestActivities(t)
	res, err := a.CallCmdActivity(context.Background(), &common.Cmd{
		Spec: &common.Cmd_Spec{
			Command: &common.Cmd_Spec_Script{
				Script: &common.Cmd_Script{Text: "sleep 5"},
			},
			Timeout: durationpb.New(200_000_000), // 200ms
		},
	})
	if err != nil {
		t.Fatalf("CallCmd error: %v", err)
	}
	if !res.GetTimedOut() {
		t.Fatalf("timed_out = false, want true")
	}
}

func TestWriteFile(t *testing.T) {
	a := newTestActivities(t)
	dst := filepath.Join(t.TempDir(), "sub", "out.txt")

	if err := a.WriteFileActivity(context.Background(), &common.File{
		Info:    &common.File_Info{Path: dst, Mode: 0o640},
		Content: &common.File_Text{Text: "payload"},
	}); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "payload" {
		t.Fatalf("content = %q, want %q", got, "payload")
	}
	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %v, want 0640", fi.Mode().Perm())
	}
}

func TestWriteFile_Bytes(t *testing.T) {
	a := newTestActivities(t)
	dst := filepath.Join(t.TempDir(), "b.bin")
	want := []byte{0x00, 0x01, 0x02, 0xff}
	if err := a.WriteFileActivity(context.Background(), &common.File{
		Info:    &common.File_Info{Path: dst},
		Content: &common.File_Bytes{Bytes: want},
	}); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != string(want) {
		t.Fatalf("bytes mismatch")
	}
}

func TestFetchFile_DownloadVerifyAndCache(t *testing.T) {
	payload := []byte("#!/bin/sh\necho stroppy\n")
	sum := sha256.Sum256(payload)
	checksum := hex.EncodeToString(sum[:])

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	cacheDir := filepath.Join(t.TempDir(), "cache")
	a := NewActivities(
		WithCacheDir(cacheDir),
		WithHeartbeater(func(context.Context, ...any) {}),
		WithHTTPClient(srv.Client()),
	)

	dst := filepath.Join(t.TempDir(), "bin", "stroppy")
	req := &common.File{
		Info: &common.File_Info{Path: dst, Mode: 0o755},
		Content: &common.File_AsRef_{AsRef: &common.File_AsRef{
			Uri:      srv.URL + "/stroppy",
			Checksum: checksum,
		}},
	}

	// First fetch downloads.
	if err := a.FetchFileActivity(context.Background(), req); err != nil {
		t.Fatalf("FetchFile (1) error: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read fetched: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("content mismatch")
	}
	fi, _ := os.Stat(dst)
	if fi.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, want 0755 (executable)", fi.Mode().Perm())
	}
	// Cached by checksum.
	if _, err := os.Stat(filepath.Join(cacheDir, checksum)); err != nil {
		t.Fatalf("expected cache entry for checksum: %v", err)
	}
	if hits != 1 {
		t.Fatalf("server hits = %d, want 1 after first fetch", hits)
	}

	// Second fetch to a new path should hit the cache, not the server.
	dst2 := filepath.Join(t.TempDir(), "bin2", "stroppy")
	req.Info.Path = dst2
	if err := a.FetchFileActivity(context.Background(), req); err != nil {
		t.Fatalf("FetchFile (2) error: %v", err)
	}
	if hits != 1 {
		t.Fatalf("server hits = %d, want 1 (cache hit on 2nd fetch)", hits)
	}
	got2, _ := os.ReadFile(dst2)
	if string(got2) != string(payload) {
		t.Fatalf("cached content mismatch")
	}
}

func TestFetchFile_ChecksumMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("actual"))
	}))
	defer srv.Close()

	a := NewActivities(
		WithCacheDir(filepath.Join(t.TempDir(), "cache")),
		WithHeartbeater(func(context.Context, ...any) {}),
		WithHTTPClient(srv.Client()),
	)
	err := a.FetchFileActivity(context.Background(), &common.File{
		Info: &common.File_Info{Path: filepath.Join(t.TempDir(), "f")},
		Content: &common.File_AsRef_{AsRef: &common.File_AsRef{
			Uri:      srv.URL,
			Checksum: "deadbeef",
		}},
	})
	if err == nil {
		t.Fatalf("expected checksum mismatch error, got nil")
	}
}

func TestCreateTempDir(t *testing.T) {
	a := newTestActivities(t)
	base := t.TempDir()
	dir, err := a.CreateTempDirActivity(context.Background(), &common.Dir_Temp{
		Base:    base,
		Pattern: "work-*",
		Mode:    0o700,
	})
	if err != nil {
		t.Fatalf("CreateTempDir error: %v", err)
	}
	if dir.GetInfo().GetPath() == "" {
		t.Fatalf("empty temp dir path")
	}
	fi, err := os.Stat(dir.GetInfo().GetPath())
	if err != nil || !fi.IsDir() {
		t.Fatalf("temp dir not created: %v", err)
	}
	if fi.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %v, want 0700", fi.Mode().Perm())
	}
}

func TestCreateDir(t *testing.T) {
	a := newTestActivities(t)
	p := filepath.Join(t.TempDir(), "a", "b", "c")
	if err := a.CreateDirActivity(context.Background(), &common.Dir{
		Info:          &common.Dir_Info{Path: p, Mode: 0o750},
		CreateParents: true,
	}); err != nil {
		t.Fatalf("CreateDir error: %v", err)
	}
	fi, err := os.Stat(p)
	if err != nil || !fi.IsDir() {
		t.Fatalf("dir not created: %v", err)
	}
	if fi.Mode().Perm() != 0o750 {
		t.Fatalf("mode = %v, want 0750", fi.Mode().Perm())
	}
}

func TestEnsureAgentOnline(t *testing.T) {
	a := newTestActivities(t)
	if err := a.EnsureAgentOnlineActivity(context.Background()); err != nil {
		t.Fatalf("EnsureAgentOnline error: %v", err)
	}
}
