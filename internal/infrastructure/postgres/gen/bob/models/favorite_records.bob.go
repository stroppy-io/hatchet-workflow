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

// FavoriteRecord is an object representing the database table.
type FavoriteRecord struct {
	ID        string          `db:"id,pk" `
	AccountID string          `db:"account_id" `
	TenantID  string          `db:"tenant_id" `
	Kind      int32           `db:"kind" `
	TargetID  string          `db:"target_id" `
	CreatedAt time.Time       `db:"created_at" `
	UpdatedAt time.Time       `db:"updated_at" `
	Data      json.RawMessage `db:"data" `
}

// FavoriteRecordSlice is an alias for a slice of pointers to FavoriteRecord.
// This should almost always be used instead of []*FavoriteRecord.
type FavoriteRecordSlice []*FavoriteRecord

// FavoriteRecords contains methods to work with the favorite_records table
var FavoriteRecords = psql.NewTablex[*FavoriteRecord, FavoriteRecordSlice, *FavoriteRecordSetter]("", "favorite_records", buildFavoriteRecordColumns("favorite_records"))

// FavoriteRecordsQuery is a query on the favorite_records table
type FavoriteRecordsQuery = *psql.ViewQuery[*FavoriteRecord, FavoriteRecordSlice]

func buildFavoriteRecordColumns(tableName string) favoriteRecordColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "account_id", "tenant_id", "kind", "target_id", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return favoriteRecordColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildFavoriteRecordColumn(tableName, "id"),
		AccountID:   buildFavoriteRecordColumn(tableName, "account_id"),
		TenantID:    buildFavoriteRecordColumn(tableName, "tenant_id"),
		Kind:        buildFavoriteRecordColumn(tableName, "kind"),
		TargetID:    buildFavoriteRecordColumn(tableName, "target_id"),
		CreatedAt:   buildFavoriteRecordColumn(tableName, "created_at"),
		UpdatedAt:   buildFavoriteRecordColumn(tableName, "updated_at"),
		Data:        buildFavoriteRecordColumn(tableName, "data"),
	}
}

type favoriteRecordColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         favoriteRecordColumn
	AccountID  favoriteRecordColumn
	TenantID   favoriteRecordColumn
	Kind       favoriteRecordColumn
	TargetID   favoriteRecordColumn
	CreatedAt  favoriteRecordColumn
	UpdatedAt  favoriteRecordColumn
	Data       favoriteRecordColumn
}

// Alias returns the current table alias for the columns set.
func (c favoriteRecordColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (favoriteRecordColumns) AliasedAs(tableName string) favoriteRecordColumns {
	return buildFavoriteRecordColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c favoriteRecordColumns) Unqualified() favoriteRecordColumns {
	return buildFavoriteRecordColumns("")
}

func buildFavoriteRecordColumn(alias, name string) favoriteRecordColumn {
	return favoriteRecordColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type favoriteRecordColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c favoriteRecordColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c favoriteRecordColumn) ShouldOmitParens() bool {
	return true
}

// FavoriteRecordSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type FavoriteRecordSetter struct {
	ID        *string          `db:"id,pk" `
	AccountID *string          `db:"account_id" `
	TenantID  *string          `db:"tenant_id" `
	Kind      *int32           `db:"kind" `
	TargetID  *string          `db:"target_id" `
	CreatedAt *time.Time       `db:"created_at" `
	UpdatedAt *time.Time       `db:"updated_at" `
	Data      *json.RawMessage `db:"data" `
}

func (s FavoriteRecordSetter) SetColumns() []string {
	vals := make([]string, 0, 8)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.AccountID != nil {
		vals = append(vals, "account_id")
	}
	if s.TenantID != nil {
		vals = append(vals, "tenant_id")
	}
	if s.Kind != nil {
		vals = append(vals, "kind")
	}
	if s.TargetID != nil {
		vals = append(vals, "target_id")
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

func (s FavoriteRecordSetter) Overwrite(t *FavoriteRecord) {
	if s.ID != nil {
		t.ID = func() string {
			if s.ID == nil {
				return *new(string)
			}
			return *s.ID
		}()
	}
	if s.AccountID != nil {
		t.AccountID = func() string {
			if s.AccountID == nil {
				return *new(string)
			}
			return *s.AccountID
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
	if s.Kind != nil {
		t.Kind = func() int32 {
			if s.Kind == nil {
				return *new(int32)
			}
			return *s.Kind
		}()
	}
	if s.TargetID != nil {
		t.TargetID = func() string {
			if s.TargetID == nil {
				return *new(string)
			}
			return *s.TargetID
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

func (s *FavoriteRecordSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return FavoriteRecords.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 8)
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

		if s.AccountID != nil {
			vals[1] = psql.Arg(func() string {
				if s.AccountID == nil {
					return *new(string)
				}
				return *s.AccountID
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.TenantID != nil {
			vals[2] = psql.Arg(func() string {
				if s.TenantID == nil {
					return *new(string)
				}
				return *s.TenantID
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		if s.Kind != nil {
			vals[3] = psql.Arg(func() int32 {
				if s.Kind == nil {
					return *new(int32)
				}
				return *s.Kind
			}())
		} else {
			vals[3] = psql.Raw("DEFAULT")
		}

		if s.TargetID != nil {
			vals[4] = psql.Arg(func() string {
				if s.TargetID == nil {
					return *new(string)
				}
				return *s.TargetID
			}())
		} else {
			vals[4] = psql.Raw("DEFAULT")
		}

		if s.CreatedAt != nil {
			vals[5] = psql.Arg(func() time.Time {
				if s.CreatedAt == nil {
					return *new(time.Time)
				}
				return *s.CreatedAt
			}())
		} else {
			vals[5] = psql.Raw("DEFAULT")
		}

		if s.UpdatedAt != nil {
			vals[6] = psql.Arg(func() time.Time {
				if s.UpdatedAt == nil {
					return *new(time.Time)
				}
				return *s.UpdatedAt
			}())
		} else {
			vals[6] = psql.Raw("DEFAULT")
		}

		if s.Data != nil {
			vals[7] = psql.Arg(func() json.RawMessage {
				if s.Data == nil {
					return *new(json.RawMessage)
				}
				return *s.Data
			}())
		} else {
			vals[7] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s FavoriteRecordSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s FavoriteRecordSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 8)

	if s.ID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "id")...),
			psql.Arg(s.ID),
		}})
	}

	if s.AccountID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "account_id")...),
			psql.Arg(s.AccountID),
		}})
	}

	if s.TenantID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "tenant_id")...),
			psql.Arg(s.TenantID),
		}})
	}

	if s.Kind != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "kind")...),
			psql.Arg(s.Kind),
		}})
	}

	if s.TargetID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "target_id")...),
			psql.Arg(s.TargetID),
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

