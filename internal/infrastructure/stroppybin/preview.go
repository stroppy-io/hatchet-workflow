package stroppybin

import (
	"context"
	"encoding/json"
)

type PreviewInput = ProbeInput

func RenderConfig(_ context.Context, in PreviewInput) ([]byte, error) {
	cfg := buildRunConfig(in)
	return json.MarshalIndent(cfg, "", "  ")
}
