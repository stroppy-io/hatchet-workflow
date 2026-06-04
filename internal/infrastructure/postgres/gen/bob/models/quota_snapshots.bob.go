// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/bob/expr"
)

// QuotaSnapshot is an object representing the database table.
type QuotaSnapshot struct {
	TenantID          string          `db:"tenant_id,pk" `
	Provider          int32           `db:"provider,pk" `
	ResourceType      string          `db:"resource_type,pk" `
	ResourceID        string          `db:"resource_id,pk" `
	Service           string          `db:"service" `
	QuotaName         string          `db:"quota_name,pk" `
	Units             string          `db:"units" `
	ProviderUsed      string          `db:"provider_used" `
	QuotaLimit        string          `db:"quota_limit" `
	ProviderAvailable string          `db:"provider_available" `
	ObservedAt        time.Time       `db:"observed_at" `
	StaleAfter        time.Time       `db:"stale_after" `
	Raw               json.RawMessage `db:"raw" `
}

// QuotaSnapshotSlice is an alias for a slice of pointers to QuotaSnapshot.
// This should almost always be used instead of []*QuotaSnapshot.
type QuotaSnapshotSlice []*QuotaSnapshot

// QuotaSnapshots contains methods to work with the quota_snapshots table
var QuotaSnapshots = psql.NewTablex[*QuotaSnapshot, QuotaSnapshotSlice, *QuotaSnapshotSetter]("", "quota_snapshots", buildQuotaSnapshotColumns("quota_snapshots"))

// QuotaSnapshotsQuery is a query on the quota_snapshots table
type QuotaSnapshotsQuery = *psql.ViewQuery[*QuotaSnapshot, QuotaSnapshotSlice]

