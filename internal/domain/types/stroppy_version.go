package types

import (
	"os"
	"strings"
	"sync"

	hcversion "github.com/hashicorp/go-version"
)

const defaultMinStroppyVersion = "5.1.1"

var (
	minStroppyVersionOnce sync.Once
	minStroppyVersionStr  string
	minStroppyVersionVal  *hcversion.Version
)

func loadMinStroppyVersion() {
	s := os.Getenv("STROPPY_MIN_VERSION")
	if s == "" {
		s = defaultMinStroppyVersion
	}
	v, err := hcversion.NewVersion(strings.TrimPrefix(s, "v"))
	if err != nil {
		v = hcversion.Must(hcversion.NewVersion(defaultMinStroppyVersion))
		s = defaultMinStroppyVersion
	}
	minStroppyVersionStr = s
	minStroppyVersionVal = v
}

// MinStroppyVersion returns the minimum stroppy semver permitted for runs,
// driven by the STROPPY_MIN_VERSION env (default 5.1.1). Below this, runs
// fall back to legacy embedded `tx.ts` that ignores driver_type — bug seen
// when v4.1.0 connected to YDB grpc with pgx-pool.
func MinStroppyVersion() *hcversion.Version {
	minStroppyVersionOnce.Do(loadMinStroppyVersion)
	return minStroppyVersionVal
}

func MinStroppyVersionString() string {
	minStroppyVersionOnce.Do(loadMinStroppyVersion)
	return minStroppyVersionStr
}

func ParseStroppyVersion(s string) (*hcversion.Version, error) {
	return hcversion.NewVersion(strings.TrimPrefix(s, "v"))
}
