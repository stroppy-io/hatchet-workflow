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

// TenantSettingsRecord is an object representing the database table.
type TenantSettingsRecord struct {
	TenantID  string          `db:"tenant_id,pk" `
	UpdatedAt time.Time       `db:"updated_at" `
	Data      json.RawMessage `db:"data" `
}

// TenantSettingsRecordSlice is an alias for a slice of pointers to TenantSettingsRecord.
// This should almost always be used instead of []*TenantSettingsRecord.
type TenantSettingsRecordSlice []*TenantSettingsRecord

// TenantSettingsRecords contains methods to work with the tenant_settings_records table
var TenantSettingsRecords = psql.NewTablex[*TenantSettingsRecord, TenantSettingsRecordSlice, *TenantSettingsRecordSetter]("", "tenant_settings_records", buildTenantSettingsRecordColumns("tenant_settings_records"))

// TenantSettingsRecordsQuery is a query on the tenant_settings_records table
type TenantSettingsRecordsQuery = *psql.ViewQuery[*TenantSettingsRecord, TenantSettingsRecordSlice]

func buildTenantSettingsRecordColumns(tableName string) tenantSettingsRecordColumns {
	columnsExpr := expr.NewColumnsExpr(
		"tenant_id", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return tenantSettingsRecordColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		TenantID:    buildTenantSettingsRecordColumn(tableName, "tenant_id"),
		UpdatedAt:   buildTenantSettingsRecordColumn(tableName, "updated_at"),
		Data:        buildTenantSettingsRecordColumn(tableName, "data"),
	}
}

type tenantSettingsRecordColumns struct {
	expr.ColumnsExpr
	tableAlias string
	TenantID   tenantSettingsRecordColumn
	UpdatedAt  tenantSettingsRecordColumn
	Data       tenantSettingsRecordColumn
}

// Alias returns the current table alias for the columns set.
func (c tenantSettingsRecordColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (tenantSettingsRecordColumns) AliasedAs(tableName string) tenantSettingsRecordColumns {
	return buildTenantSettingsRecordColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c tenantSettingsRecordColumns) Unqualified() tenantSettingsRecordColumns {
	return buildTenantSettingsRecordColumns("")
}

func buildTenantSettingsRecordColumn(alias, name string) tenantSettingsRecordColumn {
	return tenantSettingsRecordColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type tenantSettingsRecordColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c tenantSettingsRecordColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c tenantSettingsRecordColumn) ShouldOmitParens() bool {
	return true
}

// TenantSettingsRecordSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type TenantSettingsRecordSetter struct {
	TenantID  *string          `db:"tenant_id,pk" `
	UpdatedAt *time.Time       `db:"updated_at" `
	Data      *json.RawMessage `db:"data" `
}

func (s TenantSettingsRecordSetter) SetColumns() []string {
	vals := make([]string, 0, 3)
	if s.TenantID != nil {
		vals = append(vals, "tenant_id")
	}
	if s.UpdatedAt != nil {
		vals = append(vals, "updated_at")
	}
	if s.Data != nil {
		vals = append(vals, "data")
	}
	return vals
}

func (s TenantSettingsRecordSetter) Overwrite(t *TenantSettingsRecord) {
	if s.TenantID != nil {
		t.TenantID = func() string {
			if s.TenantID == nil {
				return *new(string)
			}
			return *s.TenantID
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
	if s.Data != nil {
		t.Data = func() json.RawMessage {
			if s.Data == nil {
				return *new(json.RawMessage)
			}
			return *s.Data
		}()
	}
}

func (s *TenantSettingsRecordSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return TenantSettingsRecords.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 3)
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

		if s.UpdatedAt != nil {
			vals[1] = psql.Arg(func() time.Time {
				if s.UpdatedAt == nil {
					return *new(time.Time)
				}
				return *s.UpdatedAt
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.Data != nil {
			vals[2] = psql.Arg(func() json.RawMessage {
				if s.Data == nil {
					return *new(json.RawMessage)
				}
				return *s.Data
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s TenantSettingsRecordSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s TenantSettingsRecordSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 3)

	if s.TenantID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "tenant_id")...),
			psql.Arg(s.TenantID),
		}})
	}

	if s.UpdatedAt != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "updated_at")...),
			psql.Arg(s.UpdatedAt),
		}})
	}

	if s.Data != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "data")...),
			psql.Arg(s.Data),
		}})
	}

	return exprs
}

// FindTenantSettingsRecord retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindTenantSettingsRecord(ctx context.Context, exec bob.Executor, TenantIDPK string, cols ...string) (*TenantSettingsRecord, error) {
	if len(cols) == 0 {
		return TenantSettingsRecords.Query(
			sm.Where(TenantSettingsRecords.Columns.TenantID.EQ(psql.Arg(TenantIDPK))),
		).One(ctx, exec)
	}

	return TenantSettingsRecords.Query(
		sm.Where(TenantSettingsRecords.Columns.TenantID.EQ(psql.Arg(TenantIDPK))),
		sm.Columns(TenantSettingsRecords.Columns.Only(cols...)),
	).One(ctx, exec)
}

