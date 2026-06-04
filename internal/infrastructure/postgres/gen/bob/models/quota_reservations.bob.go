// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"io"
	"time"

	"github.com/aarondl/opt/null"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/bob/expr"
)

// QuotaReservation is an object representing the database table.
type QuotaReservation struct {
	ID           string              `db:"id,pk" `
	TenantID     string              `db:"tenant_id" `
	RunID        string              `db:"run_id" `
	NodeID       string              `db:"node_id" `
	Provider     int32               `db:"provider" `
	ResourceType string              `db:"resource_type" `
	ResourceID   string              `db:"resource_id" `
	Service      string              `db:"service" `
	QuotaName    string              `db:"quota_name" `
	Units        string              `db:"units" `
	Amount       string              `db:"amount" `
	Status       int32               `db:"status" `
	WorkflowID   string              `db:"workflow_id" `
	ExpiresAt    null.Val[time.Time] `db:"expires_at" `
	CreatedAt    time.Time           `db:"created_at" `
	UpdatedAt    time.Time           `db:"updated_at" `
}

// QuotaReservationSlice is an alias for a slice of pointers to QuotaReservation.
// This should almost always be used instead of []*QuotaReservation.
type QuotaReservationSlice []*QuotaReservation

// QuotaReservations contains methods to work with the quota_reservations table
var QuotaReservations = psql.NewTablex[*QuotaReservation, QuotaReservationSlice, *QuotaReservationSetter]("", "quota_reservations", buildQuotaReservationColumns("quota_reservations"))

// QuotaReservationsQuery is a query on the quota_reservations table
type QuotaReservationsQuery = *psql.ViewQuery[*QuotaReservation, QuotaReservationSlice]

func buildQuotaReservationColumns(tableName string) quotaReservationColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "tenant_id", "run_id", "node_id", "provider", "resource_type", "resource_id", "service", "quota_name", "units", "amount", "status", "workflow_id", "expires_at", "created_at", "updated_at",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return quotaReservationColumns{
		ColumnsExpr:  columnsExpr,
		tableAlias:   tableName,
		ID:           buildQuotaReservationColumn(tableName, "id"),
		TenantID:     buildQuotaReservationColumn(tableName, "tenant_id"),
		RunID:        buildQuotaReservationColumn(tableName, "run_id"),
		NodeID:       buildQuotaReservationColumn(tableName, "node_id"),
		Provider:     buildQuotaReservationColumn(tableName, "provider"),
		ResourceType: buildQuotaReservationColumn(tableName, "resource_type"),
		ResourceID:   buildQuotaReservationColumn(tableName, "resource_id"),
		Service:      buildQuotaReservationColumn(tableName, "service"),
		QuotaName:    buildQuotaReservationColumn(tableName, "quota_name"),
		Units:        buildQuotaReservationColumn(tableName, "units"),
		Amount:       buildQuotaReservationColumn(tableName, "amount"),
		Status:       buildQuotaReservationColumn(tableName, "status"),
		WorkflowID:   buildQuotaReservationColumn(tableName, "workflow_id"),
		ExpiresAt:    buildQuotaReservationColumn(tableName, "expires_at"),
		CreatedAt:    buildQuotaReservationColumn(tableName, "created_at"),
		UpdatedAt:    buildQuotaReservationColumn(tableName, "updated_at"),
	}
}

type quotaReservationColumns struct {
	expr.ColumnsExpr
	tableAlias   string
	ID           quotaReservationColumn
	TenantID     quotaReservationColumn
	RunID        quotaReservationColumn
	NodeID       quotaReservationColumn
	Provider     quotaReservationColumn
	ResourceType quotaReservationColumn
	ResourceID   quotaReservationColumn
	Service      quotaReservationColumn
	QuotaName    quotaReservationColumn
	Units        quotaReservationColumn
	Amount       quotaReservationColumn
	Status       quotaReservationColumn
	WorkflowID   quotaReservationColumn
	ExpiresAt    quotaReservationColumn
	CreatedAt    quotaReservationColumn
	UpdatedAt    quotaReservationColumn
}

// Alias returns the current table alias for the columns set.
func (c quotaReservationColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (quotaReservationColumns) AliasedAs(tableName string) quotaReservationColumns {
	return buildQuotaReservationColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c quotaReservationColumns) Unqualified() quotaReservationColumns {
	return buildQuotaReservationColumns("")
}

func buildQuotaReservationColumn(alias, name string) quotaReservationColumn {
	return quotaReservationColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type quotaReservationColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c quotaReservationColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c quotaReservationColumn) ShouldOmitParens() bool {
	return true
}

// QuotaReservationSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type QuotaReservationSetter struct {
	ID           *string              `db:"id,pk" `
	TenantID     *string              `db:"tenant_id" `
	RunID        *string              `db:"run_id" `
	NodeID       *string              `db:"node_id" `
	Provider     *int32               `db:"provider" `
	ResourceType *string              `db:"resource_type" `
	ResourceID   *string              `db:"resource_id" `
	Service      *string              `db:"service" `
	QuotaName    *string              `db:"quota_name" `
	Units        *string              `db:"units" `
	Amount       *string              `db:"amount" `
	Status       *int32               `db:"status" `
	WorkflowID   *string              `db:"workflow_id" `
	ExpiresAt    *null.Val[time.Time] `db:"expires_at" `
	CreatedAt    *time.Time           `db:"created_at" `
	UpdatedAt    *time.Time           `db:"updated_at" `
}

