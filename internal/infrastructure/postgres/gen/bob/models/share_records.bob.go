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

// ShareRecord is an object representing the database table.
type ShareRecord struct {
	ID        string          `db:"id,pk" `
	TenantID  string          `db:"tenant_id" `
	Token     string          `db:"token" `
	CreatedAt time.Time       `db:"created_at" `
	UpdatedAt time.Time       `db:"updated_at" `
	Data      json.RawMessage `db:"data" `
}

// ShareRecordSlice is an alias for a slice of pointers to ShareRecord.
// This should almost always be used instead of []*ShareRecord.
type ShareRecordSlice []*ShareRecord

// ShareRecords contains methods to work with the share_records table
var ShareRecords = psql.NewTablex[*ShareRecord, ShareRecordSlice, *ShareRecordSetter]("", "share_records", buildShareRecordColumns("share_records"))

// ShareRecordsQuery is a query on the share_records table
type ShareRecordsQuery = *psql.ViewQuery[*ShareRecord, ShareRecordSlice]

func buildShareRecordColumns(tableName string) shareRecordColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "tenant_id", "token", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return shareRecordColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildShareRecordColumn(tableName, "id"),
		TenantID:    buildShareRecordColumn(tableName, "tenant_id"),
		Token:       buildShareRecordColumn(tableName, "token"),
		CreatedAt:   buildShareRecordColumn(tableName, "created_at"),
		UpdatedAt:   buildShareRecordColumn(tableName, "updated_at"),
		Data:        buildShareRecordColumn(tableName, "data"),
	}
}

type shareRecordColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         shareRecordColumn
	TenantID   shareRecordColumn
	Token      shareRecordColumn
	CreatedAt  shareRecordColumn
	UpdatedAt  shareRecordColumn
	Data       shareRecordColumn
}

// Alias returns the current table alias for the columns set.
func (c shareRecordColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (shareRecordColumns) AliasedAs(tableName string) shareRecordColumns {
	return buildShareRecordColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c shareRecordColumns) Unqualified() shareRecordColumns {
	return buildShareRecordColumns("")
}

func buildShareRecordColumn(alias, name string) shareRecordColumn {
	return shareRecordColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type shareRecordColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c shareRecordColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c shareRecordColumn) ShouldOmitParens() bool {
	return true
}

// ShareRecordSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type ShareRecordSetter struct {
	ID        *string          `db:"id,pk" `
	TenantID  *string          `db:"tenant_id" `
	Token     *string          `db:"token" `
	CreatedAt *time.Time       `db:"created_at" `
	UpdatedAt *time.Time       `db:"updated_at" `
	Data      *json.RawMessage `db:"data" `
}

