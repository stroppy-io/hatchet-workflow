package expand

func init() { Register("ydb-managed", ExpandYDBManaged) }

// ExpandYDBManaged derives the managed-YDB database VMs from its config —
// which is NONE. Managed YDB (serverless or dedicated) is a fully YC-managed
// offering: the database nodes are provisioned and operated by Yandex Cloud,
// not as self-hosted machines in the deployment. There is therefore no
// database VM to expand, so it returns an empty slice. The deployment's only
// machine is the stroppy workload runner, which WithWorkload adds downstream.
func ExpandYDBManaged(db map[string]any) []VM {
	return nil
}