func buildQuotaSnapshotColumns(tableName string) quotaSnapshotColumns {
	columnsExpr := expr.NewColumnsExpr(
		"tenant_id", "provider", "resource_type", "resource_id", "service", "quota_name", "units", "provider_used", "quota_limit", "provider_available", "observed_at", "stale_after", "raw",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return quotaSnapshotColumns{
		ColumnsExpr:       columnsExpr,
		tableAlias:        tableName,
		TenantID:          buildQuotaSnapshotColumn(tableName, "tenant_id"),
		Provider:          buildQuotaSnapshotColumn(tableName, "provider"),
		ResourceType:      buildQuotaSnapshotColumn(tableName, "resource_type"),
		ResourceID:        buildQuotaSnapshotColumn(tableName, "resource_id"),
		Service:           buildQuotaSnapshotColumn(tableName, "service"),
		QuotaName:         buildQuotaSnapshotColumn(tableName, "quota_name"),
		Units:             buildQuotaSnapshotColumn(tableName, "units"),
		ProviderUsed:      buildQuotaSnapshotColumn(tableName, "provider_used"),
		QuotaLimit:        buildQuotaSnapshotColumn(tableName, "quota_limit"),
		ProviderAvailable: buildQuotaSnapshotColumn(tableName, "provider_available"),
		ObservedAt:        buildQuotaSnapshotColumn(tableName, "observed_at"),
		StaleAfter:        buildQuotaSnapshotColumn(tableName, "stale_after"),
		Raw:               buildQuotaSnapshotColumn(tableName, "raw"),
	}
}

type quotaSnapshotColumns struct {
	expr.ColumnsExpr
	tableAlias        string
	TenantID          quotaSnapshotColumn
	Provider          quotaSnapshotColumn
	ResourceType      quotaSnapshotColumn
	ResourceID        quotaSnapshotColumn
	Service           quotaSnapshotColumn
	QuotaName         quotaSnapshotColumn
	Units             quotaSnapshotColumn
	ProviderUsed      quotaSnapshotColumn
	QuotaLimit        quotaSnapshotColumn
	ProviderAvailable quotaSnapshotColumn
	ObservedAt        quotaSnapshotColumn
	StaleAfter        quotaSnapshotColumn
	Raw               quotaSnapshotColumn
}

// Alias returns the current table alias for the columns set.
func (c quotaSnapshotColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (quotaSnapshotColumns) AliasedAs(tableName string) quotaSnapshotColumns {
	return buildQuotaSnapshotColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c quotaSnapshotColumns) Unqualified() quotaSnapshotColumns {
	return buildQuotaSnapshotColumns("")
}

func buildQuotaSnapshotColumn(alias, name string) quotaSnapshotColumn {
	return quotaSnapshotColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type quotaSnapshotColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c quotaSnapshotColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c quotaSnapshotColumn) ShouldOmitParens() bool {
	return true
}

// QuotaSnapshotSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type QuotaSnapshotSetter struct {
	TenantID          *string          `db:"tenant_id,pk" `
	Provider          *int32           `db:"provider,pk" `
	ResourceType      *string          `db:"resource_type,pk" `
	ResourceID        *string          `db:"resource_id,pk" `
	Service           *string          `db:"service" `
	QuotaName         *string          `db:"quota_name,pk" `
	Units             *string          `db:"units" `
	ProviderUsed      *string          `db:"provider_used" `
	QuotaLimit        *string          `db:"quota_limit" `
	ProviderAvailable *string          `db:"provider_available" `
	ObservedAt        *time.Time       `db:"observed_at" `
	StaleAfter        *time.Time       `db:"stale_after" `
	Raw               *json.RawMessage `db:"raw" `
}

func (s QuotaSnapshotSetter) SetColumns() []string {
	vals := make([]string, 0, 13)
	if s.TenantID != nil {
		vals = append(vals, "tenant_id")
	}
	if s.Provider != nil {
		vals = append(vals, "provider")
	}
	if s.ResourceType != nil {
		vals = append(vals, "resource_type")
	}
	if s.ResourceID != nil {
		vals = append(vals, "resource_id")
	}
	if s.Service != nil {
		vals = append(vals, "service")
	}
	if s.QuotaName != nil {
		vals = append(vals, "quota_name")
	}
	if s.Units != nil {
		vals = append(vals, "units")
	}
	if s.ProviderUsed != nil {
		vals = append(vals, "provider_used")
	}
	if s.QuotaLimit != nil {
		vals = append(vals, "quota_limit")
	}
	if s.ProviderAvailable != nil {
		vals = append(vals, "provider_available")
	}
	if s.ObservedAt != nil {
		vals = append(vals, "observed_at")
	}
	if s.StaleAfter != nil {
		vals = append(vals, "stale_after")
	}
	if s.Raw != nil {
		vals = append(vals, "raw")
	}
	return vals
}

func (s QuotaSnapshotSetter) Overwrite(t *QuotaSnapshot) {
	if s.TenantID != nil {
		t.TenantID = func() string {
			if s.TenantID == nil {
				return *new(string)
			}
			return *s.TenantID
		}()
	}
	if s.Provider != nil {
		t.Provider = func() int32 {
			if s.Provider == nil {
				return *new(int32)
			}
			return *s.Provider
		}()
	}
	if s.ResourceType != nil {
		t.ResourceType = func() string {
			if s.ResourceType == nil {
				return *new(string)
			}
			return *s.ResourceType
		}()
	}
	if s.ResourceID != nil {
		t.ResourceID = func() string {
			if s.ResourceID == nil {
				return *new(string)
			}
			return *s.ResourceID
		}()
	}
	if s.Service != nil {
		t.Service = func() string {
			if s.Service == nil {
				return *new(string)
			}
			return *s.Service
		}()
	}
	if s.QuotaName != nil {
		t.QuotaName = func() string {
			if s.QuotaName == nil {
				return *new(string)
			}
			return *s.QuotaName
		}()
	}
	if s.Units != nil {
		t.Units = func() string {
			if s.Units == nil {
				return *new(string)
			}
			return *s.Units
		}()
	}
	if s.ProviderUsed != nil {
		t.ProviderUsed = func() string {
			if s.ProviderUsed == nil {
				return *new(string)
			}
			return *s.ProviderUsed
		}()
	}
	if s.QuotaLimit != nil {
		t.QuotaLimit = func() string {
			if s.QuotaLimit == nil {
				return *new(string)
			}
			return *s.QuotaLimit
		}()
	}
	if s.ProviderAvailable != nil {
		t.ProviderAvailable = func() string {
			if s.ProviderAvailable == nil {
				return *new(string)
			}
			return *s.ProviderAvailable
		}()
	}
	if s.ObservedAt != nil {
		t.ObservedAt = func() time.Time {
			if s.ObservedAt == nil {
				return *new(time.Time)
			}
			return *s.ObservedAt
		}()
	}
	if s.StaleAfter != nil {
		t.StaleAfter = func() time.Time {
			if s.StaleAfter == nil {
				return *new(time.Time)
			}
			return *s.StaleAfter
		}()
	}
	if s.Raw != nil {
		t.Raw = func() json.RawMessage {
			if s.Raw == nil {
				return *new(json.RawMessage)
			}
			return *s.Raw
		}()
	}
}

func (s *QuotaSnapshotSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return QuotaSnapshots.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 13)
		if s.TenantID != nil {
			vals[0] = psql.Arg(func() string {
				if s.TenantID == nil {
					return *new(string)
				}
				return *s.TenantID
			}())
		} else {
			vals[0] = psql.Raw("DEFAULT")
		}

		if s.Provider != nil {
			vals[1] = psql.Arg(func() int32 {
				if s.Provider == nil {
					return *new(int32)
				}
				return *s.Provider
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.ResourceType != nil {
			vals[2] = psql.Arg(func() string {
				if s.ResourceType == nil {
					return *new(string)
				}
				return *s.ResourceType
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		if s.ResourceID != nil {
			vals[3] = psql.Arg(func() string {
				if s.ResourceID == nil {
					return *new(string)
				}
				return *s.ResourceID
			}())
		} else {
			vals[3] = psql.Raw("DEFAULT")
		}

		if s.Service != nil {
			vals[4] = psql.Arg(func() string {
				if s.Service == nil {
					return *new(string)
				}
				return *s.Service
			}())
		} else {
			vals[4] = psql.Raw("DEFAULT")
		}

		if s.QuotaName != nil {
			vals[5] = psql.Arg(func() string {
				if s.QuotaName == nil {
					return *new(string)
				}
				return *s.QuotaName
			}())
		} else {
			vals[5] = psql.Raw("DEFAULT")
		}

		if s.Units != nil {
			vals[6] = psql.Arg(func() string {
				if s.Units == nil {
					return *new(string)
				}
				return *s.Units
			}())
		} else {
			vals[6] = psql.Raw("DEFAULT")
		}

		if s.ProviderUsed != nil {
			vals[7] = psql.Arg(func() string {
				if s.ProviderUsed == nil {
					return *new(string)
				}
				return *s.ProviderUsed
			}())
		} else {
			vals[7] = psql.Raw("DEFAULT")
		}

		if s.QuotaLimit != nil {
			vals[8] = psql.Arg(func() string {
				if s.QuotaLimit == nil {
					return *new(string)
				}
				return *s.QuotaLimit
			}())
		} else {
			vals[8] = psql.Raw("DEFAULT")
		}

		if s.ProviderAvailable != nil {
			vals[9] = psql.Arg(func() string {
				if s.ProviderAvailable == nil {
					return *new(string)
				}
				return *s.ProviderAvailable
			}())
		} else {
			vals[9] = psql.Raw("DEFAULT")
		}

		if s.ObservedAt != nil {
			vals[10] = psql.Arg(func() time.Time {
				if s.ObservedAt == nil {
					return *new(time.Time)
				}
				return *s.ObservedAt
			}())
		} else {
			vals[10] = psql.Raw("DEFAULT")
		}

		if s.StaleAfter != nil {
			vals[11] = psql.Arg(func() time.Time {
				if s.StaleAfter == nil {
					return *new(time.Time)
				}
				return *s.StaleAfter
			}())
		} else {
			vals[11] = psql.Raw("DEFAULT")
		}

		if s.Raw != nil {
			vals[12] = psql.Arg(func() json.RawMessage {
				if s.Raw == nil {
					return *new(json.RawMessage)
				}
				return *s.Raw
			}())
		} else {
			vals[12] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s QuotaSnapshotSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s QuotaSnapshotSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 13)

	if s.TenantID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "tenant_id")...),
			psql.Arg(s.TenantID),
		}})
	}

	if s.Provider != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "provider")...),
			psql.Arg(s.Provider),
		}})
	}

	if s.ResourceType != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "resource_type")...),
			psql.Arg(s.ResourceType),
		}})
	}

	if s.ResourceID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "resource_id")...),
			psql.Arg(s.ResourceID),
		}})
	}

	if s.Service != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "service")...),
			psql.Arg(s.Service),
		}})
	}

	if s.QuotaName != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "quota_name")...),
			psql.Arg(s.QuotaName),
		}})
	}

	if s.Units != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "units")...),
			psql.Arg(s.Units),
		}})
	}

	if s.ProviderUsed != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "provider_used")...),
			psql.Arg(s.ProviderUsed),
		}})
	}

	if s.QuotaLimit != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "quota_limit")...),
			psql.Arg(s.QuotaLimit),
		}})
	}

	if s.ProviderAvailable != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "provider_available")...),
			psql.Arg(s.ProviderAvailable),
		}})
	}

	if s.ObservedAt != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "observed_at")...),
			psql.Arg(s.ObservedAt),
		}})
	}

	if s.StaleAfter != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "stale_after")...),
			psql.Arg(s.StaleAfter),
		}})
	}

	if s.Raw != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "raw")...),
			psql.Arg(s.Raw),
		}})
	}

	return exprs
}

