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

// WorkloadPresetRecord is an object representing the database table.
type WorkloadPresetRecord struct {
	ID        string          `db:"id,pk" `
	TenantID  string          `db:"tenant_id" `
	CreatedAt time.Time       `db:"created_at" `
	UpdatedAt time.Time       `db:"updated_at" `
	Data      json.RawMessage `db:"data" `
}

// WorkloadPresetRecordSlice is an alias for a slice of pointers to WorkloadPresetRecord.
// This should almost always be used instead of []*WorkloadPresetRecord.
type WorkloadPresetRecordSlice []*WorkloadPresetRecord

// WorkloadPresetRecords contains methods to work with the workload_preset_records table
var WorkloadPresetRecords = psql.NewTablex[*WorkloadPresetRecord, WorkloadPresetRecordSlice, *WorkloadPresetRecordSetter]("", "workload_preset_records", buildWorkloadPresetRecordColumns("workload_preset_records"))

// WorkloadPresetRecordsQuery is a query on the workload_preset_records table
type WorkloadPresetRecordsQuery = *psql.ViewQuery[*WorkloadPresetRecord, WorkloadPresetRecordSlice]

func buildWorkloadPresetRecordColumns(tableName string) workloadPresetRecordColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "tenant_id", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return workloadPresetRecordColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildWorkloadPresetRecordColumn(tableName, "id"),
		TenantID:    buildWorkloadPresetRecordColumn(tableName, "tenant_id"),
		CreatedAt:   buildWorkloadPresetRecordColumn(tableName, "created_at"),
		UpdatedAt:   buildWorkloadPresetRecordColumn(tableName, "updated_at"),
		Data:        buildWorkloadPresetRecordColumn(tableName, "data"),
	}
}

type workloadPresetRecordColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         workloadPresetRecordColumn
	TenantID   workloadPresetRecordColumn
	CreatedAt  workloadPresetRecordColumn
	UpdatedAt  workloadPresetRecordColumn
	Data       workloadPresetRecordColumn
}

// Alias returns the current table alias for the columns set.
func (c workloadPresetRecordColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (workloadPresetRecordColumns) AliasedAs(tableName string) workloadPresetRecordColumns {
	return buildWorkloadPresetRecordColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c workloadPresetRecordColumns) Unqualified() workloadPresetRecordColumns {
	return buildWorkloadPresetRecordColumns("")
}

func buildWorkloadPresetRecordColumn(alias, name string) workloadPresetRecordColumn {
	return workloadPresetRecordColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type workloadPresetRecordColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c workloadPresetRecordColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c workloadPresetRecordColumn) ShouldOmitParens() bool {
	return true
}

// WorkloadPresetRecordSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type WorkloadPresetRecordSetter struct {
	ID        *string          `db:"id,pk" `
	TenantID  *string          `db:"tenant_id" `
	CreatedAt *time.Time       `db:"created_at" `
	UpdatedAt *time.Time       `db:"updated_at" `
	Data      *json.RawMessage `db:"data" `
}

func (s WorkloadPresetRecordSetter) SetColumns() []string {
	vals := make([]string, 0, 5)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.TenantID != nil {
		vals = append(vals, "tenant_id")
	}
	if s.CreatedAt != nil {
		vals = append(vals, "created_at")
	}
	if s.UpdatedAt != nil {
		vals = append(vals, "updated_at")
	}
	if s.Data != nil {
		vals = append(vals, "data")
	}
	return vals
}

func (s WorkloadPresetRecordSetter) Overwrite(t *WorkloadPresetRecord) {
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
	if s.Data != nil {
		t.Data = func() json.RawMessage {
			if s.Data == nil {
				return *new(json.RawMessage)
			}
			return *s.Data
		}()
	}
}

func (s *WorkloadPresetRecordSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return WorkloadPresetRecords.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 5)
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

		if s.CreatedAt != nil {
			vals[2] = psql.Arg(func() time.Time {
				if s.CreatedAt == nil {
					return *new(time.Time)
				}
				return *s.CreatedAt
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		if s.UpdatedAt != nil {
			vals[3] = psql.Arg(func() time.Time {
				if s.UpdatedAt == nil {
					return *new(time.Time)
				}
				return *s.UpdatedAt
			}())
		} else {
			vals[3] = psql.Raw("DEFAULT")
		}

		if s.Data != nil {
			vals[4] = psql.Arg(func() json.RawMessage {
				if s.Data == nil {
					return *new(json.RawMessage)
				}
				return *s.Data
			}())
		} else {
			vals[4] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s WorkloadPresetRecordSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s WorkloadPresetRecordSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 5)

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

	if s.Data != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "data")...),
			psql.Arg(s.Data),
		}})
	}

	return exprs
}

