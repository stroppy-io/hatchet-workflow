package database

import (
	"context"
	"fmt"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
)

func ValidateDatabaseTemplate(ctx context.Context, template *database.Database_Template) error {
	_ = ctx
	if template == nil {
		return fmt.Errorf("database template is nil")
	}
	if err := template.Validate(); err != nil {
		return err
	}

	switch t := template.Template.(type) {
	case *database.Database_Template_PostgresInstance:
		return validatePostgresInstanceTemplate(t.PostgresInstance)
	case *database.Database_Template_PostgresCluster:
		return validatePostgresClusterTemplate(t.PostgresCluster)
	case nil:
		return fmt.Errorf("database template content is nil")
	default:
		return fmt.Errorf("unknown database template type")
	}
}

func validatePostgresInstanceTemplate(tmpl *database.Postgres_Instance_Template) error {
	if tmpl == nil {
		return fmt.Errorf("postgres instance template is nil")
	}
	if err := validateSettings(tmpl.GetSettings()); err != nil {
		return err
	}

	if patroni := tmpl.GetSettings().GetPatroni(); patroni != nil && patroni.GetEnabled() {
		return fmt.Errorf("patroni is only supported for postgres cluster template")
	}

	return nil
}

func validatePostgresClusterTemplate(tmpl *database.Postgres_Cluster_Template) error {
	if tmpl == nil {
		return fmt.Errorf("postgres cluster template is nil")
	}
	topology := tmpl.GetTopology()
	if topology == nil {
		return fmt.Errorf("cluster topology is required")
	}
	if err := validateSettings(topology.GetSettings()); err != nil {
		return err
	}

	replicaCount := int(topology.GetReplicasCount())

	seenReplicaOverrides := make(map[uint32]struct{})
	for _, override := range tmpl.GetReplicaOverrides() {
		if override == nil {
			continue
		}
		idx := override.GetReplicaIndex()
		if int(idx) >= replicaCount {
			return fmt.Errorf("replica override index %d out of bounds for replicas_count=%d", idx, replicaCount)
		}
		if _, exists := seenReplicaOverrides[idx]; exists {
			return fmt.Errorf("replica override for index %d is duplicated", idx)
		}
		seenReplicaOverrides[idx] = struct{}{}
		if override.GetSettings() != nil {
			if err := validateSettings(override.GetSettings()); err != nil {
				return fmt.Errorf("replica override %d settings error: %w", idx, err)
			}
		}
	}

	patroni := topology.GetSettings().GetPatroni()
	if patroni != nil && patroni.GetEnabled() {
		if err := validatePatroni(patroni, replicaCount); err != nil {
			return err
		}
		if tmpl.GetAddons().GetDcs().GetEtcd() == nil {
			return fmt.Errorf("patroni is enabled but addons.dcs.etcd is not configured")
		}
	}

	if err := validateAddons(tmpl.GetAddons(), replicaCount); err != nil {
		return err
	}

	return nil
}

func validateSettings(s *database.Postgres_Settings) error {
	if s == nil {
		return fmt.Errorf("postgres settings is nil")
	}
	if s.GetStorageEngine() == database.Postgres_Settings_STORAGE_ENGINE_ORIOLEDB {
		v := s.GetVersion()
		if v != database.Postgres_Settings_VERSION_16 && v != database.Postgres_Settings_VERSION_17 {
			return fmt.Errorf("orioledb storage engine is only supported on postgres versions 16 and 17")
		}
	}
	return nil
}

func validatePatroni(patroni *database.Postgres_Settings_Patroni, replicaCount int) error {
	if patroni == nil || !patroni.GetEnabled() {
		return nil
	}
	if patroni.GetSynchronousMode() {
		requiredReplicas := int(patroni.GetSynchronousNodeCount())
		if requiredReplicas == 0 {
			requiredReplicas = 1
		}
		if replicaCount < requiredReplicas {
			return fmt.Errorf("synchronous_mode is enabled with %d sync nodes, but only %d replicas defined", requiredReplicas, replicaCount)
		}
	}
	return nil
}

