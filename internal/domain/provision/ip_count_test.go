package provision

import (
	"testing"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
)

func TestRequiredIPCount(t *testing.T) {
	tests := []struct {
		name string
		tmpl *database.Database_Template
		want int
	}{
		{
			name: "nil template",
			tmpl: nil,
			want: 0,
		},
		{
			name: "postgres instance",
			tmpl: &database.Database_Template{
				Template: &database.Database_Template_PostgresInstance{
					PostgresInstance: &database.Postgres_Instance_Template{},
				},
			},
			want: 1,
		},
		{
			name: "cluster with 2 replicas",
			tmpl: &database.Database_Template{
				Template: &database.Database_Template_PostgresCluster{
					PostgresCluster: &database.Postgres_Cluster_Template{
						Topology: &database.Postgres_Cluster_Template_Topology{
							ReplicasCount: 2,
						},
					},
				},
			},
			want: 3,
		},
		{
			name: "cluster with 1 replica",
			tmpl: &database.Database_Template{
				Template: &database.Database_Template_PostgresCluster{
					PostgresCluster: &database.Postgres_Cluster_Template{
						Topology: &database.Postgres_Cluster_Template_Topology{
							ReplicasCount: 1,
						},
					},
				},
			},
			want: 2,
		},
		{
			name: "cluster with dedicated etcd",
			tmpl: &database.Database_Template{
				Template: &database.Database_Template_PostgresCluster{
					PostgresCluster: &database.Postgres_Cluster_Template{
						Topology: &database.Postgres_Cluster_Template_Topology{
							ReplicasCount: 2,
						},
						Addons: &database.Postgres_Addons{
							Dcs: &database.Postgres_Addons_Dcs{
								Etcd: &database.Postgres_Addons_Dcs_Etcd{
									Size: 3,
									Placement: &database.Postgres_Placement{
										Mode: &database.Postgres_Placement_Dedicated_{
											Dedicated: &database.Postgres_Placement_Dedicated{
												InstancesCount: 3,
												Hardware:       &deployment.Hardware{Cores: 2, Memory: 4, Disk: 10},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: 6, // 1 master + 2 replicas + 3 dedicated etcd
		},
		{
			name: "cluster with colocated etcd (no extra VMs)",
			tmpl: &database.Database_Template{
				Template: &database.Database_Template_PostgresCluster{
					PostgresCluster: &database.Postgres_Cluster_Template{
						Topology: &database.Postgres_Cluster_Template_Topology{
							ReplicasCount: 2,
						},
						Addons: &database.Postgres_Addons{
							Dcs: &database.Postgres_Addons_Dcs{
								Etcd: &database.Postgres_Addons_Dcs_Etcd{
									Size: 3,
									Placement: &database.Postgres_Placement{
										Mode: &database.Postgres_Placement_Colocate_{
											Colocate: &database.Postgres_Placement_Colocate{
												Scope: database.Postgres_Placement_SCOPE_ALL_NODES,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: 3, // 1 master + 2 replicas, etcd colocated
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RequiredIPCount(tt.tmpl); got != tt.want {
				t.Errorf("RequiredIPCount() = %d, want %d", got, tt.want)
			}
		})
	}
}
