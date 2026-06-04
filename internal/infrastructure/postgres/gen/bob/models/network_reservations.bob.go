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

// NetworkReservation is an object representing the database table.
type NetworkReservation struct {
	ID           string              `db:"id,pk" `
	TenantID     string              `db:"tenant_id" `
	RunID        string              `db:"run_id" `
	Provider     int32               `db:"provider" `
	ResourceType string              `db:"resource_type" `
	ResourceID   string              `db:"resource_id" `
	Cidr         string              `db:"cidr" `
	Status       int32               `db:"status" `
	WorkflowID   string              `db:"workflow_id" `
	ExpiresAt    null.Val[time.Time] `db:"expires_at" `
	CreatedAt    time.Time           `db:"created_at" `
	UpdatedAt    time.Time           `db:"updated_at" `
}

// NetworkReservationSlice is an alias for a slice of pointers to NetworkReservation.
// This should almost always be used instead of []*NetworkReservation.
type NetworkReservationSlice []*NetworkReservation

// NetworkReservations contains methods to work with the network_reservations table
var NetworkReservations = psql.NewTablex[*NetworkReservation, NetworkReservationSlice, *NetworkReservationSetter]("", "network_reservations", buildNetworkReservationColumns("network_reservations"))

// NetworkReservationsQuery is a query on the network_reservations table
type NetworkReservationsQuery = *psql.ViewQuery[*NetworkReservation, NetworkReservationSlice]

func buildNetworkReservationColumns(tableName string) networkReservationColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "tenant_id", "run_id", "provider", "resource_type", "resource_id", "cidr", "status", "workflow_id", "expires_at", "created_at", "updated_at",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return networkReservationColumns{
		ColumnsExpr:  columnsExpr,
		tableAlias:   tableName,
		ID:           buildNetworkReservationColumn(tableName, "id"),
		TenantID:     buildNetworkReservationColumn(tableName, "tenant_id"),
		RunID:        buildNetworkReservationColumn(tableName, "run_id"),
		Provider:     buildNetworkReservationColumn(tableName, "provider"),
		ResourceType: buildNetworkReservationColumn(tableName, "resource_type"),
		ResourceID:   buildNetworkReservationColumn(tableName, "resource_id"),
		Cidr:         buildNetworkReservationColumn(tableName, "cidr"),
		Status:       buildNetworkReservationColumn(tableName, "status"),
		WorkflowID:   buildNetworkReservationColumn(tableName, "workflow_id"),
		ExpiresAt:    buildNetworkReservationColumn(tableName, "expires_at"),
		CreatedAt:    buildNetworkReservationColumn(tableName, "created_at"),
		UpdatedAt:    buildNetworkReservationColumn(tableName, "updated_at"),
	}
}

type networkReservationColumns struct {
	expr.ColumnsExpr
	tableAlias   string
	ID           networkReservationColumn
	TenantID     networkReservationColumn
	RunID        networkReservationColumn
	Provider     networkReservationColumn
	ResourceType networkReservationColumn
	ResourceID   networkReservationColumn
	Cidr         networkReservationColumn
	Status       networkReservationColumn
	WorkflowID   networkReservationColumn
	ExpiresAt    networkReservationColumn
	CreatedAt    networkReservationColumn
	UpdatedAt    networkReservationColumn
}

// Alias returns the current table alias for the columns set.
func (c networkReservationColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (networkReservationColumns) AliasedAs(tableName string) networkReservationColumns {
	return buildNetworkReservationColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c networkReservationColumns) Unqualified() networkReservationColumns {
	return buildNetworkReservationColumns("")
}

func buildNetworkReservationColumn(alias, name string) networkReservationColumn {
	return networkReservationColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type networkReservationColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c networkReservationColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c networkReservationColumn) ShouldOmitParens() bool {
	return true
}

// NetworkReservationSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type NetworkReservationSetter struct {
	ID           *string              `db:"id,pk" `
	TenantID     *string              `db:"tenant_id" `
	RunID        *string              `db:"run_id" `
	Provider     *int32               `db:"provider" `
	ResourceType *string              `db:"resource_type" `
	ResourceID   *string              `db:"resource_id" `
	Cidr         *string              `db:"cidr" `
	Status       *int32               `db:"status" `
	WorkflowID   *string              `db:"workflow_id" `
	ExpiresAt    *null.Val[time.Time] `db:"expires_at" `
	CreatedAt    *time.Time           `db:"created_at" `
	UpdatedAt    *time.Time           `db:"updated_at" `
}

