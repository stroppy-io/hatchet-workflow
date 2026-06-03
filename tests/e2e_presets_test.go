//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	api "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
)

// TestE2EPresets exercises the database/test preset surface: list the builtin
// presets seeded at first boot, fetch one, clone it (create), fetch the clone,
// then delete the clone.
func TestE2EPresets(t *testing.T) {
	e := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// List builtin database presets (21 seeded at first boot).
	lr, err := e.dbPresets.ListDatabasePresets(ctx, &api.ListDatabasePresetsRequest{
		TenantId: e.tenantID,
	})
	if err != nil {
		t.Fatalf("ListDatabasePresets: %v", err)
	}
	presets := lr.GetPresets()
	if len(presets) == 0 {
		t.Fatal("ListDatabasePresets: expected seeded builtin presets, got 0")
	}
	t.Logf("database presets: %d", len(presets))

	first := presets[0]
	srcID := first.GetEntity().GetId()

	// Get the preset by id.
	gr, err := e.dbPresets.GetDatabasePreset(ctx, &api.GetDatabasePresetRequest{
		TenantId: e.tenantID,
		Id:       srcID,
	})
	if err != nil {
		t.Fatalf("GetDatabasePreset(%s): %v", srcID, err)
	}
	if gr.GetPreset().GetEntity().GetId() != srcID {
		t.Fatalf("GetDatabasePreset returned id %q, want %q", gr.GetPreset().GetEntity().GetId(), srcID)
	}

	// Clone it (a create path that does not require building a full record).
	cr, err := e.dbPresets.CloneDatabasePreset(ctx, &api.CloneDatabasePresetRequest{
		TenantId: e.tenantID,
		Id:       srcID,
		Name:     "e2e-clone",
	})
	if err != nil {
		t.Fatalf("CloneDatabasePreset(%s): %v", srcID, err)
	}
	cloneID := cr.GetPreset().GetEntity().GetId()
	if cloneID == "" || cloneID == srcID {
		t.Fatalf("CloneDatabasePreset: bad clone id %q (src %q)", cloneID, srcID)
	}
	t.Logf("cloned preset %s -> %s", srcID, cloneID)

	// Verify the clone is fetchable.
	if _, err := e.dbPresets.GetDatabasePreset(ctx, &api.GetDatabasePresetRequest{
		TenantId: e.tenantID,
		Id:       cloneID,
	}); err != nil {
		t.Fatalf("GetDatabasePreset(clone %s): %v", cloneID, err)
	}

	// Delete the clone.
	if _, err := e.dbPresets.DeleteDatabasePreset(ctx, &api.DeleteDatabasePresetRequest{
		TenantId: e.tenantID,
		Id:       cloneID,
	}); err != nil {
		t.Fatalf("DeleteDatabasePreset(%s): %v", cloneID, err)
	}
	t.Logf("deleted clone %s", cloneID)

	// Test presets list should be reachable too (may be empty).
	tp, err := e.testPresets.ListTestPresets(ctx, &api.ListTestPresetsRequest{
		TenantId: e.tenantID,
	})
	if err != nil {
		t.Fatalf("ListTestPresets: %v", err)
	}
	t.Logf("test presets: %d", len(tp.GetPresets()))
}
