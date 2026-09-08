package spec

import (
	"encoding/json"
	"fmt"
)

// Segment is the part of a workload.segment@1 value the pipeline needs to
// run stroppy. Fields the pipeline does not interpret stay in Raw.
type Segment struct {
	Name      string          `json:"name"`
	Script    string          `json:"script"`
	Execution Execution       `json:"execution"`
	Params    SegmentParams   `json:"params,omitzero"`
	Files     []SegmentFile   `json:"files,omitempty"`
	Warmup    Duration        `json:"warmup,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

// Execution bounds one segment.
type Execution struct {
	VUs          int      `json:"vus"`
	Limit        Limit    `json:"limit"`
	Quiet        bool     `json:"quiet,omitempty"`
	NoThresholds bool     `json:"no_thresholds,omitempty"`
	ExtraArgs    []string `json:"extra_args,omitempty"`
}

// Limit is the discriminated duration|iterations bound of a segment.
type Limit struct {
	Kind       string   `json:"kind"`
	Duration   Duration `json:"duration,omitempty"`
	Iterations int64    `json:"count,omitempty"`
}

// SegmentParams are the stroppy-level knobs of a segment.
type SegmentParams struct {
	PoolSize     int               `json:"pool_size,omitempty"`
	ScaleFactor  int               `json:"scale_factor,omitempty"`
	Steps        []string          `json:"steps,omitempty"`
	NoSteps      []string          `json:"no_steps,omitempty"`
	InsertMethod string            `json:"insert_method,omitempty"`
	BulkSize     int               `json:"bulk_size,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
}

// SegmentFile is a file shipped next to the script.
type SegmentFile struct {
	Name    string `json:"name"`
	Kind    string `json:"kind,omitempty"`
	Content string `json:"content,omitempty"`
	Ref     string `json:"ref,omitempty"`
}

// DecodeSegments parses the workload segments of a run; unknown fields are
// kept in Raw, so the pipeline tolerates schema growth.
func DecodeSegments(raw []json.RawMessage) ([]Segment, error) {
	out := make([]Segment, 0, len(raw))
	for i, r := range raw {
		var s Segment
		if err := json.Unmarshal(r, &s); err != nil {
			return nil, fmt.Errorf("segment %d: %w", i, err)
		}
		if s.Name == "" {
			return nil, fmt.Errorf("segment %d: empty name", i)
		}
		s.Raw = r
		out = append(out, s)
	}
	return out, nil
}
