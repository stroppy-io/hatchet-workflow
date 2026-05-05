package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const ioM3ChunkGB = 93

// roundIOM3 rounds a disk size up to the next 93 GiB chunk. Mirrors
// run.roundIOM3GB; duplicated here to keep the migration self-contained
// (postgres package must not import the run domain package).
func roundIOM3(size int) int {
	if size <= 0 {
		return size
	}
	chunks := (size + ioM3ChunkGB - 1) / ioM3ChunkGB
	if chunks < 1 {
		chunks = 1
	}
	return chunks * ioM3ChunkGB
}

// fixIOM3InNode walks an arbitrary preset-topology JSON tree and rounds any
// MachineSpec-shaped object whose disk_type is network-ssd-io-m3 (and any
// SecondaryDisk with type=network-ssd-io-m3) up to the chunk boundary.
// Returns true when at least one value was changed so the caller knows
// whether to write the row back.
func fixIOM3InNode(node any) (bool, any) {
	switch v := node.(type) {
	case map[string]any:
		changed := false
		// Boot disk on this object.
		if dt, ok := v["disk_type"].(string); ok && dt == "network-ssd-io-m3" {
			if dg, ok := v["disk_gb"].(float64); ok {
				rounded := roundIOM3(int(dg))
				if rounded != int(dg) {
					v["disk_gb"] = float64(rounded)
					changed = true
				}
			}
		}
		// Generic SecondaryDisk shape: { device_name, size_gb, type }.
		if t, ok := v["type"].(string); ok && t == "network-ssd-io-m3" {
			if sg, ok := v["size_gb"].(float64); ok {
				rounded := roundIOM3(int(sg))
				if rounded != int(sg) {
					v["size_gb"] = float64(rounded)
					changed = true
				}
			}
		}
		// Recurse into all values.
		for _, vv := range v {
			if c, _ := fixIOM3InNode(vv); c {
				changed = true
			}
		}
		return changed, v
	case []any:
		changed := false
		for i := range v {
			if c, _ := fixIOM3InNode(v[i]); c {
				changed = true
			}
		}
		return changed, v
	}
	return false, node
}

// MigrateIOM3Presets walks every row in `presets` and rounds any io-m3 disk
// size up to a 93 GiB multiple. Yandex Cloud rejects io-m3 disks whose size
// isn't a multiple of 99857989632 bytes (= 93 GiB); pre-existing presets
// created before the frontend chunk constraint may carry illegal sizes.
// Idempotent: rounding a value already at a 93 multiple is a no-op.
func MigrateIOM3Presets(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) error {
	rows, err := pool.Query(ctx, `SELECT id, tenant_id, topology FROM presets`)
	if err != nil {
		return fmt.Errorf("io-m3 migration: query presets: %w", err)
	}
	type row struct {
		id, tenantID, topology string
	}
	var batch []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.tenantID, &r.topology); err != nil {
			rows.Close()
			return fmt.Errorf("io-m3 migration: scan: %w", err)
		}
		batch = append(batch, r)
	}
	rows.Close()

	fixed := 0
	for _, r := range batch {
		var node any
		if err := json.Unmarshal([]byte(r.topology), &node); err != nil {
			// Malformed JSON in DB — skip, log, keep going.
			if logger != nil {
				logger.Warn("io-m3 migration: skip malformed preset topology",
					zap.String("preset_id", r.id), zap.Error(err))
			}
			continue
		}
		changed, fixedNode := fixIOM3InNode(node)
		if !changed {
			continue
		}
		newJSON, err := json.Marshal(fixedNode)
		if err != nil {
			if logger != nil {
				logger.Warn("io-m3 migration: marshal failed",
					zap.String("preset_id", r.id), zap.Error(err))
			}
			continue
		}
		_, err = pool.Exec(ctx,
			`UPDATE presets SET topology = $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3`,
			string(newJSON), r.id, r.tenantID,
		)
		if err != nil {
			if logger != nil {
				logger.Warn("io-m3 migration: update failed",
					zap.String("preset_id", r.id), zap.Error(err))
			}
			continue
		}
		fixed++
	}
	if logger != nil && fixed > 0 {
		logger.Info("io-m3 preset migration applied", zap.Int("rows_updated", fixed))
	}
	return nil
}
