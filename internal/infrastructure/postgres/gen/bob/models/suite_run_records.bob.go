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

// SuiteRunRecord is an object representing the database table.
type SuiteRunRecord struct {
	ID        string          `db:"id,pk" `
	TenantID  string          `db:"tenant_id" `
	CreatedAt time.Time       `db:"created_at" `
	UpdatedAt time.Time       `db:"updated_at" `
	Data      json.RawMessage `db:"data" `
}

// SuiteRunRecordSlice is an alias for a slice of pointers to SuiteRunRecord.
// This should almost always be used instead of []*SuiteRunRecord.
type SuiteRunRecordSlice []*SuiteRunRecord

// SuiteRunRecords contains methods to work with the suite_run_records table
var SuiteRunRecords = psql.NewTablex[*SuiteRunRecord, SuiteRunRecordSlice, *SuiteRunRecordSetter]("", "suite_run_records", buildSuiteRunRecordColumns("suite_run_records"))

// SuiteRunRecordsQuery is a query on the suite_run_records table
type SuiteRunRecordsQuery = *psql.ViewQuery[*SuiteRunRecord, SuiteRunRecordSlice]

func buildSuiteRunRecordColumns(tableName string) suiteRunRecordColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "tenant_id", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return suiteRunRecordColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildSuiteRunRecordColumn(tableName, "id"),
		TenantID:    buildSuiteRunRecordColumn(tableName, "tenant_id"),
		CreatedAt:   buildSuiteRunRecordColumn(tableName, "created_at"),
		UpdatedAt:   buildSuiteRunRecordColumn(tableName, "updated_at"),
		Data:        buildSuiteRunRecordColumn(tableName, "data"),
	}
}

type suiteRunRecordColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         suiteRunRecordColumn
	TenantID   suiteRunRecordColumn
	CreatedAt  suiteRunRecordColumn
	UpdatedAt  suiteRunRecordColumn
	Data       suiteRunRecordColumn
}

// Alias returns the current table alias for the columns set.
func (c suiteRunRecordColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (suiteRunRecordColumns) AliasedAs(tableName string) suiteRunRecordColumns {
	return buildSuiteRunRecordColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c suiteRunRecordColumns) Unqualified() suiteRunRecordColumns {
	return buildSuiteRunRecordColumns("")
}

func buildSuiteRunRecordColumn(alias, name string) suiteRunRecordColumn {
	return suiteRunRecordColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type suiteRunRecordColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c suiteRunRecordColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c suiteRunRecordColumn) ShouldOmitParens() bool {
	return true
}

// SuiteRunRecordSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type SuiteRunRecordSetter struct {
	ID        *string          `db:"id,pk" `
	TenantID  *string          `db:"tenant_id" `
	CreatedAt *time.Time       `db:"created_at" `
	UpdatedAt *time.Time       `db:"updated_at" `
	Data      *json.RawMessage `db:"data" `
}

func (s SuiteRunRecordSetter) SetColumns() []string {
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

func (s SuiteRunRecordSetter) Overwrite(t *SuiteRunRecord) {
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

func (s *SuiteRunRecordSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return SuiteRunRecords.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

func (s SuiteRunRecordSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s SuiteRunRecordSetter) Expressions(prefix ...string) []bob.Expression {
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

// FindSuiteRunRecord retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindSuiteRunRecord(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*SuiteRunRecord, error) {
	if len(cols) == 0 {
		return SuiteRunRecords.Query(
			sm.Where(SuiteRunRecords.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return SuiteRunRecords.Query(
		sm.Where(SuiteRunRecords.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(SuiteRunRecords.Columns.Only(cols...)),
	).One(ctx, exec)
}

// SuiteRunRecordExists checks the presence of a single record by primary key
func SuiteRunRecordExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return SuiteRunRecords.Query(
		sm.Where(SuiteRunRecords.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after SuiteRunRecord is retrieved from the database
func (o *SuiteRunRecord) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = SuiteRunRecords.AfterSelectHooks.RunHooks(ctx, exec, SuiteRunRecordSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = SuiteRunRecords.AfterInsertHooks.RunHooks(ctx, exec, SuiteRunRecordSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = SuiteRunRecords.AfterUpdateHooks.RunHooks(ctx, exec, SuiteRunRecordSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = SuiteRunRecords.AfterDeleteHooks.RunHooks(ctx, exec, SuiteRunRecordSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = SuiteRunRecords.AfterMergeHooks.RunHooks(ctx, exec, SuiteRunRecordSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the SuiteRunRecord
func (o *SuiteRunRecord) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *SuiteRunRecord) pkEQ() dialect.Expression {
	return psql.Quote("suite_run_records", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the SuiteRunRecord
func (o *SuiteRunRecord) Update(ctx context.Context, exec bob.Executor, s *SuiteRunRecordSetter) error {
	v, err := SuiteRunRecords.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single SuiteRunRecord record with an executor
func (o *SuiteRunRecord) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := SuiteRunRecords.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the SuiteRunRecord using the executor
func (o *SuiteRunRecord) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := SuiteRunRecords.Query(
		sm.Where(SuiteRunRecords.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after SuiteRunRecordSlice is retrieved from the database
func (o SuiteRunRecordSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = SuiteRunRecords.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = SuiteRunRecords.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = SuiteRunRecords.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = SuiteRunRecords.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = SuiteRunRecords.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o SuiteRunRecordSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("suite_run_records", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o SuiteRunRecordSlice) copyMatchingRows(from ...*SuiteRunRecord) {
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
func (o SuiteRunRecordSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return SuiteRunRecords.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *SuiteRunRecord:
				o.copyMatchingRows(retrieved)
			case []*SuiteRunRecord:
				o.copyMatchingRows(retrieved...)
			case SuiteRunRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a SuiteRunRecord or a slice of SuiteRunRecord
				// then run the AfterUpdateHooks on the slice
				_, err = SuiteRunRecords.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o SuiteRunRecordSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return SuiteRunRecords.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *SuiteRunRecord:
				o.copyMatchingRows(retrieved)
			case []*SuiteRunRecord:
				o.copyMatchingRows(retrieved...)
			case SuiteRunRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a SuiteRunRecord or a slice of SuiteRunRecord
				// then run the AfterDeleteHooks on the slice
				_, err = SuiteRunRecords.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o SuiteRunRecordSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return SuiteRunRecords.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *SuiteRunRecord:
				o.copyMatchingRows(retrieved)
			case []*SuiteRunRecord:
				o.copyMatchingRows(retrieved...)
			case SuiteRunRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a SuiteRunRecord or a slice of SuiteRunRecord
				// then run the AfterMergeHooks on the slice
				_, err = SuiteRunRecords.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o SuiteRunRecordSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals SuiteRunRecordSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := SuiteRunRecords.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o SuiteRunRecordSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := SuiteRunRecords.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o SuiteRunRecordSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := SuiteRunRecords.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type suiteRunRecordWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	TenantID  psql.WhereMod[Q, string]
	CreatedAt psql.WhereMod[Q, time.Time]
	UpdatedAt psql.WhereMod[Q, time.Time]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (suiteRunRecordWhere[Q]) AliasedAs(alias string) suiteRunRecordWhere[Q] {
	return buildSuiteRunRecordWhere[Q](buildSuiteRunRecordColumns(alias))
}

func buildSuiteRunRecordWhere[Q psql.Filterable](cols suiteRunRecordColumns) suiteRunRecordWhere[Q] {
	return suiteRunRecordWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		TenantID:  psql.Where[Q, string](cols.TenantID.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