// FindQuotaSnapshot retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindQuotaSnapshot(ctx context.Context, exec bob.Executor, TenantIDPK string, ProviderPK int32, ResourceTypePK string, ResourceIDPK string, QuotaNamePK string, cols ...string) (*QuotaSnapshot, error) {
	if len(cols) == 0 {
		return QuotaSnapshots.Query(
			sm.Where(QuotaSnapshots.Columns.TenantID.EQ(psql.Arg(TenantIDPK))),
			sm.Where(QuotaSnapshots.Columns.Provider.EQ(psql.Arg(ProviderPK))),
			sm.Where(QuotaSnapshots.Columns.ResourceType.EQ(psql.Arg(ResourceTypePK))),
			sm.Where(QuotaSnapshots.Columns.ResourceID.EQ(psql.Arg(ResourceIDPK))),
			sm.Where(QuotaSnapshots.Columns.QuotaName.EQ(psql.Arg(QuotaNamePK))),
		).One(ctx, exec)
	}

	return QuotaSnapshots.Query(
		sm.Where(QuotaSnapshots.Columns.TenantID.EQ(psql.Arg(TenantIDPK))),
		sm.Where(QuotaSnapshots.Columns.Provider.EQ(psql.Arg(ProviderPK))),
		sm.Where(QuotaSnapshots.Columns.ResourceType.EQ(psql.Arg(ResourceTypePK))),
		sm.Where(QuotaSnapshots.Columns.ResourceID.EQ(psql.Arg(ResourceIDPK))),
		sm.Where(QuotaSnapshots.Columns.QuotaName.EQ(psql.Arg(QuotaNamePK))),
		sm.Columns(QuotaSnapshots.Columns.Only(cols...)),
	).One(ctx, exec)
}

