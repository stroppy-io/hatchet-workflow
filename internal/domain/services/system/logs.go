package system

import (
	"context"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// AgentLogLine is the engine-facing shape for a single log row produced by an
// agent. Decoupled from agentpb.LogLine so this package doesn't import agent.
type AgentLogLine struct {
	DagRunID  string
	NodeRunID string
	CommandID string
	Timestamp time.Time
	Stream    string // STDOUT / STDERR / UNSPECIFIED
	Line      string
}

// LogBus is an in-memory pub/sub keyed by DagRunID. Subscribers receive every
// log line inserted for that run until they call release().
type LogBus struct {
	mu   sync.RWMutex
	subs map[string]map[int64]chan *AgentLogLine
	next int64
}

// NewLogBus builds a fresh bus.
func NewLogBus() *LogBus {
	return &LogBus{subs: map[string]map[int64]chan *AgentLogLine{}}
}

// Subscribe returns a channel + release callback. release MUST be called on
// disconnect; otherwise the bus leaks.
func (b *LogBus) Subscribe(dagRunID string) (<-chan *AgentLogLine, func()) {
	ch := make(chan *AgentLogLine, 256)

	b.mu.Lock()
	if _, ok := b.subs[dagRunID]; !ok {
		b.subs[dagRunID] = map[int64]chan *AgentLogLine{}
	}
	id := b.next
	b.next++
	b.subs[dagRunID][id] = ch
	b.mu.Unlock()

	release := func() {
		b.mu.Lock()
		if m, ok := b.subs[dagRunID]; ok {
			delete(m, id)
			if len(m) == 0 {
				delete(b.subs, dagRunID)
			}
		}
		b.mu.Unlock()
		close(ch)
	}
	return ch, release
}

// publish fanouts to all subscribers for a dag_run. Drops on full channel.
func (b *LogBus) publish(l *AgentLogLine) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs[l.DagRunID] {
		select {
		case ch <- l:
		default:
		}
	}
}

// IngestLogs persists a batch into node_run_logs and publishes each line
// to the in-memory bus for live subscribers.
func (s *Service) IngestLogs(ctx context.Context, lines []AgentLogLine) error {
	if len(lines) == 0 {
		return nil
	}
	for _, l := range lines {
		ts := l.Timestamp
		if ts.IsZero() {
			ts = time.Now()
		}
		row := &systempb.NodeRunLog{
			Id:         &systempb.NodeRunLogId{Value: ids.New()},
			DagRunId:   l.DagRunID,
			NodeRunId:  l.NodeRunID,
			CommandId:  l.CommandID,
			Ts:         timestamppb.New(ts),
			Stream:     l.Stream,
			Line:       l.Line,
			Timestamps: &commonpb.Timestamps{CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
		}
		scanner := row.IntoPlain()
		if _, err := s.logRepo.Execute(ctx,
			systempb.NodeRunLogs.Insert().From(scanner.AllSetters()...),
		); err != nil {
			return err
		}
		s.logBus.publish(&l)
	}
	return nil
}
