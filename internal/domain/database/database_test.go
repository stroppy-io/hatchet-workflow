package database

import (
	"context"
	"testing"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
)

func settings(version database.Postgres_Settings_Version, storageEngine database.Postgres_Settings_StorageEngine) *database.Postgres_Settings {
	return &database.Postgres_Settings{
		Version:       version,
		StorageEngine: storageEngine,
	}
}

func hw(cores, mem, disk uint32) *deployment.Hardware {
	return &deployment.Hardware{Cores: cores, Memory: mem, Disk: disk}
}

func TestValidateDatabaseTemplate_Nil(t *testing.T) {
	ctx := context.Background()
	err := ValidateDatabaseTemplate(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil template")
	}
	if err.Error() != "database template is nil" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateDatabaseTemplate_NilContent(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for nil template content")
	}
}

func TestValidatePostgresInstance_Valid(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresInstance{
			PostgresInstance: &database.Postgres_Instance_Template{
				Settings: settings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
				Hardware: hw(4, 8, 100),
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePostgresInstance_NilTemplate(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresInstance{},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for nil postgres instance template")
	}
}

func TestValidatePostgresInstance_NilSettings(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresInstance{
			PostgresInstance: &database.Postgres_Instance_Template{
				Hardware: hw(4, 8, 100),
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for nil settings")
	}
}

func TestValidatePostgresInstance_PatroniEnabled(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresInstance{
			PostgresInstance: &database.Postgres_Instance_Template{
				Settings: &database.Postgres_Settings{
					Version:       database.Postgres_Settings_VERSION_17,
					StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_HEAP,
					Patroni:       &database.Postgres_Settings_Patroni{Enabled: true},
				},
				Hardware: hw(4, 8, 100),
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for patroni on instance")
	}
}

func TestValidatePostgresCluster_Valid(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster_Template{
				Topology: &database.Postgres_Cluster_Template_Topology{
					Settings:        settings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
					MasterHardware:  hw(4, 8, 100),
					ReplicaHardware: hw(2, 4, 50),
					ReplicasCount:   2,
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePostgresCluster_NilTemplate(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for nil cluster template")
	}
}

func TestValidatePostgresCluster_NilTopology(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster_Template{},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for nil topology")
	}
}

func TestValidatePostgresCluster_NilSettings(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster_Template{
				Topology: &database.Postgres_Cluster_Template_Topology{
					MasterHardware:  hw(4, 8, 100),
					ReplicaHardware: hw(2, 4, 50),
					ReplicasCount:   2,
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for nil settings")
	}
}

func TestValidatePostgresCluster_ReplicaOverride_Valid(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster_Template{
				Topology: &database.Postgres_Cluster_Template_Topology{
					Settings:        settings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
					MasterHardware:  hw(4, 8, 100),
					ReplicaHardware: hw(2, 4, 50),
					ReplicasCount:   2,
				},
				ReplicaOverrides: []*database.Postgres_Cluster_Template_ReplicaOverride{
					{ReplicaIndex: 0, Hardware: hw(8, 16, 200)},
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePostgresCluster_ReplicaOverride_OutOfBounds(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster_Template{
				Topology: &database.Postgres_Cluster_Template_Topology{
					Settings:        settings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
					MasterHardware:  hw(4, 8, 100),
					ReplicaHardware: hw(2, 4, 50),
					ReplicasCount:   2,
				},
				ReplicaOverrides: []*database.Postgres_Cluster_Template_ReplicaOverride{
					{ReplicaIndex: 5},
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for replica override out of bounds")
	}
}

func TestValidatePostgresCluster_ReplicaOverride_Duplicate(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster_Template{
				Topology: &database.Postgres_Cluster_Template_Topology{
					Settings:        settings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
					MasterHardware:  hw(4, 8, 100),
					ReplicaHardware: hw(2, 4, 50),
					ReplicasCount:   2,
				},
				ReplicaOverrides: []*database.Postgres_Cluster_Template_ReplicaOverride{
					{ReplicaIndex: 0},
					{ReplicaIndex: 0},
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for duplicate replica override")
	}
}

func TestValidatePostgresCluster_ReplicaOverride_InvalidSettings(t *testing.T) {
	ctx := context.Background()
	invalidSettings := &database.Postgres_Settings{
		Version:       database.Postgres_Settings_VERSION_18,
		StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_ORIOLEDB,
	}
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster_Template{
				Topology: &database.Postgres_Cluster_Template_Topology{
					Settings:        settings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
					MasterHardware:  hw(4, 8, 100),
					ReplicaHardware: hw(2, 4, 50),
					ReplicasCount:   2,
				},
				ReplicaOverrides: []*database.Postgres_Cluster_Template_ReplicaOverride{
					{ReplicaIndex: 0, Settings: invalidSettings},
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for invalid override settings")
	}
}

func TestValidatePostgresCluster_PatroniWithoutEtcd(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster_Template{
				Topology: &database.Postgres_Cluster_Template_Topology{
					Settings: &database.Postgres_Settings{
						Version:       database.Postgres_Settings_VERSION_17,
						StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_HEAP,
						Patroni:       &database.Postgres_Settings_Patroni{Enabled: true},
					},
					MasterHardware:  hw(4, 8, 100),
					ReplicaHardware: hw(2, 4, 50),
					ReplicasCount:   2,
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for patroni without etcd")
	}
}

func TestValidatePostgresCluster_PatroniWithEtcd(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster_Template{
				Topology: &database.Postgres_Cluster_Template_Topology{
					Settings: &database.Postgres_Settings{
						Version:       database.Postgres_Settings_VERSION_17,
						StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_HEAP,
						Patroni:       &database.Postgres_Settings_Patroni{Enabled: true},
					},
					MasterHardware:  hw(4, 8, 100),
					ReplicaHardware: hw(2, 4, 50),
					ReplicasCount:   2,
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
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePostgresCluster_PatroniSynchronousMode(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name         string
		syncNodes    uint32
		replicaCount uint32
		etcdSize     uint32
		expectError  bool
	}{
		{"sync_nodes_equals_replicas", 2, 2, 3, false},
		{"sync_nodes_less_than_replicas", 1, 2, 3, false},
		{"sync_nodes_more_than_replicas", 3, 2, 3, true},
		{"sync_nodes_zero_uses_default", 0, 2, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &database.Database_Template{
				Template: &database.Database_Template_PostgresCluster{
					PostgresCluster: &database.Postgres_Cluster_Template{
						Topology: &database.Postgres_Cluster_Template_Topology{
							Settings: &database.Postgres_Settings{
								Version:       database.Postgres_Settings_VERSION_17,
								StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_HEAP,
								Patroni: &database.Postgres_Settings_Patroni{
									Enabled:              true,
									SynchronousMode:      true,
									SynchronousNodeCount: tt.syncNodes,
								},
							},
							MasterHardware:  hw(4, 8, 100),
							ReplicaHardware: hw(2, 4, 50),
							ReplicasCount:   tt.replicaCount,
						},
						Addons: &database.Postgres_Addons{
							Dcs: &database.Postgres_Addons_Dcs{
								Etcd: &database.Postgres_Addons_Dcs_Etcd{
									Size: tt.etcdSize,
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
			}
			err := ValidateDatabaseTemplate(ctx, tmpl)
			if tt.expectError && err == nil {
				t.Fatal("expected error for patroni synchronous mode")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePostgresCluster_EtcdPlacement_Colocate(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name        string
		scope       database.Postgres_Placement_Scope
		replicaIdx  *uint32
		size        uint32
		expectError bool
	}{
		{"master", database.Postgres_Placement_SCOPE_MASTER, nil, 1, false},
		{"replicas", database.Postgres_Placement_SCOPE_REPLICAS, nil, 1, true},
		{"all_nodes", database.Postgres_Placement_SCOPE_ALL_NODES, nil, 3, false},
		{"replica_valid_idx", database.Postgres_Placement_SCOPE_REPLICA, uint32Ptr(0), 1, false},
		{"replica_nil_idx", database.Postgres_Placement_SCOPE_REPLICA, nil, 0, true},
		{"replica_oob_idx", database.Postgres_Placement_SCOPE_REPLICA, uint32Ptr(5), 0, true},
		{"unspecified", database.Postgres_Placement_SCOPE_UNSPECIFIED, nil, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &database.Database_Template{
				Template: &database.Database_Template_PostgresCluster{
					PostgresCluster: &database.Postgres_Cluster_Template{
						Topology: &database.Postgres_Cluster_Template_Topology{
							Settings:        settings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
							MasterHardware:  hw(4, 8, 100),
							ReplicaHardware: hw(2, 4, 50),
							ReplicasCount:   2,
						},
						Addons: &database.Postgres_Addons{
							Dcs: &database.Postgres_Addons_Dcs{
								Etcd: &database.Postgres_Addons_Dcs_Etcd{
									Size: tt.size,
									Placement: &database.Postgres_Placement{
										Mode: &database.Postgres_Placement_Colocate_{
											Colocate: &database.Postgres_Placement_Colocate{
												Scope:        tt.scope,
												ReplicaIndex: tt.replicaIdx,
											},
										},
									},
								},
							},
						},
					},
				},
			}
			err := ValidateDatabaseTemplate(ctx, tmpl)
			if tt.expectError && err == nil {
				t.Fatal("expected error for etcd placement")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePostgresCluster_EtcdPlacement_Dedicated(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name           string
		instancesCount uint32
		size           uint32
		expectError    bool
	}{
		{"matching", 3, 3, false},
		{"size_greater", 2, 3, true},
		{"size_lesser", 4, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &database.Database_Template{
				Template: &database.Database_Template_PostgresCluster{
					PostgresCluster: &database.Postgres_Cluster_Template{
						Topology: &database.Postgres_Cluster_Template_Topology{
							Settings:        settings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
							MasterHardware:  hw(4, 8, 100),
							ReplicaHardware: hw(2, 4, 50),
							ReplicasCount:   2,
						},
						Addons: &database.Postgres_Addons{
							Dcs: &database.Postgres_Addons_Dcs{
								Etcd: &database.Postgres_Addons_Dcs_Etcd{
									Size: tt.size,
									Placement: &database.Postgres_Placement{
										Mode: &database.Postgres_Placement_Dedicated_{
											Dedicated: &database.Postgres_Placement_Dedicated{
												InstancesCount: tt.instancesCount,
												Hardware:       hw(2, 4, 50),
											},
										},
									},
								},
							},
						},
					},
				},
			}
			err := ValidateDatabaseTemplate(ctx, tmpl)
			if tt.expectError && err == nil {
				t.Fatal("expected error for etcd dedicated placement")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePostgresCluster_EtcdColocateSizeMismatch(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster_Template{
				Topology: &database.Postgres_Cluster_Template_Topology{
					Settings:        settings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
					MasterHardware:  hw(4, 8, 100),
					ReplicaHardware: hw(2, 4, 50),
					ReplicasCount:   2,
				},
				Addons: &database.Postgres_Addons{
					Dcs: &database.Postgres_Addons_Dcs{
						Etcd: &database.Postgres_Addons_Dcs_Etcd{
							Size: 5,
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
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for etcd size mismatch")
	}
}

func TestValidatePostgresCluster_PgbouncerPlacement(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster_Template{
				Topology: &database.Postgres_Cluster_Template_Topology{
					Settings:        settings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
					MasterHardware:  hw(4, 8, 100),
					ReplicaHardware: hw(2, 4, 50),
					ReplicasCount:   2,
				},
				Addons: &database.Postgres_Addons{
					Pooling: &database.Postgres_Addons_Pooling{
						Pgbouncer: &database.Postgres_Addons_Pooling_Pgbouncer{
							Enabled:  true,
							PoolSize: 20,
							PoolMode: database.Postgres_Addons_Pooling_Pgbouncer_TRANSACTION,
							Placement: &database.Postgres_Placement{
								Mode: &database.Postgres_Placement_Colocate_{
									Colocate: &database.Postgres_Placement_Colocate{
										Scope: database.Postgres_Placement_SCOPE_MASTER,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePostgresCluster_PgbouncerInvalidPlacement(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster_Template{
				Topology: &database.Postgres_Cluster_Template_Topology{
					Settings:        settings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
					MasterHardware:  hw(4, 8, 100),
					ReplicaHardware: hw(2, 4, 50),
					ReplicasCount:   2,
				},
				Addons: &database.Postgres_Addons{
					Pooling: &database.Postgres_Addons_Pooling{
						Pgbouncer: &database.Postgres_Addons_Pooling_Pgbouncer{
							Enabled:  true,
							PoolSize: 20,
							PoolMode: database.Postgres_Addons_Pooling_Pgbouncer_TRANSACTION,
							Placement: &database.Postgres_Placement{
								Mode: &database.Postgres_Placement_Colocate_{
									Colocate: &database.Postgres_Placement_Colocate{
										Scope: database.Postgres_Placement_SCOPE_UNSPECIFIED,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for invalid pgbouncer placement")
	}
}

func TestValidatePostgresCluster_BackupScopeInvalid(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster_Template{
				Topology: &database.Postgres_Cluster_Template_Topology{
					Settings:        settings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
					MasterHardware:  hw(4, 8, 100),
					ReplicaHardware: hw(2, 4, 50),
					ReplicasCount:   2,
				},
				Addons: &database.Postgres_Addons{
					Backup: &database.Postgres_Addons_Backup{
						Enabled: true,
						Scope:   database.Postgres_Placement_SCOPE_REPLICA,
						Config: &database.Postgres_Sidecar_Backup{
							Schedule: "0 3 * * *",
							Tool:     database.Postgres_Sidecar_Backup_WAL_G,
						},
					},
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for backup scope REPLICA without index")
	}
}

func TestValidatePostgresCluster_BackupScopeValid(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster_Template{
				Topology: &database.Postgres_Cluster_Template_Topology{
					Settings:        settings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
					MasterHardware:  hw(4, 8, 100),
					ReplicaHardware: hw(2, 4, 50),
					ReplicasCount:   2,
				},
				Addons: &database.Postgres_Addons{
					Backup: &database.Postgres_Addons_Backup{
						Enabled: true,
						Scope:   database.Postgres_Placement_SCOPE_MASTER,
						Config: &database.Postgres_Sidecar_Backup{
							Schedule: "0 3 * * *",
							Tool:     database.Postgres_Sidecar_Backup_WAL_G,
							Storage: &database.Postgres_Sidecar_Backup_Local{
								Local: &database.Postgres_Sidecar_Backup_LocalStorage{Path: "/backups"},
							},
						},
					},
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSettings_OrioledbValidVersion(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		version database.Postgres_Settings_Version
	}{
		{"orioledb_v16", database.Postgres_Settings_VERSION_16},
		{"orioledb_v17", database.Postgres_Settings_VERSION_17},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &database.Database_Template{
				Template: &database.Database_Template_PostgresInstance{
					PostgresInstance: &database.Postgres_Instance_Template{
						Settings: &database.Postgres_Settings{
							Version:       tt.version,
							StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_ORIOLEDB,
						},
						Hardware: hw(4, 8, 100),
					},
				},
			}
			err := ValidateDatabaseTemplate(ctx, tmpl)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateSettings_OrioledbInvalidVersion(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresInstance{
			PostgresInstance: &database.Postgres_Instance_Template{
				Settings: &database.Postgres_Settings{
					Version:       database.Postgres_Settings_VERSION_18,
					StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_ORIOLEDB,
				},
				Hardware: hw(4, 8, 100),
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for orioledb with invalid version")
	}
}

func uint32Ptr(v uint32) *uint32 {
	return &v
}
