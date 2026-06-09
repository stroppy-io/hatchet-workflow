package agent

import (
	"bytes"
	"strings"
	"testing"
)

func TestTruncateCapturedStreamKeepsSmallOutput(t *testing.T) {
	input := []byte("small output")

	got := truncateCapturedStream(input)
	if !bytes.Equal(got, input) {
		t.Fatalf("small output changed: %q", got)
	}
}

func TestTruncateCapturedStreamKeepsTail(t *testing.T) {
	input := append(bytes.Repeat([]byte("a"), maxCapturedStreamBytes+128), []byte("tail-marker")...)

	got := truncateCapturedStream(input)
	if !strings.HasPrefix(string(got), "... output truncated") {
		t.Fatalf("truncated output missing marker: %q", got[:64])
	}
	if !bytes.HasSuffix(got, []byte("tail-marker")) {
		t.Fatalf("truncated output does not keep tail")
	}
	if len(got) <= maxCapturedStreamBytes || len(got) > maxCapturedStreamBytes+128 {
		t.Fatalf("truncated length = %d, want bounded tail plus marker", len(got))
	}
}