// FindWorkloadPresetRecord retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindWorkloadPresetRecord(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*WorkloadPresetRecord, error) {
	if len(cols) == 0 {
		return WorkloadPresetRecords.Query(
			sm.Where(WorkloadPresetRecords.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return WorkloadPresetRecords.Query(
		sm.Where(WorkloadPresetRecords.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(WorkloadPresetRecords.Columns.Only(cols...)),
	).One(ctx, exec)
}

// WorkloadPresetRecordExists checks the presence of a single record by primary key
func WorkloadPresetRecordExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return WorkloadPresetRecords.Query(
		sm.Where(WorkloadPresetRecords.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after WorkloadPresetRecord is retrieved from the database
func (o *WorkloadPresetRecord) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = WorkloadPresetRecords.AfterSelectHooks.RunHooks(ctx, exec, WorkloadPresetRecordSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = WorkloadPresetRecords.AfterInsertHooks.RunHooks(ctx, exec, WorkloadPresetRecordSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = WorkloadPresetRecords.AfterUpdateHooks.RunHooks(ctx, exec, WorkloadPresetRecordSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = WorkloadPresetRecords.AfterDeleteHooks.RunHooks(ctx, exec, WorkloadPresetRecordSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = WorkloadPresetRecords.AfterMergeHooks.RunHooks(ctx, exec, WorkloadPresetRecordSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the WorkloadPresetRecord
func (o *WorkloadPresetRecord) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *WorkloadPresetRecord) pkEQ() dialect.Expression {
	return psql.Quote("workload_preset_records", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the WorkloadPresetRecord
func (o *WorkloadPresetRecord) Update(ctx context.Context, exec bob.Executor, s *WorkloadPresetRecordSetter) error {
	v, err := WorkloadPresetRecords.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single WorkloadPresetRecord record with an executor
func (o *WorkloadPresetRecord) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := WorkloadPresetRecords.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the WorkloadPresetRecord using the executor
func (o *WorkloadPresetRecord) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := WorkloadPresetRecords.Query(
		sm.Where(WorkloadPresetRecords.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after WorkloadPresetRecordSlice is retrieved from the database
func (o WorkloadPresetRecordSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = WorkloadPresetRecords.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = WorkloadPresetRecords.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = WorkloadPresetRecords.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = WorkloadPresetRecords.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = WorkloadPresetRecords.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o WorkloadPresetRecordSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("workload_preset_records", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o WorkloadPresetRecordSlice) copyMatchingRows(from ...*WorkloadPresetRecord) {
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
func (o WorkloadPresetRecordSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return WorkloadPresetRecords.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *WorkloadPresetRecord:
				o.copyMatchingRows(retrieved)
			case []*WorkloadPresetRecord:
				o.copyMatchingRows(retrieved...)
			case WorkloadPresetRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a WorkloadPresetRecord or a slice of WorkloadPresetRecord
				// then run the AfterUpdateHooks on the slice
				_, err = WorkloadPresetRecords.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o WorkloadPresetRecordSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return WorkloadPresetRecords.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *WorkloadPresetRecord:
				o.copyMatchingRows(retrieved)
			case []*WorkloadPresetRecord:
				o.copyMatchingRows(retrieved...)
			case WorkloadPresetRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a WorkloadPresetRecord or a slice of WorkloadPresetRecord
				// then run the AfterDeleteHooks on the slice
				_, err = WorkloadPresetRecords.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o WorkloadPresetRecordSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return WorkloadPresetRecords.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *WorkloadPresetRecord:
				o.copyMatchingRows(retrieved)
			case []*WorkloadPresetRecord:
				o.copyMatchingRows(retrieved...)
			case WorkloadPresetRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a WorkloadPresetRecord or a slice of WorkloadPresetRecord
				// then run the AfterMergeHooks on the slice
				_, err = WorkloadPresetRecords.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o WorkloadPresetRecordSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals WorkloadPresetRecordSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := WorkloadPresetRecords.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o WorkloadPresetRecordSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := WorkloadPresetRecords.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o WorkloadPresetRecordSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := WorkloadPresetRecords.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type workloadPresetRecordWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	TenantID  psql.WhereMod[Q, string]
	CreatedAt psql.WhereMod[Q, time.Time]
	UpdatedAt psql.WhereMod[Q, time.Time]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (workloadPresetRecordWhere[Q]) AliasedAs(alias string) workloadPresetRecordWhere[Q] {
	return buildWorkloadPresetRecordWhere[Q](buildWorkloadPresetRecordColumns(alias))
}

func buildWorkloadPresetRecordWhere[Q psql.Filterable](cols workloadPresetRecordColumns) workloadPresetRecordWhere[Q] {
	return workloadPresetRecordWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		TenantID:  psql.Where[Q, string](cols.TenantID.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
