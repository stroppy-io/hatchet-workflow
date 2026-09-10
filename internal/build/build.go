// Package build carries the identity of this binary: what was built, from
// which commit, when. Filled by ldflags (see the Makefile); the defaults are
// what a plain `go build` gets.
//
// The version doubles as the pipeline revision: one commit = one server =
// one set of pipeline images (STROPPY.MD §7).
package build

import (
	"os"

	"github.com/google/uuid"
)

// ServiceName is the OTel service.name and the logger root name.
const ServiceName = "stroppy-cloud"

var (
	// Version is the git describe of the build ("dev" when unknown).
	Version = "dev"
	// Commit is the short git commit.
	Commit = "unknown"
	// BuildTime is the RFC 3339 build timestamp.
	BuildTime = "unknown"
	// InstanceID identifies this process: the hostname (pod name in k8s)
	// or a random id when the hostname is unavailable.
	InstanceID = instanceID()
)

func instanceID() string {
	if host, err := os.Hostname(); err == nil && host != "" {
		return host
	}
	return uuid.NewString()
}
