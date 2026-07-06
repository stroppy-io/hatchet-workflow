package agent

import "net"

// NomadServerRPCPort is the port a Nomad client connects to a Nomad server
// on. It is Nomad's default RPC port (see NomadServerHCL's server block,
// which advertises the same port implicitly); AssignNomadRoles joins it with
// the gateway's private IP to build each client's ServerAddr.
const NomadServerRPCPort = "4647"

// NomadAssignment is a per-machine Bootstrap-field patch: the caller (the
// RunRecipeWorkflow, phase 1D) merges Role/ServerAddr into that machine's
// Bootstrap.NomadRole/NomadServerAddr before rendering cloud-init/docker.
type NomadAssignment struct {
	Role NomadRole
	// ServerAddr is "" for the server (NomadRoleServer is its own
	// coordinator) and "<gatewayPrivateIP>:4647" for every client.
	ServerAddr string
}

// AssignNomadRoles decides each provisioned machine's Nomad role: gatewayID
// becomes the Nomad server, every other id in machineIDs becomes a client
// pointing at gatewayPrivateIP:4647. It is deterministic — the same inputs
// always produce the same map.
//
// If gatewayID is not present in machineIDs, no server assignment is
// produced; every listed machine is still assigned as a client pointing at
// gatewayPrivateIP. This covers a gateway that is a separate control node
// (not itself one of the provisioned worker machines) — the caller is
// expected to have provisioned Nomad's server role there independently, or
// this simply reflects "no machine in this list runs the server." Callers
// that require the gateway to be a worker machine must validate that
// themselves; AssignNomadRoles does not error on the mismatch.
//
// If gatewayPrivateIP is empty while clients are present, clients receive a
// ServerAddr with an empty host (e.g. ":4647") — it is the caller's
// responsibility to supply a non-empty gatewayPrivateIP whenever any client
// assignment is expected to be usable.
func AssignNomadRoles(machineIDs []string, gatewayID, gatewayPrivateIP string) map[string]NomadAssignment {
	clientAddr := net.JoinHostPort(gatewayPrivateIP, NomadServerRPCPort)

	assignments := make(map[string]NomadAssignment, len(machineIDs))
	for _, id := range machineIDs {
		if id == gatewayID {
			assignments[id] = NomadAssignment{Role: NomadRoleServer, ServerAddr: ""}
			continue
		}
		assignments[id] = NomadAssignment{Role: NomadRoleClient, ServerAddr: clientAddr}
	}
	return assignments
}