func (s QuotaReservationSetter) SetColumns() []string {
	vals := make([]string, 0, 16)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.TenantID != nil {
		vals = append(vals, "tenant_id")
	}
	if s.RunID != nil {
		vals = append(vals, "run_id")
	}
	if s.NodeID != nil {
		vals = append(vals, "node_id")
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
	if s.Amount != nil {
		vals = append(vals, "amount")
	}
	if s.Status != nil {
		vals = append(vals, "status")
	}
	if s.WorkflowID != nil {
		vals = append(vals, "workflow_id")
	}
	if s.ExpiresAt != nil {
		vals = append(vals, "expires_at")
	}
	if s.CreatedAt != nil {
		vals = append(vals, "created_at")
	}
	if s.UpdatedAt != nil {
		vals = append(vals, "updated_at")
	}
	return vals
}

func (s QuotaReservationSetter) Overwrite(t *QuotaReservation) {
	if s.ID != nil {
		t.ID = func() string {
			if s.ID == nil {
				return *new(string)
			}
			return *s.ID
		}()
	}
	if s.TenantID != nil {
		t.TenantID = func() string {
			if s.TenantID == nil {
				return *new(string)
			}
			return *s.TenantID
		}()
	}
	if s.RunID != nil {
		t.RunID = func() string {
			if s.RunID == nil {
				return *new(string)
			}
			return *s.RunID
		}()
	}
	if s.NodeID != nil {
		t.NodeID = func() string {
			if s.NodeID == nil {
				return *new(string)
			}
			return *s.NodeID
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
	if s.Amount != nil {
		t.Amount = func() string {
			if s.Amount == nil {
				return *new(string)
			}
			return *s.Amount
		}()
	}
	if s.Status != nil {
		t.Status = func() int32 {
			if s.Status == nil {
				return *new(int32)
			}
			return *s.Status
		}()
	}
	if s.WorkflowID != nil {
		t.WorkflowID = func() string {
			if s.WorkflowID == nil {
				return *new(string)
			}
			return *s.WorkflowID
		}()
	}
	if s.ExpiresAt != nil {
		t.ExpiresAt = func() null.Val[time.Time] {
			if s.ExpiresAt == nil {
				return *new(null.Val[time.Time])
			}
			v := s.ExpiresAt
			return *v
		}()
	}
	if s.CreatedAt != nil {
		t.CreatedAt = func() time.Time {
			if s.CreatedAt == nil {
				return *new(time.Time)
			}
			return *s.CreatedAt
		}()
	}
	if s.UpdatedAt != nil {
		t.UpdatedAt = func() time.Time {
			if s.UpdatedAt == nil {
				return *new(time.Time)
			}
			return *s.UpdatedAt
		}()
	}
}

func (s *QuotaReservationSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return QuotaReservations.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 16)
		if s.ID != nil {
			vals[0] = psql.Arg(func() string {
				if s.ID == nil {
					return *new(string)
				}
				return *s.ID
			}())
		} else {
			vals[0] = psql.Raw("DEFAULT")
		}

		if s.TenantID != nil {
			vals[1] = psql.Arg(func() string {
				if s.TenantID == nil {
					return *new(string)
				}
				return *s.TenantID
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.RunID != nil {
			vals[2] = psql.Arg(func() string {
				if s.RunID == nil {
					return *new(string)
				}
				return *s.RunID
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		if s.NodeID != nil {
			vals[3] = psql.Arg(func() string {
				if s.NodeID == nil {
					return *new(string)
				}
				return *s.NodeID
			}())
		} else {
			vals[3] = psql.Raw("DEFAULT")
		}

		if s.Provider != nil {
			vals[4] = psql.Arg(func() int32 {
				if s.Provider == nil {
					return *new(int32)
				}
				return *s.Provider
			}())
		} else {
			vals[4] = psql.Raw("DEFAULT")
		}

		if s.ResourceType != nil {
			vals[5] = psql.Arg(func() string {
				if s.ResourceType == nil {
					return *new(string)
				}
				return *s.ResourceType
			}())
		} else {
			vals[5] = psql.Raw("DEFAULT")
		}

		if s.ResourceID != nil {
			vals[6] = psql.Arg(func() string {
				if s.ResourceID == nil {
					return *new(string)
				}
				return *s.ResourceID
			}())
		} else {
			vals[6] = psql.Raw("DEFAULT")
		}

		if s.Service != nil {
			vals[7] = psql.Arg(func() string {
				if s.Service == nil {
					return *new(string)
				}
				return *s.Service
			}())
		} else {
			vals[7] = psql.Raw("DEFAULT")
		}

		if s.QuotaName != nil {
			vals[8] = psql.Arg(func() string {
				if s.QuotaName == nil {
					return *new(string)
				}
				return *s.QuotaName
			}())
		} else {
			vals[8] = psql.Raw("DEFAULT")
		}

		if s.Units != nil {
			vals[9] = psql.Arg(func() string {
				if s.Units == nil {
					return *new(string)
				}
				return *s.Units
			}())
		} else {
			vals[9] = psql.Raw("DEFAULT")
		}

		if s.Amount != nil {
			vals[10] = psql.Arg(func() string {
				if s.Amount == nil {
					return *new(string)
				}
				return *s.Amount
			}())
		} else {
			vals[10] = psql.Raw("DEFAULT")
		}

		if s.Status != nil {
			vals[11] = psql.Arg(func() int32 {
				if s.Status == nil {
					return *new(int32)
				}
				return *s.Status
			}())
		} else {
			vals[11] = psql.Raw("DEFAULT")
		}

		if s.WorkflowID != nil {
			vals[12] = psql.Arg(func() string {
				if s.WorkflowID == nil {
					return *new(string)
				}
				return *s.WorkflowID
			}())
		} else {
			vals[12] = psql.Raw("DEFAULT")
		}

		if s.ExpiresAt != nil {
			vals[13] = psql.Arg(func() null.Val[time.Time] {
				if s.ExpiresAt == nil {
					return *new(null.Val[time.Time])
				}
				v := s.ExpiresAt
				return *v
			}())
		} else {
			vals[13] = psql.Raw("DEFAULT")
		}

		if s.CreatedAt != nil {
			vals[14] = psql.Arg(func() time.Time {
				if s.CreatedAt == nil {
					return *new(time.Time)
				}
				return *s.CreatedAt
			}())
		} else {
			vals[14] = psql.Raw("DEFAULT")
		}

		if s.UpdatedAt != nil {
			vals[15] = psql.Arg(func() time.Time {
				if s.UpdatedAt == nil {
					return *new(time.Time)
				}
				return *s.UpdatedAt
			}())
		} else {
			vals[15] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s QuotaReservationSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s QuotaReservationSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 16)

	if s.ID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "id")...),
			psql.Arg(s.ID),
		}})
	}

	if s.TenantID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "tenant_id")...),
			psql.Arg(s.TenantID),
		}})
	}

	if s.RunID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "run_id")...),
			psql.Arg(s.RunID),
		}})
	}

	if s.NodeID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "node_id")...),
			psql.Arg(s.NodeID),
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

	if s.Amount != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "amount")...),
			psql.Arg(s.Amount),
		}})
	}

	if s.Status != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "status")...),
			psql.Arg(s.Status),
		}})
	}

	if s.WorkflowID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "workflow_id")...),
			psql.Arg(s.WorkflowID),
		}})
	}

	if s.ExpiresAt != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "expires_at")...),
			psql.Arg(s.ExpiresAt),
		}})
	}

	if s.CreatedAt != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "created_at")...),
			psql.Arg(s.CreatedAt),
		}})
	}

	if s.UpdatedAt != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "updated_at")...),
			psql.Arg(s.UpdatedAt),
		}})
	}

	return exprs
}