func (s ShareRecordSetter) SetColumns() []string {
	vals := make([]string, 0, 6)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.TenantID != nil {
		vals = append(vals, "tenant_id")
	}
	if s.Token != nil {
		vals = append(vals, "token")
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

func (s ShareRecordSetter) Overwrite(t *ShareRecord) {
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
	if s.Token != nil {
		t.Token = func() string {
			if s.Token == nil {
				return *new(string)
			}
			return *s.Token
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

func (s *ShareRecordSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return ShareRecords.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 6)
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

		if s.Token != nil {
			vals[2] = psql.Arg(func() string {
				if s.Token == nil {
					return *new(string)
				}
				return *s.Token
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		if s.CreatedAt != nil {
			vals[3] = psql.Arg(func() time.Time {
				if s.CreatedAt == nil {
					return *new(time.Time)
				}
				return *s.CreatedAt
			}())
		} else {
			vals[3] = psql.Raw("DEFAULT")
		}

		if s.UpdatedAt != nil {
			vals[4] = psql.Arg(func() time.Time {
				if s.UpdatedAt == nil {
					return *new(time.Time)
				}
				return *s.UpdatedAt
			}())
		} else {
			vals[4] = psql.Raw("DEFAULT")
		}

		if s.Data != nil {
			vals[5] = psql.Arg(func() json.RawMessage {
				if s.Data == nil {
					return *new(json.RawMessage)
				}
				return *s.Data
			}())
		} else {
			vals[5] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s ShareRecordSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s ShareRecordSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 6)

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

	if s.Token != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "token")...),
			psql.Arg(s.Token),
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

// FindShareRecord retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindShareRecord(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*ShareRecord, error) {
	if len(cols) == 0 {
		return ShareRecords.Query(
			sm.Where(ShareRecords.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return ShareRecords.Query(
		sm.Where(ShareRecords.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(ShareRecords.Columns.Only(cols...)),
	).One(ctx, exec)
}

// ShareRecordExists checks the presence of a single record by primary key
func ShareRecordExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return ShareRecords.Query(
		sm.Where(ShareRecords.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after ShareRecord is retrieved from the database
func (o *ShareRecord) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = ShareRecords.AfterSelectHooks.RunHooks(ctx, exec, ShareRecordSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = ShareRecords.AfterInsertHooks.RunHooks(ctx, exec, ShareRecordSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = ShareRecords.AfterUpdateHooks.RunHooks(ctx, exec, ShareRecordSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = ShareRecords.AfterDeleteHooks.RunHooks(ctx, exec, ShareRecordSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = ShareRecords.AfterMergeHooks.RunHooks(ctx, exec, ShareRecordSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the ShareRecord
func (o *ShareRecord) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *ShareRecord) pkEQ() dialect.Expression {
	return psql.Quote("share_records", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the ShareRecord
func (o *ShareRecord) Update(ctx context.Context, exec bob.Executor, s *ShareRecordSetter) error {
	v, err := ShareRecords.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single ShareRecord record with an executor
func (o *ShareRecord) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := ShareRecords.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the ShareRecord using the executor
func (o *ShareRecord) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := ShareRecords.Query(
		sm.Where(ShareRecords.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after ShareRecordSlice is retrieved from the database
func (o ShareRecordSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = ShareRecords.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = ShareRecords.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = ShareRecords.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = ShareRecords.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = ShareRecords.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o ShareRecordSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("share_records", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o ShareRecordSlice) copyMatchingRows(from ...*ShareRecord) {
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
func (o ShareRecordSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return ShareRecords.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *ShareRecord:
				o.copyMatchingRows(retrieved)
			case []*ShareRecord:
				o.copyMatchingRows(retrieved...)
			case ShareRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a ShareRecord or a slice of ShareRecord
				// then run the AfterUpdateHooks on the slice
				_, err = ShareRecords.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o ShareRecordSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return ShareRecords.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *ShareRecord:
				o.copyMatchingRows(retrieved)
			case []*ShareRecord:
				o.copyMatchingRows(retrieved...)
			case ShareRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a ShareRecord or a slice of ShareRecord
				// then run the AfterDeleteHooks on the slice
				_, err = ShareRecords.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o ShareRecordSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return ShareRecords.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *ShareRecord:
				o.copyMatchingRows(retrieved)
			case []*ShareRecord:
				o.copyMatchingRows(retrieved...)
			case ShareRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a ShareRecord or a slice of ShareRecord
				// then run the AfterMergeHooks on the slice
				_, err = ShareRecords.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o ShareRecordSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals ShareRecordSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := ShareRecords.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o ShareRecordSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := ShareRecords.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o ShareRecordSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := ShareRecords.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type shareRecordWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	TenantID  psql.WhereMod[Q, string]
	Token     psql.WhereMod[Q, string]
	CreatedAt psql.WhereMod[Q, time.Time]
	UpdatedAt psql.WhereMod[Q, time.Time]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (shareRecordWhere[Q]) AliasedAs(alias string) shareRecordWhere[Q] {
	return buildShareRecordWhere[Q](buildShareRecordColumns(alias))
}

func buildShareRecordWhere[Q psql.Filterable](cols shareRecordColumns) shareRecordWhere[Q] {
	return shareRecordWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		TenantID:  psql.Where[Q, string](cols.TenantID.Expression),
		Token:     psql.Where[Q, string](cols.Token.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