// QuotaSnapshotExists checks the presence of a single record by primary key
func QuotaSnapshotExists(ctx context.Context, exec bob.Executor, TenantIDPK string, ProviderPK int32, ResourceTypePK string, ResourceIDPK string, QuotaNamePK string) (bool, error) {
	return QuotaSnapshots.Query(
		sm.Where(QuotaSnapshots.Columns.TenantID.EQ(psql.Arg(TenantIDPK))),
		sm.Where(QuotaSnapshots.Columns.Provider.EQ(psql.Arg(ProviderPK))),
		sm.Where(QuotaSnapshots.Columns.ResourceType.EQ(psql.Arg(ResourceTypePK))),
		sm.Where(QuotaSnapshots.Columns.ResourceID.EQ(psql.Arg(ResourceIDPK))),
		sm.Where(QuotaSnapshots.Columns.QuotaName.EQ(psql.Arg(QuotaNamePK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after QuotaSnapshot is retrieved from the database
func (o *QuotaSnapshot) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = QuotaSnapshots.AfterSelectHooks.RunHooks(ctx, exec, QuotaSnapshotSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = QuotaSnapshots.AfterInsertHooks.RunHooks(ctx, exec, QuotaSnapshotSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = QuotaSnapshots.AfterUpdateHooks.RunHooks(ctx, exec, QuotaSnapshotSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = QuotaSnapshots.AfterDeleteHooks.RunHooks(ctx, exec, QuotaSnapshotSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = QuotaSnapshots.AfterMergeHooks.RunHooks(ctx, exec, QuotaSnapshotSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the QuotaSnapshot
func (o *QuotaSnapshot) primaryKeyVals() bob.Expression {
	return psql.ArgGroup(
		o.TenantID,
		o.Provider,
		o.ResourceType,
		o.ResourceID,
		o.QuotaName,
	)
}

func (o *QuotaSnapshot) pkEQ() dialect.Expression {
	return psql.Group(psql.Quote("quota_snapshots", "tenant_id"), psql.Quote("quota_snapshots", "provider"), psql.Quote("quota_snapshots", "resource_type"), psql.Quote("quota_snapshots", "resource_id"), psql.Quote("quota_snapshots", "quota_name")).EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the QuotaSnapshot
func (o *QuotaSnapshot) Update(ctx context.Context, exec bob.Executor, s *QuotaSnapshotSetter) error {
	v, err := QuotaSnapshots.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single QuotaSnapshot record with an executor
func (o *QuotaSnapshot) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := QuotaSnapshots.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the QuotaSnapshot using the executor
func (o *QuotaSnapshot) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := QuotaSnapshots.Query(
		sm.Where(QuotaSnapshots.Columns.TenantID.EQ(psql.Arg(o.TenantID))),
		sm.Where(QuotaSnapshots.Columns.Provider.EQ(psql.Arg(o.Provider))),
		sm.Where(QuotaSnapshots.Columns.ResourceType.EQ(psql.Arg(o.ResourceType))),
		sm.Where(QuotaSnapshots.Columns.ResourceID.EQ(psql.Arg(o.ResourceID))),
		sm.Where(QuotaSnapshots.Columns.QuotaName.EQ(psql.Arg(o.QuotaName))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after QuotaSnapshotSlice is retrieved from the database
func (o QuotaSnapshotSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = QuotaSnapshots.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = QuotaSnapshots.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = QuotaSnapshots.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = QuotaSnapshots.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = QuotaSnapshots.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o QuotaSnapshotSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Group(psql.Quote("quota_snapshots", "tenant_id"), psql.Quote("quota_snapshots", "provider"), psql.Quote("quota_snapshots", "resource_type"), psql.Quote("quota_snapshots", "resource_id"), psql.Quote("quota_snapshots", "quota_name")).In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		pkPairs := make([]bob.Expression, len(o))
		for i, row := range o {
			pkPairs[i] = row.primaryKeyVals()
		}
		return bob.ExpressSlice(ctx, w, d, start, pkPairs, "", ", ", "")
	}))
}

// copyMatchingRows finds models in the given slice that have the same primary key
// then it first copies the existing relationships from the old model to the new model
// and then replaces the old model in the slice with the new model
func (o QuotaSnapshotSlice) copyMatchingRows(from ...*QuotaSnapshot) {
	for i, old := range o {
		for _, new := range from {
			if new.TenantID != old.TenantID {
				continue
			}
			if new.Provider != old.Provider {
				continue
			}
			if new.ResourceType != old.ResourceType {
				continue
			}
			if new.ResourceID != old.ResourceID {
				continue
			}
			if new.QuotaName != old.QuotaName {
				continue
			}

			o[i] = new
			break
		}
	}
}

// UpdateMod modifies an update query with "WHERE primary_key IN (o...)"
func (o QuotaSnapshotSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return QuotaSnapshots.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *QuotaSnapshot:
				o.copyMatchingRows(retrieved)
			case []*QuotaSnapshot:
				o.copyMatchingRows(retrieved...)
			case QuotaSnapshotSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a QuotaSnapshot or a slice of QuotaSnapshot
				// then run the AfterUpdateHooks on the slice
				_, err = QuotaSnapshots.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o QuotaSnapshotSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return QuotaSnapshots.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *QuotaSnapshot:
				o.copyMatchingRows(retrieved)
			case []*QuotaSnapshot:
				o.copyMatchingRows(retrieved...)
			case QuotaSnapshotSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a QuotaSnapshot or a slice of QuotaSnapshot
				// then run the AfterDeleteHooks on the slice
				_, err = QuotaSnapshots.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o QuotaSnapshotSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return QuotaSnapshots.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *QuotaSnapshot:
				o.copyMatchingRows(retrieved)
			case []*QuotaSnapshot:
				o.copyMatchingRows(retrieved...)
			case QuotaSnapshotSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a QuotaSnapshot or a slice of QuotaSnapshot
				// then run the AfterMergeHooks on the slice
				_, err = QuotaSnapshots.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o QuotaSnapshotSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals QuotaSnapshotSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := QuotaSnapshots.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o QuotaSnapshotSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := QuotaSnapshots.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o QuotaSnapshotSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := QuotaSnapshots.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type quotaSnapshotWhere[Q psql.Filterable] struct {
	TenantID          psql.WhereMod[Q, string]
	Provider          psql.WhereMod[Q, int32]
	ResourceType      psql.WhereMod[Q, string]
	ResourceID        psql.WhereMod[Q, string]
	Service           psql.WhereMod[Q, string]
	QuotaName         psql.WhereMod[Q, string]
	Units             psql.WhereMod[Q, string]
	ProviderUsed      psql.WhereMod[Q, string]
	QuotaLimit        psql.WhereMod[Q, string]
	ProviderAvailable psql.WhereMod[Q, string]
	ObservedAt        psql.WhereMod[Q, time.Time]
	StaleAfter        psql.WhereMod[Q, time.Time]
	Raw               psql.WhereMod[Q, json.RawMessage]
}

func (quotaSnapshotWhere[Q]) AliasedAs(alias string) quotaSnapshotWhere[Q] {
	return buildQuotaSnapshotWhere[Q](buildQuotaSnapshotColumns(alias))
}

func buildQuotaSnapshotWhere[Q psql.Filterable](cols quotaSnapshotColumns) quotaSnapshotWhere[Q] {
	return quotaSnapshotWhere[Q]{
		TenantID:          psql.Where[Q, string](cols.TenantID.Expression),
		Provider:          psql.Where[Q, int32](cols.Provider.Expression),
		ResourceType:      psql.Where[Q, string](cols.ResourceType.Expression),
		ResourceID:        psql.Where[Q, string](cols.ResourceID.Expression),
		Service:           psql.Where[Q, string](cols.Service.Expression),
		QuotaName:         psql.Where[Q, string](cols.QuotaName.Expression),
		Units:             psql.Where[Q, string](cols.Units.Expression),
		ProviderUsed:      psql.Where[Q, string](cols.ProviderUsed.Expression),
		QuotaLimit:        psql.Where[Q, string](cols.QuotaLimit.Expression),
		ProviderAvailable: psql.Where[Q, string](cols.ProviderAvailable.Expression),
		ObservedAt:        psql.Where[Q, time.Time](cols.ObservedAt.Expression),
		StaleAfter:        psql.Where[Q, time.Time](cols.StaleAfter.Expression),
		Raw:               psql.Where[Q, json.RawMessage](cols.Raw.Expression),
	}
}