// FindQuotaReservation retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindQuotaReservation(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*QuotaReservation, error) {
	if len(cols) == 0 {
		return QuotaReservations.Query(
			sm.Where(QuotaReservations.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return QuotaReservations.Query(
		sm.Where(QuotaReservations.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(QuotaReservations.Columns.Only(cols...)),
	).One(ctx, exec)
}

// QuotaReservationExists checks the presence of a single record by primary key
func QuotaReservationExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return QuotaReservations.Query(
		sm.Where(QuotaReservations.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after QuotaReservation is retrieved from the database
func (o *QuotaReservation) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = QuotaReservations.AfterSelectHooks.RunHooks(ctx, exec, QuotaReservationSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = QuotaReservations.AfterInsertHooks.RunHooks(ctx, exec, QuotaReservationSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = QuotaReservations.AfterUpdateHooks.RunHooks(ctx, exec, QuotaReservationSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = QuotaReservations.AfterDeleteHooks.RunHooks(ctx, exec, QuotaReservationSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = QuotaReservations.AfterMergeHooks.RunHooks(ctx, exec, QuotaReservationSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the QuotaReservation
func (o *QuotaReservation) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *QuotaReservation) pkEQ() dialect.Expression {
	return psql.Quote("quota_reservations", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the QuotaReservation
func (o *QuotaReservation) Update(ctx context.Context, exec bob.Executor, s *QuotaReservationSetter) error {
	v, err := QuotaReservations.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single QuotaReservation record with an executor
func (o *QuotaReservation) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := QuotaReservations.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the QuotaReservation using the executor
func (o *QuotaReservation) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := QuotaReservations.Query(
		sm.Where(QuotaReservations.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after QuotaReservationSlice is retrieved from the database
func (o QuotaReservationSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = QuotaReservations.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = QuotaReservations.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = QuotaReservations.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = QuotaReservations.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = QuotaReservations.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o QuotaReservationSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("quota_reservations", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o QuotaReservationSlice) copyMatchingRows(from ...*QuotaReservation) {
	for i, old := range o {
		for _, new := range from {
			if new.ID != old.ID {
				continue
			}

			o[i] = new
			break
		}
	}
}

// UpdateMod modifies an update query with "WHERE primary_key IN (o...)"
func (o QuotaReservationSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return QuotaReservations.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *QuotaReservation:
				o.copyMatchingRows(retrieved)
			case []*QuotaReservation:
				o.copyMatchingRows(retrieved...)
			case QuotaReservationSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a QuotaReservation or a slice of QuotaReservation
				// then run the AfterUpdateHooks on the slice
				_, err = QuotaReservations.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o QuotaReservationSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return QuotaReservations.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *QuotaReservation:
				o.copyMatchingRows(retrieved)
			case []*QuotaReservation:
				o.copyMatchingRows(retrieved...)
			case QuotaReservationSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a QuotaReservation or a slice of QuotaReservation
				// then run the AfterDeleteHooks on the slice
				_, err = QuotaReservations.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o QuotaReservationSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return QuotaReservations.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *QuotaReservation:
				o.copyMatchingRows(retrieved)
			case []*QuotaReservation:
				o.copyMatchingRows(retrieved...)
			case QuotaReservationSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a QuotaReservation or a slice of QuotaReservation
				// then run the AfterMergeHooks on the slice
				_, err = QuotaReservations.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o QuotaReservationSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals QuotaReservationSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := QuotaReservations.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o QuotaReservationSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := QuotaReservations.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o QuotaReservationSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := QuotaReservations.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type quotaReservationWhere[Q psql.Filterable] struct {
	ID           psql.WhereMod[Q, string]
	TenantID     psql.WhereMod[Q, string]
	RunID        psql.WhereMod[Q, string]
	NodeID       psql.WhereMod[Q, string]
	Provider     psql.WhereMod[Q, int32]
	ResourceType psql.WhereMod[Q, string]
	ResourceID   psql.WhereMod[Q, string]
	Service      psql.WhereMod[Q, string]
	QuotaName    psql.WhereMod[Q, string]
	Units        psql.WhereMod[Q, string]
	Amount       psql.WhereMod[Q, string]
	Status       psql.WhereMod[Q, int32]
	WorkflowID   psql.WhereMod[Q, string]
	ExpiresAt    psql.WhereNullMod[Q, time.Time]
	CreatedAt    psql.WhereMod[Q, time.Time]
	UpdatedAt    psql.WhereMod[Q, time.Time]
}

func (quotaReservationWhere[Q]) AliasedAs(alias string) quotaReservationWhere[Q] {
	return buildQuotaReservationWhere[Q](buildQuotaReservationColumns(alias))
}

func buildQuotaReservationWhere[Q psql.Filterable](cols quotaReservationColumns) quotaReservationWhere[Q] {
	return quotaReservationWhere[Q]{
		ID:           psql.Where[Q, string](cols.ID.Expression),
		TenantID:     psql.Where[Q, string](cols.TenantID.Expression),
		RunID:        psql.Where[Q, string](cols.RunID.Expression),
		NodeID:       psql.Where[Q, string](cols.NodeID.Expression),
		Provider:     psql.Where[Q, int32](cols.Provider.Expression),
		ResourceType: psql.Where[Q, string](cols.ResourceType.Expression),
		ResourceID:   psql.Where[Q, string](cols.ResourceID.Expression),
		Service:      psql.Where[Q, string](cols.Service.Expression),
		QuotaName:    psql.Where[Q, string](cols.QuotaName.Expression),
		Units:        psql.Where[Q, string](cols.Units.Expression),
		Amount:       psql.Where[Q, string](cols.Amount.Expression),
		Status:       psql.Where[Q, int32](cols.Status.Expression),
		WorkflowID:   psql.Where[Q, string](cols.WorkflowID.Expression),
		ExpiresAt:    psql.WhereNull[Q, time.Time](cols.ExpiresAt.Expression),
		CreatedAt:    psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt:    psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
	}
}