func validateAddons(addons *database.Postgres_Addons, replicaCount int) error {
	if addons == nil {
		return nil
	}

	etcd := addons.GetDcs().GetEtcd()
	if etcd != nil {
		if err := validatePlacement(etcd.GetPlacement(), replicaCount); err != nil {
			return fmt.Errorf("addons.dcs.etcd placement error: %w", err)
		}
		if err := validateEtcdPlacementSize(etcd, replicaCount); err != nil {
			return err
		}
	}

	pgbouncer := addons.GetPooling().GetPgbouncer()
	if pgbouncer != nil && pgbouncer.GetEnabled() {
		if err := validatePlacement(pgbouncer.GetPlacement(), replicaCount); err != nil {
			return fmt.Errorf("addons.pooling.pgbouncer placement error: %w", err)
		}
	}

	if backup := addons.GetBackup(); backup != nil && backup.GetEnabled() {
		if backup.GetScope() == database.Postgres_Placement_SCOPE_REPLICA {
			return fmt.Errorf("addons.backup.scope=SCOPE_REPLICA is not supported without replica index selector")
		}
	}

	return nil
}

func validateEtcdPlacementSize(etcd *database.Postgres_Addons_Dcs_Etcd, replicaCount int) error {
	if etcd == nil {
		return nil
	}
	placement := etcd.GetPlacement()
	if placement == nil {
		return nil
	}

	targetSize := int(etcd.GetSize())
	switch mode := placement.GetMode().(type) {
	case *database.Postgres_Placement_Dedicated_:
		if int(mode.Dedicated.GetInstancesCount()) != targetSize {
			return fmt.Errorf("addons.dcs.etcd.size=%d must equal dedicated.instances_count=%d", targetSize, mode.Dedicated.GetInstancesCount())
		}
	case *database.Postgres_Placement_Colocate_:
		covered := colocateScopeSize(mode.Colocate, replicaCount)
		if covered != targetSize {
			return fmt.Errorf("addons.dcs.etcd.size=%d does not match placement coverage=%d", targetSize, covered)
		}
	}
	return nil
}

func validatePlacement(placement *database.Postgres_Placement, replicaCount int) error {
	if placement == nil {
		return fmt.Errorf("placement is nil")
	}
	switch mode := placement.GetMode().(type) {
	case *database.Postgres_Placement_Colocate_:
		c := mode.Colocate
		if c == nil {
			return fmt.Errorf("colocate mode is nil")
		}
		scope := c.GetScope()
		if scope == database.Postgres_Placement_SCOPE_UNSPECIFIED {
			return fmt.Errorf("colocate scope must be specified")
		}
		if scope == database.Postgres_Placement_SCOPE_REPLICA {
			if c.ReplicaIndex == nil {
				return fmt.Errorf("colocate replica scope requires replica_index")
			}
			if int(c.GetReplicaIndex()) >= replicaCount {
				return fmt.Errorf("colocate replica_index=%d out of bounds for replicas_count=%d", c.GetReplicaIndex(), replicaCount)
			}
		} else if c.ReplicaIndex != nil {
			return fmt.Errorf("replica_index can be set only for colocate scope SCOPE_REPLICA")
		}
	case *database.Postgres_Placement_Dedicated_:
		if mode.Dedicated == nil {
			return fmt.Errorf("dedicated mode is nil")
		}
	default:
		return fmt.Errorf("placement mode is not set")
	}
	return nil
}

func colocateScopeSize(colocate *database.Postgres_Placement_Colocate, replicaCount int) int {
	if colocate == nil {
		return 0
	}
	switch colocate.GetScope() {
	case database.Postgres_Placement_SCOPE_MASTER:
		return 1
	case database.Postgres_Placement_SCOPE_REPLICAS:
		return replicaCount
	case database.Postgres_Placement_SCOPE_ALL_NODES:
		return replicaCount + 1
	case database.Postgres_Placement_SCOPE_REPLICA:
		return 1
	default:
		return 0
	}
}
