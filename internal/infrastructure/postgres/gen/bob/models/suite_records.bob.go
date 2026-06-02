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

// SuiteRecord is an object representing the database table.
type SuiteRecord struct {
	ID        string          `db:"id,pk" `
	TenantID  string          `db:"tenant_id" `
	CreatedAt time.Time       `db:"created_at" `
	UpdatedAt time.Time       `db:"updated_at" `
	Data      json.RawMessage `db:"data" `
}

// SuiteRecordSlice is an alias for a slice of pointers to SuiteRecord.
// This should almost always be used instead of []*SuiteRecord.
type SuiteRecordSlice []*SuiteRecord

// SuiteRecords contains methods to work with the suite_records table
var SuiteRecords = psql.NewTablex[*SuiteRecord, SuiteRecordSlice, *SuiteRecordSetter]("", "suite_records", buildSuiteRecordColumns("suite_records"))

// SuiteRecordsQuery is a query on the suite_records table
type SuiteRecordsQuery = *psql.ViewQuery[*SuiteRecord, SuiteRecordSlice]

func buildSuiteRecordColumns(tableName string) suiteRecordColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "tenant_id", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return suiteRecordColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildSuiteRecordColumn(tableName, "id"),
		TenantID:    buildSuiteRecordColumn(tableName, "tenant_id"),
		CreatedAt:   buildSuiteRecordColumn(tableName, "created_at"),
		UpdatedAt:   buildSuiteRecordColumn(tableName, "updated_at"),
		Data:        buildSuiteRecordColumn(tableName, "data"),
	}
}

type suiteRecordColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         suiteRecordColumn
	TenantID   suiteRecordColumn
	CreatedAt  suiteRecordColumn
	UpdatedAt  suiteRecordColumn
	Data       suiteRecordColumn
}

// Alias returns the current table alias for the columns set.
func (c suiteRecordColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (suiteRecordColumns) AliasedAs(tableName string) suiteRecordColumns {
	return buildSuiteRecordColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c suiteRecordColumns) Unqualified() suiteRecordColumns {
	return buildSuiteRecordColumns("")
}

func buildSuiteRecordColumn(alias, name string) suiteRecordColumn {
	return suiteRecordColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type suiteRecordColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c suiteRecordColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c suiteRecordColumn) ShouldOmitParens() bool {
	return true
}

// SuiteRecordSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type SuiteRecordSetter struct {
	ID        *string          `db:"id,pk" `
	TenantID  *string          `db:"tenant_id" `
	CreatedAt *time.Time       `db:"created_at" `
	UpdatedAt *time.Time       `db:"updated_at" `
	Data      *json.RawMessage `db:"data" `
}

func (s SuiteRecordSetter) SetColumns() []string {
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

func (s SuiteRecordSetter) Overwrite(t *SuiteRecord) {
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

func (s *SuiteRecordSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return SuiteRecords.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

func (s SuiteRecordSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s SuiteRecordSetter) Expressions(prefix ...string) []bob.Expression {
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

// FindSuiteRecord retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindSuiteRecord(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*SuiteRecord, error) {
	if len(cols) == 0 {
		return SuiteRecords.Query(
			sm.Where(SuiteRecords.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return SuiteRecords.Query(
		sm.Where(SuiteRecords.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(SuiteRecords.Columns.Only(cols...)),
	).One(ctx, exec)
}

// SuiteRecordExists checks the presence of a single record by primary key
func SuiteRecordExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return SuiteRecords.Query(
		sm.Where(SuiteRecords.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after SuiteRecord is retrieved from the database
func (o *SuiteRecord) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = SuiteRecords.AfterSelectHooks.RunHooks(ctx, exec, SuiteRecordSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = SuiteRecords.AfterInsertHooks.RunHooks(ctx, exec, SuiteRecordSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = SuiteRecords.AfterUpdateHooks.RunHooks(ctx, exec, SuiteRecordSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = SuiteRecords.AfterDeleteHooks.RunHooks(ctx, exec, SuiteRecordSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = SuiteRecords.AfterMergeHooks.RunHooks(ctx, exec, SuiteRecordSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the SuiteRecord
func (o *SuiteRecord) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *SuiteRecord) pkEQ() dialect.Expression {
	return psql.Quote("suite_records", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the SuiteRecord
func (o *SuiteRecord) Update(ctx context.Context, exec bob.Executor, s *SuiteRecordSetter) error {
	v, err := SuiteRecords.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single SuiteRecord record with an executor
func (o *SuiteRecord) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := SuiteRecords.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the SuiteRecord using the executor
func (o *SuiteRecord) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := SuiteRecords.Query(
		sm.Where(SuiteRecords.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after SuiteRecordSlice is retrieved from the database
func (o SuiteRecordSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = SuiteRecords.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = SuiteRecords.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = SuiteRecords.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = SuiteRecords.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = SuiteRecords.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o SuiteRecordSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("suite_records", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o SuiteRecordSlice) copyMatchingRows(from ...*SuiteRecord) {
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
func (o SuiteRecordSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return SuiteRecords.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *SuiteRecord:
				o.copyMatchingRows(retrieved)
			case []*SuiteRecord:
				o.copyMatchingRows(retrieved...)
			case SuiteRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a SuiteRecord or a slice of SuiteRecord
				// then run the AfterUpdateHooks on the slice
				_, err = SuiteRecords.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o SuiteRecordSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return SuiteRecords.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *SuiteRecord:
				o.copyMatchingRows(retrieved)
			case []*SuiteRecord:
				o.copyMatchingRows(retrieved...)
			case SuiteRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a SuiteRecord or a slice of SuiteRecord
				// then run the AfterDeleteHooks on the slice
				_, err = SuiteRecords.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o SuiteRecordSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return SuiteRecords.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *SuiteRecord:
				o.copyMatchingRows(retrieved)
			case []*SuiteRecord:
				o.copyMatchingRows(retrieved...)
			case SuiteRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a SuiteRecord or a slice of SuiteRecord
				// then run the AfterMergeHooks on the slice
				_, err = SuiteRecords.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o SuiteRecordSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals SuiteRecordSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := SuiteRecords.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o SuiteRecordSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := SuiteRecords.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o SuiteRecordSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := SuiteRecords.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type suiteRecordWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	TenantID  psql.WhereMod[Q, string]
	CreatedAt psql.WhereMod[Q, time.Time]
	UpdatedAt psql.WhereMod[Q, time.Time]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (suiteRecordWhere[Q]) AliasedAs(alias string) suiteRecordWhere[Q] {
	return buildSuiteRecordWhere[Q](buildSuiteRecordColumns(alias))
}

func buildSuiteRecordWhere[Q psql.Filterable](cols suiteRecordColumns) suiteRecordWhere[Q] {
	return suiteRecordWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		TenantID:  psql.Where[Q, string](cols.TenantID.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