func (s NetworkReservationSetter) SetColumns() []string {
	vals := make([]string, 0, 12)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.TenantID != nil {
		vals = append(vals, "tenant_id")
	}
	if s.RunID != nil {
		vals = append(vals, "run_id")
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
	if s.Cidr != nil {
		vals = append(vals, "cidr")
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

func (s NetworkReservationSetter) Overwrite(t *NetworkReservation) {
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
	if s.Cidr != nil {
		t.Cidr = func() string {
			if s.Cidr == nil {
				return *new(string)
			}
			return *s.Cidr
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

func (s *NetworkReservationSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return NetworkReservations.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 12)
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

		if s.Provider != nil {
			vals[3] = psql.Arg(func() int32 {
				if s.Provider == nil {
					return *new(int32)
				}
				return *s.Provider
			}())
		} else {
			vals[3] = psql.Raw("DEFAULT")
		}

		if s.ResourceType != nil {
			vals[4] = psql.Arg(func() string {
				if s.ResourceType == nil {
					return *new(string)
				}
				return *s.ResourceType
			}())
		} else {
			vals[4] = psql.Raw("DEFAULT")
		}

		if s.ResourceID != nil {
			vals[5] = psql.Arg(func() string {
				if s.ResourceID == nil {
					return *new(string)
				}
				return *s.ResourceID
			}())
		} else {
			vals[5] = psql.Raw("DEFAULT")
		}

		if s.Cidr != nil {
			vals[6] = psql.Arg(func() string {
				if s.Cidr == nil {
					return *new(string)
				}
				return *s.Cidr
			}())
		} else {
			vals[6] = psql.Raw("DEFAULT")
		}

		if s.Status != nil {
			vals[7] = psql.Arg(func() int32 {
				if s.Status == nil {
					return *new(int32)
				}
				return *s.Status
			}())
		} else {
			vals[7] = psql.Raw("DEFAULT")
		}

		if s.WorkflowID != nil {
			vals[8] = psql.Arg(func() string {
				if s.WorkflowID == nil {
					return *new(string)
				}
				return *s.WorkflowID
			}())
		} else {
			vals[8] = psql.Raw("DEFAULT")
		}

		if s.ExpiresAt != nil {
			vals[9] = psql.Arg(func() null.Val[time.Time] {
				if s.ExpiresAt == nil {
					return *new(null.Val[time.Time])
				}
				v := s.ExpiresAt
				return *v
			}())
		} else {
			vals[9] = psql.Raw("DEFAULT")
		}

		if s.CreatedAt != nil {
			vals[10] = psql.Arg(func() time.Time {
				if s.CreatedAt == nil {
					return *new(time.Time)
				}
				return *s.CreatedAt
			}())
		} else {
			vals[10] = psql.Raw("DEFAULT")
		}

		if s.UpdatedAt != nil {
			vals[11] = psql.Arg(func() time.Time {
				if s.UpdatedAt == nil {
					return *new(time.Time)
				}
				return *s.UpdatedAt
			}())
		} else {
			vals[11] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s NetworkReservationSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s NetworkReservationSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 12)

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

	if s.Cidr != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "cidr")...),
			psql.Arg(s.Cidr),
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

// FindNetworkReservation retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindNetworkReservation(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*NetworkReservation, error) {
	if len(cols) == 0 {
		return NetworkReservations.Query(
			sm.Where(NetworkReservations.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return NetworkReservations.Query(
		sm.Where(NetworkReservations.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(NetworkReservations.Columns.Only(cols...)),
	).One(ctx, exec)
}

// NetworkReservationExists checks the presence of a single record by primary key
func NetworkReservationExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return NetworkReservations.Query(
		sm.Where(NetworkReservations.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after NetworkReservation is retrieved from the database
func (o *NetworkReservation) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = NetworkReservations.AfterSelectHooks.RunHooks(ctx, exec, NetworkReservationSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = NetworkReservations.AfterInsertHooks.RunHooks(ctx, exec, NetworkReservationSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = NetworkReservations.AfterUpdateHooks.RunHooks(ctx, exec, NetworkReservationSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = NetworkReservations.AfterDeleteHooks.RunHooks(ctx, exec, NetworkReservationSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = NetworkReservations.AfterMergeHooks.RunHooks(ctx, exec, NetworkReservationSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the NetworkReservation
func (o *NetworkReservation) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *NetworkReservation) pkEQ() dialect.Expression {
	return psql.Quote("network_reservations", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the NetworkReservation
func (o *NetworkReservation) Update(ctx context.Context, exec bob.Executor, s *NetworkReservationSetter) error {
	v, err := NetworkReservations.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single NetworkReservation record with an executor
func (o *NetworkReservation) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := NetworkReservations.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the NetworkReservation using the executor
func (o *NetworkReservation) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := NetworkReservations.Query(
		sm.Where(NetworkReservations.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after NetworkReservationSlice is retrieved from the database
func (o NetworkReservationSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = NetworkReservations.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = NetworkReservations.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = NetworkReservations.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = NetworkReservations.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = NetworkReservations.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o NetworkReservationSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("network_reservations", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o NetworkReservationSlice) copyMatchingRows(from ...*NetworkReservation) {
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
func (o NetworkReservationSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return NetworkReservations.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *NetworkReservation:
				o.copyMatchingRows(retrieved)
			case []*NetworkReservation:
				o.copyMatchingRows(retrieved...)
			case NetworkReservationSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a NetworkReservation or a slice of NetworkReservation
				// then run the AfterUpdateHooks on the slice
				_, err = NetworkReservations.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o NetworkReservationSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return NetworkReservations.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *NetworkReservation:
				o.copyMatchingRows(retrieved)
			case []*NetworkReservation:
				o.copyMatchingRows(retrieved...)
			case NetworkReservationSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a NetworkReservation or a slice of NetworkReservation
				// then run the AfterDeleteHooks on the slice
				_, err = NetworkReservations.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o NetworkReservationSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return NetworkReservations.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *NetworkReservation:
				o.copyMatchingRows(retrieved)
			case []*NetworkReservation:
				o.copyMatchingRows(retrieved...)
			case NetworkReservationSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a NetworkReservation or a slice of NetworkReservation
				// then run the AfterMergeHooks on the slice
				_, err = NetworkReservations.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o NetworkReservationSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals NetworkReservationSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := NetworkReservations.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o NetworkReservationSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := NetworkReservations.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o NetworkReservationSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := NetworkReservations.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type networkReservationWhere[Q psql.Filterable] struct {
	ID           psql.WhereMod[Q, string]
	TenantID     psql.WhereMod[Q, string]
	RunID        psql.WhereMod[Q, string]
	Provider     psql.WhereMod[Q, int32]
	ResourceType psql.WhereMod[Q, string]
	ResourceID   psql.WhereMod[Q, string]
	Cidr         psql.WhereMod[Q, string]
	Status       psql.WhereMod[Q, int32]
	WorkflowID   psql.WhereMod[Q, string]
	ExpiresAt    psql.WhereNullMod[Q, time.Time]
	CreatedAt    psql.WhereMod[Q, time.Time]
	UpdatedAt    psql.WhereMod[Q, time.Time]
}

func (networkReservationWhere[Q]) AliasedAs(alias string) networkReservationWhere[Q] {
	return buildNetworkReservationWhere[Q](buildNetworkReservationColumns(alias))
}

func buildNetworkReservationWhere[Q psql.Filterable](cols networkReservationColumns) networkReservationWhere[Q] {
	return networkReservationWhere[Q]{
		ID:           psql.Where[Q, string](cols.ID.Expression),
		TenantID:     psql.Where[Q, string](cols.TenantID.Expression),
		RunID:        psql.Where[Q, string](cols.RunID.Expression),
		Provider:     psql.Where[Q, int32](cols.Provider.Expression),
		ResourceType: psql.Where[Q, string](cols.ResourceType.Expression),
		ResourceID:   psql.Where[Q, string](cols.ResourceID.Expression),
		Cidr:         psql.Where[Q, string](cols.Cidr.Expression),
		Status:       psql.Where[Q, int32](cols.Status.Expression),
		WorkflowID:   psql.Where[Q, string](cols.WorkflowID.Expression),
		ExpiresAt:    psql.WhereNull[Q, time.Time](cols.ExpiresAt.Expression),
		CreatedAt:    psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt:    psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
	}
}
