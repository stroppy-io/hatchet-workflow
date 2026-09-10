package graphene

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"connectrpc.com/connect"

	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

/*
OBSERVE helpers: the run-level reads the projection and the UI need —
the event stream (resumable by event id), the status watch, the ownership
tree, keep/stand commands and artifact download. All namespace-scoped
through ctx.
*/

// Events follows the run's event stream from afterID and hands every
// event to fn until the stream ends or fn returns an error.
func (c *Client) Events(ctx context.Context, runID string, afterID int64, follow bool, fn func(run.RawEvent) error) error {
	stream, err := c.Observe.Events(ctx, connect.NewRequest(&managementv1.EventsRequest{Ref: "run/" + runID, AfterEventId: afterID, Follow: follow}))
	if err != nil {
		return fmt.Errorf("graphene: events of %s: %w", runID, err)
	}
	defer stream.Close()
	for stream.Receive() {
		e := stream.Msg()
		raw := run.RawEvent{
			ID: e.GetEventId(), At: time.Unix(0, e.GetTimeUnixNano()).UTC(), Kind: e.GetKind(), Subject: e.GetSubject(),
			Agent: e.GetAgent(), Status: e.GetStatus(), Error: e.GetError(), Attempt: int(e.GetAttempt()),
			Input: e.GetInput(), Result: e.GetResult(), ActivityID: e.GetActivityId(),
		}
		if err := fn(raw); err != nil {
			return err
		}
	}
	if err := stream.Err(); err != nil {
		return fmt.Errorf("graphene: events of %s: %w", runID, err)
	}
	return nil
}

// WatchRun streams status transitions until terminal.
func (c *Client) WatchRun(ctx context.Context, runID string, fn func(status string) error) error {
	stream, err := c.Runs.WatchRun(ctx, connect.NewRequest(&managementv1.WatchRunRequest{RunId: runID}))
	if err != nil {
		return fmt.Errorf("graphene: watch %s: %w", runID, err)
	}
	defer stream.Close()
	for stream.Receive() {
		if err := fn(stream.Msg().GetStatus()); err != nil {
			return err
		}
	}
	if err := stream.Err(); err != nil {
		return fmt.Errorf("graphene: watch %s: %w", runID, err)
	}
	return nil
}

// Tree reads the ownership tree of a ref.
func (c *Client) Tree(ctx context.Context, owner string) (run.TreeNode, error) {
	resp, err := c.Resources.Tree(ctx, connect.NewRequest(&managementv1.TreeRequest{Owner: owner}))
	if err != nil {
		return run.TreeNode{}, fmt.Errorf("graphene: tree of %s: %w", owner, err)
	}
	roots := resp.Msg.GetRoots()
	if len(roots) == 0 {
		return run.TreeNode{Ref: owner, Children: []run.TreeNode{}}, nil
	}
	if len(roots) == 1 {
		return treeOf(roots[0]), nil
	}
	out := run.TreeNode{Ref: owner, Children: make([]run.TreeNode, 0, len(roots))}
	for _, r := range roots {
		out.Children = append(out.Children, treeOf(r))
	}
	return out, nil
}

func treeOf(n *managementv1.TreeNode) run.TreeNode {
	if n == nil {
		return run.TreeNode{Children: []run.TreeNode{}}
	}
	r := n.GetResource()
	out := run.TreeNode{Ref: r.GetRef(), Kind: r.GetKind(), Phase: r.GetPhase(), Labels: r.GetLabels(), Children: make([]run.TreeNode, 0, len(n.GetChildren()))}
	for _, ch := range n.GetChildren() {
		out.Children = append(out.Children, treeOf(ch))
	}
	return out
}

// DeleteRef tears a subtree down (blocking on the Graphene side).
func (c *Client) DeleteRef(ctx context.Context, ref string) error {
	_, err := c.Resources.Delete(ctx, connect.NewRequest(&managementv1.DeleteRequest{Ref: ref}))
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("graphene: delete %s: %w", ref, err)
	}
	return nil
}

// KeepExtend parks the run's stand for keep more (the pipeline moved it
// to the stand already; extend just moves the deadline).
func (c *Client) KeepExtend(ctx context.Context, runID string, keep time.Duration) error {
	payload, _ := json.Marshal(map[string]any{"ref": "run/" + runID, "keep": keep.Nanoseconds()}) //nolint:errcheck // map
	_, err := c.Resources.Invoke(ctx, connect.NewRequest(&managementv1.InvokeRequest{
		Ref: "stand/" + PipelineRun, Command: "extend", Payload: payload,
	}))
	if err != nil {
		return fmt.Errorf("graphene: extend keep of %s: %w", runID, err)
	}
	return nil
}

// KeepRelease tears the kept stand down now.
func (c *Client) KeepRelease(ctx context.Context, runID string) error {
	payload, _ := json.Marshal(map[string]any{"ref": "run/" + runID}) //nolint:errcheck // map
	_, err := c.Resources.Invoke(ctx, connect.NewRequest(&managementv1.InvokeRequest{
		Ref: "stand/" + PipelineRun, Command: "release", Payload: payload,
	}))
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("graphene: release keep of %s: %w", runID, err)
	}
	return nil
}

// Artifacts lists the artifact records owned by the run.
func (c *Client) Artifacts(ctx context.Context, runID string) ([]run.Artifact, error) {
	resp, err := c.Resources.List(ctx, connect.NewRequest(&managementv1.ListRequest{
		Selector: &managementv1.Selector{Kind: "artifact", Owner: "run/" + runID}, PageSize: 200,
	}))
	if err != nil {
		return nil, fmt.Errorf("graphene: artifacts of %s: %w", runID, err)
	}
	out := make([]run.Artifact, 0, len(resp.Msg.GetResources()))
	for _, r := range resp.Msg.GetResources() {
		a := run.Artifact{Ref: r.GetRef(), Name: r.GetRef()}
		var state struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
			Blob struct {
				Location    string `json:"location"`
				ContentType string `json:"contentType"`
				Size        int64  `json:"size"`
				Digest      string `json:"digest"`
			} `json:"blob"`
		}
		if json.Unmarshal(r.GetState(), &state) == nil {
			if state.Name != "" {
				a.Name = state.Name
			}
			a.Kind = state.Kind
			a.ContentType, a.SizeBytes, a.Digest = state.Blob.ContentType, state.Blob.Size, state.Blob.Digest
		}
		out = append(out, a)
	}
	return out, nil
}

// Download streams an artifact's bytes.
func (c *Client) Download(ctx context.Context, ref string) (io.ReadCloser, error) {
	stream, err := c.Resources.Download(ctx, connect.NewRequest(&managementv1.DownloadRequest{Ref: ref}))
	if err != nil {
		return nil, fmt.Errorf("graphene: download %s: %w", ref, err)
	}
	pr, pw := io.Pipe()
	go func() {
		defer stream.Close()
		for stream.Receive() {
			if _, err := pw.Write(stream.Msg().GetData()); err != nil {
				pw.CloseWithError(err)
				return
			}
		}
		pw.CloseWithError(stream.Err())
	}()
	return pr, nil
}
