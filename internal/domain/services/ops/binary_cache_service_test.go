package ops_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/ops"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestBinaryCacheResolve(t *testing.T) {
	f := fixture.New(t)
	exec := f.Executor.(*sqlexec.TxExecutor)
	svc := ops.NewBinaryCacheService(exec, f.TxMgr)
	ctx := context.Background()

	now := time.Now().UTC()
	artifactID := ids.New()

	// Insert directly via raw SQL on the pool.
	_, err := f.Pool.Exec(ctx, `
		INSERT INTO binary_artifacts
			(id, created_at, updated_at, kind, name, version, filename, arch, os,
			 origin_url, storage_uri, sha_256, size_bytes, content_type, serve_count)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`,
		artifactID,
		now, now,
		opspb.BinaryArtifactKind_BINARY_ARTIFACT_KIND_AGENT.String(),
		"stroppy-agent",
		"v1.2.3",
		"stroppy-agent_linux_amd64",
		"amd64",
		"linux",
		"https://github.com/example/releases/v1.2.3",
		"s3://bucket/stroppy-agent/v1.2.3/stroppy-agent_linux_amd64",
		"abc123deadbeef",
		int64(1234567),
		"application/octet-stream",
		int64(0),
	)
	require.NoError(t, err)

	// Resolve hit.
	resp, err := svc.Resolve(ctx, &agentpb.ResolveArtifactRequest{
		Name:     "stroppy-agent",
		Version:  "v1.2.3",
		Filename: "stroppy-agent_linux_amd64",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetArtifact())
	require.Equal(t, "abc123deadbeef", resp.GetArtifact().GetSha256())
	require.Equal(t, "s3://bucket/stroppy-agent/v1.2.3/stroppy-agent_linux_amd64", resp.GetDownloadUrl())

	// Resolve miss.
	_, err = svc.Resolve(ctx, &agentpb.ResolveArtifactRequest{
		Name:     "nonexistent",
		Version:  "v0.0.0",
		Filename: "nope",
	})
	require.Error(t, err)
}
