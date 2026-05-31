package recipe

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/expand"
)

// pdRecipe is the Picodata deploy engine. Instances form one cluster joining via
// instance-0's iproto peer; stroppy connects directly (the picodata driver does
// its own topology discovery, so it bypasses any HAProxy).
type pdRecipe struct{}

const (
	picodataPGPort    = 5432
	picodataPeerPort  = 3301
	picodataHTTPPort  = 8081
	picodataAdminPass = "T0psecret"
)

func (pdRecipe) BuildComponent(string, map[string]any, map[string]any, Refs) *topology.Component_Strategy {
	return nil
}
func (pdRecipe) Wire(map[string][]string) []*topology.Connection { return nil }

func (pdRecipe) Steps(self Node, cl Cluster, db, wl map[string]any, refs Refs) []Step {
	switch self.Role {
	case expand.RoleDatabase:
		return picodataNodeSteps(self, cl)
	case expand.RoleProxy:
		return nil // stroppy bypasses haproxy for picodata; skip
	case expand.RoleWorkload:
		host := "127.0.0.1"
		if n, ok := cl.First(expand.RoleDatabase); ok {
			host = n.IP
		}
		dsn := fmt.Sprintf("postgres://admin:%s@%s:%d?sslmode=disable", picodataAdminPass, host, picodataPGPort)
		return stroppyRunSteps(driverPicodata, "tpcc/tx", dsn, refs)
	default:
		return nil
	}
}

// picodataNodeSteps installs picodata, writes its config and starts the instance
// joined to the cluster bootstrap peer (instance-0).
func picodataNodeSteps(self Node, cl Cluster) []Step {
	nodes := cl.Role[expand.RoleDatabase]
	idx := nodeIndex(nodes, self.ID)
	rf := 1
	if len(nodes) > 1 {
		rf = 2
	}
	bootstrapPeer := fmt.Sprintf("%s:%d", nodes[0].IP, picodataPeerPort)

	install := `set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y curl ca-certificates gnupg lsb-release
curl -fsSL https://download.picodata.io/tarantool-picodata/picodata.gpg.key | gpg --no-default-keyring --keyring gnupg-ring:/etc/apt/trusted.gpg.d/picodata.gpg --import
chmod 644 /etc/apt/trusted.gpg.d/picodata.gpg
echo "deb https://download.picodata.io/tarantool-picodata/ubuntu/ $(lsb_release -cs) main" > /etc/apt/sources.list.d/picodata.list
apt-get update
apt-get install -y picodata`

	conf := fmt.Sprintf(`cluster:
  name: stroppy-cluster
  tier:
    default:
      replication_factor: %d
      can_vote: true
instance:
  tier: default
  iproto:
    listen: 0.0.0.0:%d
    advertise: %s:%d
  pg:
    listen: 0.0.0.0:%d
    advertise: %s:%d
    ssl: false
  http:
    listen: 0.0.0.0:%d
  memtx:
    memory: 1073741824
`, rf, picodataPeerPort, self.IP, picodataPeerPort, picodataPGPort, self.IP, picodataPGPort, picodataHTTPPort)

	start := fmt.Sprintf(`set -e
mkdir -p /var/lib/picodata
systemctl stop picodata 2>/dev/null || true; systemctl reset-failed picodata 2>/dev/null || true
systemd-run --unit=picodata \
  --setenv=PICODATA_ADMIN_PASSWORD=%s \
  -- picodata run --config /etc/picodata/picodata.yaml --peer %s --instance-name instance-%d --instance-dir /var/lib/picodata
for i in $(seq 1 30); do curl -sf http://localhost:%d/api/v1/health/ready 2>/dev/null && exit 0; sleep 2; done
echo "picodata not ready" >&2; cat /var/lib/picodata/*.log 2>/dev/null >&2 || true; exit 1`,
		picodataAdminPass, bootstrapPeer, idx, picodataHTTPPort)

	return []Step{
		cmdStep("install picodata", install),
		writeStep("write picodata.yaml", "/etc/picodata/picodata.yaml", 0o644, conf),
		cmdStep("start picodata", start),
	}
}
