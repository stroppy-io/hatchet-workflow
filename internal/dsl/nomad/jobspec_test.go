package nomad

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"

	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

func postgresServiceSpec() *dslpb.ServiceSpec {
	return &dslpb.ServiceSpec{
		Name:    "postgres",
		OnGroup: "db",
		Image:   "postgres:17",
		Network: "host",
		Volumes: []string{"/data/pg:/var/lib/postgresql/data"},
		Env: map[string]string{
			"POSTGRES_PASSWORD": "already-evaluated-secret",
		},
		Health: &dslpb.HealthCheck{
			Http:    ":8008/health",
			Timeout: "120s",
		},
	}
}

func threeNodes() []NodeRef {
	return []NodeRef{
		{NodeID: "node-1", PrivateIP: "10.0.0.1"},
		{NodeID: "node-2", PrivateIP: "10.0.0.2"},
		{NodeID: "node-3", PrivateIP: "10.0.0.3"},
	}
}

func TestBuildJobProducesOneGroupPerNode(t *testing.T) {
	nodes := threeNodes()
	job, err := BuildJob(postgresServiceSpec(), nodes)
	if err != nil {
		t.Fatalf("BuildJob: %v", err)
	}

	if got, want := deref(job.ID), "postgres"; got != want {
		t.Fatalf("job id = %q, want %q", got, want)
	}
	if got, want := deref(job.Type), api.JobTypeService; got != want {
		t.Fatalf("job type = %q, want %q", got, want)
	}
	if got, want := len(job.TaskGroups), 3; got != want {
		t.Fatalf("task groups = %d, want %d", got, want)
	}

	// Deterministic order: groups must follow the nodes slice order, not any
	// other ordering (e.g. sorted by node id would coincidentally match here,
	// so also check group names explicitly follow the input order).
	for i, node := range nodes {
		group := job.TaskGroups[i]
		wantGroupName := "postgres-" + node.NodeID
		if got := deref(group.Name); got != wantGroupName {
			t.Fatalf("group[%d] name = %q, want %q", i, got, wantGroupName)
		}

		if got, want := len(group.Constraints), 1; got != want {
			t.Fatalf("group[%d] constraints = %d, want %d", i, got, want)
		}
		constraint := group.Constraints[0]
		if got, want := constraint.LTarget, "${meta.stroppy_node_id}"; got != want {
			t.Fatalf("group[%d] constraint LTarget = %q, want %q", i, got, want)
		}
		if got, want := constraint.RTarget, node.NodeID; got != want {
			t.Fatalf("group[%d] constraint RTarget = %q, want %q", i, got, want)
		}
		if got, want := constraint.Operand, "="; got != want {
			t.Fatalf("group[%d] constraint Operand = %q, want %q", i, got, want)
		}

		if got, want := derefInt(group.Count), 1; got != want {
			t.Fatalf("group[%d] count = %d, want %d", i, got, want)
		}

		if got, want := len(group.Tasks), 1; got != want {
			t.Fatalf("group[%d] tasks = %d, want %d", i, got, want)
		}
		task := group.Tasks[0]
		if got, want := task.Driver, "docker"; got != want {
			t.Fatalf("group[%d] task driver = %q, want %q", i, got, want)
		}
		if got, want := task.Config["image"], "postgres:17"; got != want {
			t.Fatalf("group[%d] task config image = %v, want %q", i, got, want)
		}
		if got, want := task.Config["network_mode"], "host"; got != want {
			t.Fatalf("group[%d] task config network_mode = %v, want %q", i, got, want)
		}
		gotVolumes, ok := task.Config["volumes"].([]string)
		if !ok {
			t.Fatalf("group[%d] task config volumes not a []string: %#v", i, task.Config["volumes"])
		}
		if got, want := gotVolumes, []string{"/data/pg:/var/lib/postgresql/data"}; len(got) != len(want) || got[0] != want[0] {
			t.Fatalf("group[%d] task config volumes = %v, want %v", i, got, want)
		}
		if got, want := task.Env["POSTGRES_PASSWORD"], "already-evaluated-secret"; got != want {
			t.Fatalf("group[%d] task env = %v, want %q", i, got, want)
		}

		if got, want := len(task.Services), 1; got != want {
			t.Fatalf("group[%d] task services = %d, want %d", i, got, want)
		}
		service := task.Services[0]
		if got, want := len(service.Checks), 1; got != want {
			t.Fatalf("group[%d] service checks = %d, want %d", i, got, want)
		}
		check := service.Checks[0]
		if got, want := check.Type, "http"; got != want {
			t.Fatalf("group[%d] check type = %q, want %q", i, got, want)
		}
		if got, want := check.Path, "/health"; got != want {
			t.Fatalf("group[%d] check path = %q, want %q", i, got, want)
		}
		if got, want := check.PortLabel, "8008"; got != want {
			t.Fatalf("group[%d] check port label = %q, want %q", i, got, want)
		}
		if got, want := check.Timeout, 120*time.Second; got != want {
			t.Fatalf("group[%d] check timeout = %v, want %v", i, got, want)
		}
	}
}

func TestBuildJobNoHealthOmitsServiceBlock(t *testing.T) {
	svc := postgresServiceSpec()
	svc.Health = nil

	job, err := BuildJob(svc, threeNodes())
	if err != nil {
		t.Fatalf("BuildJob: %v", err)
	}
	for i, group := range job.TaskGroups {
		task := group.Tasks[0]
		if len(task.Services) != 0 {
			t.Fatalf("group[%d] task services = %v, want none", i, task.Services)
		}
	}
}

func TestBuildJobNoNodesErrors(t *testing.T) {
	_, err := BuildJob(postgresServiceSpec(), nil)
	if err == nil {
		t.Fatalf("BuildJob with no nodes: want error, got nil")
	}
}

func TestBuildJobNilServiceSpecErrors(t *testing.T) {
	_, err := BuildJob(nil, threeNodes())
	if err == nil {
		t.Fatalf("BuildJob with nil ServiceSpec: want error, got nil")
	}
}

func TestBuildJobMissingImageErrors(t *testing.T) {
	svc := postgresServiceSpec()
	svc.Image = ""
	_, err := BuildJob(svc, threeNodes())
	if err == nil {
		t.Fatalf("BuildJob with empty image: want error, got nil")
	}
}

func TestBuildJobMissingNodeIDErrors(t *testing.T) {
	_, err := BuildJob(postgresServiceSpec(), []NodeRef{{PrivateIP: "10.0.0.1"}})
	if err == nil {
		t.Fatalf("BuildJob with empty NodeID: want error, got nil")
	}
}

func TestBuildJobDeterministic(t *testing.T) {
	svc := postgresServiceSpec()
	nodes := threeNodes()

	job1, err := BuildJob(svc, nodes)
	if err != nil {
		t.Fatalf("BuildJob (1): %v", err)
	}
	job2, err := BuildJob(svc, nodes)
	if err != nil {
		t.Fatalf("BuildJob (2): %v", err)
	}

	if len(job1.TaskGroups) != len(job2.TaskGroups) {
		t.Fatalf("group counts differ: %d vs %d", len(job1.TaskGroups), len(job2.TaskGroups))
	}
	for i := range job1.TaskGroups {
		if deref(job1.TaskGroups[i].Name) != deref(job2.TaskGroups[i].Name) {
			t.Fatalf("group[%d] name differs across calls: %q vs %q",
				i, deref(job1.TaskGroups[i].Name), deref(job2.TaskGroups[i].Name))
		}
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
