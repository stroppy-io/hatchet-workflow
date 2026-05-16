package connect

import (
	"context"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/catalog"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

// CatalogHandler implements DatabasePresetServiceHandler, WorkloadPresetServiceHandler,
// PackageServiceHandler, and SettingsServiceHandler.
type CatalogHandler struct{ svc *catalog.Service }

// NewCatalogHandler constructs a CatalogHandler.
func NewCatalogHandler(svc *catalog.Service) *CatalogHandler { return &CatalogHandler{svc: svc} }

// ─── DatabasePresetService ───────────────────────────────────────────────────

func (h *CatalogHandler) CreateDatabasePreset(ctx context.Context, req *connect.Request[catalogpb.CreateDatabasePresetRequest]) (*connect.Response[catalogpb.DatabasePreset], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	result, err := h.svc.CreateDatabasePreset(ctx, tenantID, callerID, req.Msg.GetPreset())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *CatalogHandler) UpdateDatabasePreset(ctx context.Context, req *connect.Request[catalogpb.UpdateDatabasePresetRequest]) (*connect.Response[catalogpb.DatabasePreset], error) {
	result, err := h.svc.UpdateDatabasePreset(ctx, req.Msg.GetPreset())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *CatalogHandler) DeleteDatabasePreset(ctx context.Context, req *connect.Request[catalogpb.DatabasePresetId]) (*connect.Response[catalogpb.DatabasePreset], error) {
	result, err := h.svc.DeleteDatabasePresetAndReturn(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *CatalogHandler) GetDatabasePreset(ctx context.Context, req *connect.Request[catalogpb.DatabasePresetId]) (*connect.Response[catalogpb.DatabasePreset], error) {
	result, err := h.svc.GetDatabasePreset(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *CatalogHandler) ListDatabasePresets(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[catalogpb.DatabasePreset_List], error) {
	tenantID := req.Msg
	if tenantID.GetValue() == "" {
		tenantID = &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	}
	items, err := h.svc.ListDatabasePresets(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&catalogpb.DatabasePreset_List{DatabasePresets: items}), nil
}

func (h *CatalogHandler) CloneDatabasePreset(ctx context.Context, req *connect.Request[catalogpb.DatabasePresetId]) (*connect.Response[catalogpb.DatabasePreset], error) {
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	result, err := h.svc.CloneDatabasePreset(ctx, req.Msg, callerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

// ─── WorkloadPresetService ───────────────────────────────────────────────────

func (h *CatalogHandler) CreateWorkloadPreset(ctx context.Context, req *connect.Request[catalogpb.CreateWorkloadPresetRequest]) (*connect.Response[catalogpb.WorkloadPreset], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	result, err := h.svc.CreateWorkloadPreset(ctx, tenantID, callerID, req.Msg.GetPreset())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *CatalogHandler) UpdateWorkloadPreset(ctx context.Context, req *connect.Request[catalogpb.UpdateWorkloadPresetRequest]) (*connect.Response[catalogpb.WorkloadPreset], error) {
	result, err := h.svc.UpdateWorkloadPreset(ctx, req.Msg.GetPreset())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *CatalogHandler) DeleteWorkloadPreset(ctx context.Context, req *connect.Request[catalogpb.WorkloadPresetId]) (*connect.Response[catalogpb.WorkloadPreset], error) {
	result, err := h.svc.DeleteWorkloadPresetAndReturn(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *CatalogHandler) GetWorkloadPreset(ctx context.Context, req *connect.Request[catalogpb.WorkloadPresetId]) (*connect.Response[catalogpb.WorkloadPreset], error) {
	result, err := h.svc.GetWorkloadPreset(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *CatalogHandler) ListWorkloadPresets(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[catalogpb.WorkloadPreset_List], error) {
	tenantID := req.Msg
	if tenantID.GetValue() == "" {
		tenantID = &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	}
	items, err := h.svc.ListWorkloadPresets(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&catalogpb.WorkloadPreset_List{WorkloadPresets: items}), nil
}

func (h *CatalogHandler) CloneWorkloadPreset(ctx context.Context, req *connect.Request[catalogpb.WorkloadPresetId]) (*connect.Response[catalogpb.WorkloadPreset], error) {
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	result, err := h.svc.CloneWorkloadPreset(ctx, req.Msg, callerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

// ─── PackageService ──────────────────────────────────────────────────────────

func (h *CatalogHandler) CreatePackage(ctx context.Context, req *connect.Request[catalogpb.CreatePackageRequest]) (*connect.Response[catalogpb.Package], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	result, err := h.svc.CreatePackage(ctx, tenantID, callerID, req.Msg.GetPackage())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *CatalogHandler) UpdatePackage(ctx context.Context, req *connect.Request[catalogpb.UpdatePackageRequest]) (*connect.Response[catalogpb.Package], error) {
	result, err := h.svc.UpdatePackage(ctx, req.Msg.GetPackage())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *CatalogHandler) DeletePackage(ctx context.Context, req *connect.Request[catalogpb.PackageId]) (*connect.Response[catalogpb.Package], error) {
	result, err := h.svc.DeletePackageAndReturn(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *CatalogHandler) GetPackage(ctx context.Context, req *connect.Request[catalogpb.PackageId]) (*connect.Response[catalogpb.Package], error) {
	result, err := h.svc.GetPackage(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *CatalogHandler) ListPackages(ctx context.Context, req *connect.Request[catalogpb.ListPackagesRequest]) (*connect.Response[catalogpb.Package_List], error) {
	tenantID := req.Msg.GetTenantId()
	if tenantID.GetValue() == "" {
		tenantID = &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	}
	var dbKind *catalogpb.Database_Kind
	if req.Msg.GetDbKind() != catalogpb.Database_DATABASE_KIND_UNSPECIFIED {
		k := req.Msg.GetDbKind()
		dbKind = &k
	}
	items, err := h.svc.ListPackages(ctx, tenantID, dbKind)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&catalogpb.Package_List{Packages: items}), nil
}

func (h *CatalogHandler) ClonePackage(ctx context.Context, req *connect.Request[catalogpb.PackageId]) (*connect.Response[catalogpb.Package], error) {
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	result, err := h.svc.ClonePackage(ctx, req.Msg, callerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

// ─── SettingsService ─────────────────────────────────────────────────────────

func (h *CatalogHandler) ListSettings(ctx context.Context, req *connect.Request[catalogpb.ListSettingsRequest]) (*connect.Response[catalogpb.SettingsItem_List], error) {
	tenantID := req.Msg.GetTenantId()
	if tenantID.GetValue() == "" {
		tenantID = &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	}
	items, err := h.svc.ListSettings(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&catalogpb.SettingsItem_List{SettingsItems: items}), nil
}

func (h *CatalogHandler) SetSetting(ctx context.Context, req *connect.Request[catalogpb.SetSettingRequest]) (*connect.Response[catalogpb.SettingsItem], error) {
	existing, err := h.svc.GetSettingByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	result, err := h.svc.SetSetting(ctx, existing.GetTenantId(), existing.GetPart(), existing.GetKey(), req.Msg.GetValue())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *CatalogHandler) SetSettingMany(ctx context.Context, req *connect.Request[catalogpb.SetSettingManyRequest]) (*connect.Response[catalogpb.SettingsItem_List], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	// Convert SetSettingRequest entries into SettingsItem list for SetSettingMany.
	items := make([]*catalogpb.SettingsItem, 0, len(req.Msg.GetSettings()))
	for _, s := range req.Msg.GetSettings() {
		existing, err := h.svc.GetSettingByID(ctx, s.GetId())
		if err != nil {
			return nil, err
		}
		items = append(items, &catalogpb.SettingsItem{
			Part:  existing.GetPart(),
			Key:   existing.GetKey(),
			Value: s.GetValue(),
		})
	}
	results, err := h.svc.SetSettingMany(ctx, tenantID, items)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&catalogpb.SettingsItem_List{SettingsItems: results}), nil
}
