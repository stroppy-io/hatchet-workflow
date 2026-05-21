package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
)

// wsMessage is the envelope sent to WebSocket clients.
type wsMessage struct {
	Type     string `json:"type"` // "log", "report", "agent_log"
	RunID    string `json:"run_id,omitempty"`
	NodeID   string `json:"node_id,omitempty"`
	TenantID string `json:"-"` // not serialized — used for filtering only
	Payload  any    `json:"payload"`
}

type wsClient struct {
	conn        *websocket.Conn
	writeMu     sync.Mutex // gorilla/websocket is not safe for concurrent writes
	filterRunID string
	tenantID    string
}

func (c *wsClient) send(data []byte) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	c.conn.WriteMessage(websocket.TextMessage, data)
}

// wsHub manages WebSocket clients and broadcasts messages to them.
type wsHub struct {
	mu      sync.RWMutex
	clients []*wsClient

	// victoriaLogs persists server/DAG logs alongside agent logs.
	// nil when VictoriaLogs is not configured.
	victoriaLogs *victoria.LogsClient
	logger       *zap.Logger

	// accountIDResolver resolves runID → tenant accountID for per-tenant log ingestion.
	// Set by Server after construction.
	accountIDResolver func(runID string) int32
	// tenantIDResolver resolves runID → tenantID for WS broadcast filtering.
	tenantIDResolver func(runID string) string
}

func newWSHub() *wsHub {
	return &wsHub{}
}

func (h *wsHub) addClient(conn *websocket.Conn, filterRunID, tenantID string) {
	client := &wsClient{conn: conn, filterRunID: filterRunID, tenantID: tenantID}

	h.mu.Lock()
	h.clients = append(h.clients, client)
	h.mu.Unlock()
	wsConnections.Inc()

	// Read pump: drain client messages and detect disconnect.
	go func() {
		defer h.removeClient(client)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func (h *wsHub) removeClient(c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, cl := range h.clients {
		if cl == c {
			h.clients = append(h.clients[:i], h.clients[i+1:]...)
			c.conn.Close()
			wsConnections.Dec()
			return
		}
	}
}

func (h *wsHub) broadcast(msg wsMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	snapshot := make([]*wsClient, len(h.clients))
	copy(snapshot, h.clients)
	h.mu.RUnlock()

	for _, c := range snapshot {
		// Tenant isolation: only deliver to clients in the same tenant.
		if c.tenantID != "" && msg.TenantID != "" && c.tenantID != msg.TenantID {
			continue
		}
		if c.filterRunID != "" && msg.RunID != "" && c.filterRunID != msg.RunID {
			continue
		}
		c.send(data)
	}
}

// encodeFields serializes zap fields into a flat map for JSON transport.
func encodeFields(fields []zapcore.Field) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}
	return enc.Fields
}

// formatLogLine builds a human-readable log line including all zap fields.
func formatLogLine(level, nodeID, message string, fields map[string]any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] [%s] %s", level, nodeID, message)
	for k, v := range fields {
		if k == "node_id" {
			continue // already in the prefix
		}
		fmt.Fprintf(&b, "  %s=%v", k, v)
	}
	return b.String()
}

// WriteLog implements dag.LogSink.
func (h *wsHub) WriteLog(_ context.Context, executionID string, nodeID string, entry zapcore.Entry, fields []zapcore.Field) {
	encoded := encodeFields(fields)

	payload := map[string]any{
		"level":   entry.Level.String(),
		"message": entry.Message,
		"time":    entry.Time,
	}
	// Merge fields into payload so the UI gets full details.
	for k, v := range encoded {
		payload[k] = v
	}

	var tenantID string
	if h.tenantIDResolver != nil {
		tenantID = h.tenantIDResolver(executionID)
	}
	h.broadcast(wsMessage{
		Type:     "log",
		RunID:    executionID,
		NodeID:   nodeID,
		TenantID: tenantID,
		Payload:  payload,
	})

	// Persist server log to VictoriaLogs so it appears in historical queries.
	if h.victoriaLogs != nil {
		line := formatLogLine(entry.Level.CapitalString(), nodeID, entry.Message, encoded)
		var accountID int32
		if h.accountIDResolver != nil {
			accountID = h.accountIDResolver(executionID)
		}
		go func() {
			if err := h.victoriaLogs.IngestWithAccount(accountID, "server", "", nodeID, executionID, "server", line); err != nil {
				if h.logger != nil {
					h.logger.Debug("vlogs server log ingest failed", zap.Error(err))
				}
			}
		}()
	}
}