// FindFavoriteRecord retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindFavoriteRecord(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*FavoriteRecord, error) {
	if len(cols) == 0 {
		return FavoriteRecords.Query(
			sm.Where(FavoriteRecords.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return FavoriteRecords.Query(
		sm.Where(FavoriteRecords.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(FavoriteRecords.Columns.Only(cols...)),
	).One(ctx, exec)
}

// FavoriteRecordExists checks the presence of a single record by primary key
func FavoriteRecordExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return FavoriteRecords.Query(
		sm.Where(FavoriteRecords.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after FavoriteRecord is retrieved from the database
func (o *FavoriteRecord) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = FavoriteRecords.AfterSelectHooks.RunHooks(ctx, exec, FavoriteRecordSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = FavoriteRecords.AfterInsertHooks.RunHooks(ctx, exec, FavoriteRecordSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = FavoriteRecords.AfterUpdateHooks.RunHooks(ctx, exec, FavoriteRecordSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = FavoriteRecords.AfterDeleteHooks.RunHooks(ctx, exec, FavoriteRecordSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = FavoriteRecords.AfterMergeHooks.RunHooks(ctx, exec, FavoriteRecordSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the FavoriteRecord
func (o *FavoriteRecord) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *FavoriteRecord) pkEQ() dialect.Expression {
	return psql.Quote("favorite_records", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the FavoriteRecord
func (o *FavoriteRecord) Update(ctx context.Context, exec bob.Executor, s *FavoriteRecordSetter) error {
	v, err := FavoriteRecords.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single FavoriteRecord record with an executor
func (o *FavoriteRecord) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := FavoriteRecords.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the FavoriteRecord using the executor
func (o *FavoriteRecord) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := FavoriteRecords.Query(
		sm.Where(FavoriteRecords.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after FavoriteRecordSlice is retrieved from the database
func (o FavoriteRecordSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = FavoriteRecords.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = FavoriteRecords.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = FavoriteRecords.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = FavoriteRecords.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = FavoriteRecords.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o FavoriteRecordSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("favorite_records", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o FavoriteRecordSlice) copyMatchingRows(from ...*FavoriteRecord) {
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
func (o FavoriteRecordSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return FavoriteRecords.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *FavoriteRecord:
				o.copyMatchingRows(retrieved)
			case []*FavoriteRecord:
				o.copyMatchingRows(retrieved...)
			case FavoriteRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a FavoriteRecord or a slice of FavoriteRecord
				// then run the AfterUpdateHooks on the slice
				_, err = FavoriteRecords.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o FavoriteRecordSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return FavoriteRecords.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *FavoriteRecord:
				o.copyMatchingRows(retrieved)
			case []*FavoriteRecord:
				o.copyMatchingRows(retrieved...)
			case FavoriteRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a FavoriteRecord or a slice of FavoriteRecord
				// then run the AfterDeleteHooks on the slice
				_, err = FavoriteRecords.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o FavoriteRecordSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return FavoriteRecords.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *FavoriteRecord:
				o.copyMatchingRows(retrieved)
			case []*FavoriteRecord:
				o.copyMatchingRows(retrieved...)
			case FavoriteRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a FavoriteRecord or a slice of FavoriteRecord
				// then run the AfterMergeHooks on the slice
				_, err = FavoriteRecords.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o FavoriteRecordSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals FavoriteRecordSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := FavoriteRecords.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o FavoriteRecordSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := FavoriteRecords.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o FavoriteRecordSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := FavoriteRecords.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type favoriteRecordWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	AccountID psql.WhereMod[Q, string]
	TenantID  psql.WhereMod[Q, string]
	Kind      psql.WhereMod[Q, int32]
	TargetID  psql.WhereMod[Q, string]
	CreatedAt psql.WhereMod[Q, time.Time]
	UpdatedAt psql.WhereMod[Q, time.Time]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (favoriteRecordWhere[Q]) AliasedAs(alias string) favoriteRecordWhere[Q] {
	return buildFavoriteRecordWhere[Q](buildFavoriteRecordColumns(alias))
}

func buildFavoriteRecordWhere[Q psql.Filterable](cols favoriteRecordColumns) favoriteRecordWhere[Q] {
	return favoriteRecordWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		AccountID: psql.Where[Q, string](cols.AccountID.Expression),
		TenantID:  psql.Where[Q, string](cols.TenantID.Expression),
		Kind:      psql.Where[Q, int32](cols.Kind.Expression),
		TargetID:  psql.Where[Q, string](cols.TargetID.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
