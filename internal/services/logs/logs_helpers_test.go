package logs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLogsQL(t *testing.T) {
	t.Run("all empty -> wildcard", func(t *testing.T) {
		require.Equal(t, "*", logsQL("", "", "", ""))
	})

	t.Run("dag only", func(t *testing.T) {
		require.Equal(t, `dag_id:="dag-1"`, logsQL("dag-1", "", "", ""))
	})

	t.Run("node only", func(t *testing.T) {
		require.Equal(t, `node_execution_id:="node-1"`, logsQL("", "node-1", "", ""))
	})

	t.Run("component only", func(t *testing.T) {
		require.Equal(t, `component_id:="comp-1"`, logsQL("", "", "comp-1", ""))
	})

	t.Run("extra only", func(t *testing.T) {
		require.Equal(t, `_time:>x`, logsQL("", "", "", "_time:>x"))
	})

	t.Run("all set joined by space in order", func(t *testing.T) {
		got := logsQL("dag-1", "node-1", "comp-1", "extra:1")
		require.Equal(t, `dag_id:="dag-1" node_execution_id:="node-1" component_id:="comp-1" extra:1`, got)
	})

	t.Run("dag and component skips empty node", func(t *testing.T) {
		got := logsQL("dag-1", "", "comp-1", "")
		require.Equal(t, `dag_id:="dag-1" component_id:="comp-1"`, got)
	})
}

func TestSinceFilter(t *testing.T) {
	t.Run("zero time -> empty", func(t *testing.T) {
		require.Equal(t, "", sinceFilter(time.Time{}))
	})

	t.Run("set time -> _time filter in UTC RFC3339Nano", func(t *testing.T) {
		// 2023-01-02T03:04:05Z in a non-UTC zone must serialize as UTC.
		loc := time.FixedZone("UTC+3", 3*60*60)
		ts := time.Date(2023, 1, 2, 6, 4, 5, 0, loc) // == 03:04:05 UTC
		got := sinceFilter(ts)
		require.Equal(t, `_time:>"2023-01-02T03:04:05Z"`, got)
	})
}

func TestStr(t *testing.T) {
	m := map[string]any{
		"present": "value",
		"number":  42,
		"nilval":  nil,
	}

	t.Run("present string", func(t *testing.T) {
		require.Equal(t, "value", str(m, "present"))
	})

	t.Run("missing key -> empty", func(t *testing.T) {
		require.Equal(t, "", str(m, "missing"))
	})

	t.Run("non-string value -> empty", func(t *testing.T) {
		require.Equal(t, "", str(m, "number"))
	})

	t.Run("nil value -> empty", func(t *testing.T) {
		require.Equal(t, "", str(m, "nilval"))
	})
}

func TestRecordToLine(t *testing.T) {
	t.Run("maps all string fields", func(t *testing.T) {
		r := map[string]any{
			"dag_id":            "dag-1",
			"node_execution_id": "node-1",
			"component_id":      "comp-1",
			"machine_id":        "machine-1",
			"unit":              "unit-1",
			"_msg":              "hello world",
		}
		line := recordToLine(r)
		require.Equal(t, "dag-1", line.GetDagId())
		require.Equal(t, "node-1", line.GetNodeExecutionId())
		require.Equal(t, "comp-1", line.GetComponentId())
		require.Equal(t, "machine-1", line.GetMachineId())
		require.Equal(t, "unit-1", line.GetUnit())
		require.Equal(t, "hello world", line.GetLine())
	})

	t.Run("valid _time sets ObservedAt and Cursor", func(t *testing.T) {
		r := map[string]any{
			"_msg":  "msg",
			"_time": "2023-01-02T03:04:05Z",
		}
		line := recordToLine(r)
		require.NotNil(t, line.GetObservedAt())
		want := time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC)
		require.True(t, line.GetObservedAt().AsTime().Equal(want))
		require.NotNil(t, line.GetCursor())
		require.NotNil(t, line.GetCursor().GetObservedAt())
		require.True(t, line.GetCursor().GetObservedAt().AsTime().Equal(want))
	})

	t.Run("missing _time leaves ObservedAt and Cursor nil", func(t *testing.T) {
		line := recordToLine(map[string]any{"_msg": "msg"})
		require.Nil(t, line.GetObservedAt())
		require.Nil(t, line.GetCursor())
	})

	t.Run("invalid _time leaves ObservedAt and Cursor nil", func(t *testing.T) {
		line := recordToLine(map[string]any{"_msg": "msg", "_time": "not-a-time"})
		require.Nil(t, line.GetObservedAt())
		require.Nil(t, line.GetCursor())
	})

	t.Run("empty record yields zero-value line", func(t *testing.T) {
		line := recordToLine(map[string]any{})
		require.Empty(t, line.GetDagId())
		require.Empty(t, line.GetLine())
		require.Nil(t, line.GetObservedAt())
		require.Nil(t, line.GetCursor())
	})
}