// TenantSettingsRecordExists checks the presence of a single record by primary key
func TenantSettingsRecordExists(ctx context.Context, exec bob.Executor, TenantIDPK string) (bool, error) {
	return TenantSettingsRecords.Query(
		sm.Where(TenantSettingsRecords.Columns.TenantID.EQ(psql.Arg(TenantIDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after TenantSettingsRecord is retrieved from the database
func (o *TenantSettingsRecord) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = TenantSettingsRecords.AfterSelectHooks.RunHooks(ctx, exec, TenantSettingsRecordSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = TenantSettingsRecords.AfterInsertHooks.RunHooks(ctx, exec, TenantSettingsRecordSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = TenantSettingsRecords.AfterUpdateHooks.RunHooks(ctx, exec, TenantSettingsRecordSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = TenantSettingsRecords.AfterDeleteHooks.RunHooks(ctx, exec, TenantSettingsRecordSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = TenantSettingsRecords.AfterMergeHooks.RunHooks(ctx, exec, TenantSettingsRecordSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the TenantSettingsRecord
func (o *TenantSettingsRecord) primaryKeyVals() bob.Expression {
	return psql.Arg(o.TenantID)
}

func (o *TenantSettingsRecord) pkEQ() dialect.Expression {
	return psql.Quote("tenant_settings_records", "tenant_id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the TenantSettingsRecord
func (o *TenantSettingsRecord) Update(ctx context.Context, exec bob.Executor, s *TenantSettingsRecordSetter) error {
	v, err := TenantSettingsRecords.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single TenantSettingsRecord record with an executor
func (o *TenantSettingsRecord) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := TenantSettingsRecords.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the TenantSettingsRecord using the executor
func (o *TenantSettingsRecord) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := TenantSettingsRecords.Query(
		sm.Where(TenantSettingsRecords.Columns.TenantID.EQ(psql.Arg(o.TenantID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after TenantSettingsRecordSlice is retrieved from the database
func (o TenantSettingsRecordSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = TenantSettingsRecords.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = TenantSettingsRecords.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = TenantSettingsRecords.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = TenantSettingsRecords.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = TenantSettingsRecords.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o TenantSettingsRecordSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("tenant_settings_records", "tenant_id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o TenantSettingsRecordSlice) copyMatchingRows(from ...*TenantSettingsRecord) {
	for i, old := range o {
		for _, new := range from {
			if new.TenantID != old.TenantID {
				continue
			}

			o[i] = new
			break
		}
	}
}

// UpdateMod modifies an update query with "WHERE primary_key IN (o...)"
func (o TenantSettingsRecordSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return TenantSettingsRecords.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *TenantSettingsRecord:
				o.copyMatchingRows(retrieved)
			case []*TenantSettingsRecord:
				o.copyMatchingRows(retrieved...)
			case TenantSettingsRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a TenantSettingsRecord or a slice of TenantSettingsRecord
				// then run the AfterUpdateHooks on the slice
				_, err = TenantSettingsRecords.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o TenantSettingsRecordSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return TenantSettingsRecords.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *TenantSettingsRecord:
				o.copyMatchingRows(retrieved)
			case []*TenantSettingsRecord:
				o.copyMatchingRows(retrieved...)
			case TenantSettingsRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a TenantSettingsRecord or a slice of TenantSettingsRecord
				// then run the AfterDeleteHooks on the slice
				_, err = TenantSettingsRecords.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o TenantSettingsRecordSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return TenantSettingsRecords.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *TenantSettingsRecord:
				o.copyMatchingRows(retrieved)
			case []*TenantSettingsRecord:
				o.copyMatchingRows(retrieved...)
			case TenantSettingsRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a TenantSettingsRecord or a slice of TenantSettingsRecord
				// then run the AfterMergeHooks on the slice
				_, err = TenantSettingsRecords.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o TenantSettingsRecordSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals TenantSettingsRecordSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := TenantSettingsRecords.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o TenantSettingsRecordSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := TenantSettingsRecords.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o TenantSettingsRecordSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := TenantSettingsRecords.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type tenantSettingsRecordWhere[Q psql.Filterable] struct {
	TenantID  psql.WhereMod[Q, string]
	UpdatedAt psql.WhereMod[Q, time.Time]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (tenantSettingsRecordWhere[Q]) AliasedAs(alias string) tenantSettingsRecordWhere[Q] {
	return buildTenantSettingsRecordWhere[Q](buildTenantSettingsRecordColumns(alias))
}

func buildTenantSettingsRecordWhere[Q psql.Filterable](cols tenantSettingsRecordColumns) tenantSettingsRecordWhere[Q] {
	return tenantSettingsRecordWhere[Q]{
		TenantID:  psql.Where[Q, string](cols.TenantID.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
