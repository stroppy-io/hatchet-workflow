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

// IamAPIToken is an object representing the database table.
type IamAPIToken struct {
	ID        string          `db:"id,pk" `
	AccountID string          `db:"account_id" `
	CreatedAt time.Time       `db:"created_at" `
	UpdatedAt time.Time       `db:"updated_at" `
	Data      json.RawMessage `db:"data" `
}

// IamAPITokenSlice is an alias for a slice of pointers to IamAPIToken.
// This should almost always be used instead of []*IamAPIToken.
type IamAPITokenSlice []*IamAPIToken

// IamAPITokens contains methods to work with the iam_api_tokens table
var IamAPITokens = psql.NewTablex[*IamAPIToken, IamAPITokenSlice, *IamAPITokenSetter]("", "iam_api_tokens", buildIamAPITokenColumns("iam_api_tokens"))

// IamAPITokensQuery is a query on the iam_api_tokens table
type IamAPITokensQuery = *psql.ViewQuery[*IamAPIToken, IamAPITokenSlice]

func buildIamAPITokenColumns(tableName string) iamAPITokenColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "account_id", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return iamAPITokenColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildIamAPITokenColumn(tableName, "id"),
		AccountID:   buildIamAPITokenColumn(tableName, "account_id"),
		CreatedAt:   buildIamAPITokenColumn(tableName, "created_at"),
		UpdatedAt:   buildIamAPITokenColumn(tableName, "updated_at"),
		Data:        buildIamAPITokenColumn(tableName, "data"),
	}
}

type iamAPITokenColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         iamAPITokenColumn
	AccountID  iamAPITokenColumn
	CreatedAt  iamAPITokenColumn
	UpdatedAt  iamAPITokenColumn
	Data       iamAPITokenColumn
}

// Alias returns the current table alias for the columns set.
func (c iamAPITokenColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (iamAPITokenColumns) AliasedAs(tableName string) iamAPITokenColumns {
	return buildIamAPITokenColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c iamAPITokenColumns) Unqualified() iamAPITokenColumns {
	return buildIamAPITokenColumns("")
}

func buildIamAPITokenColumn(alias, name string) iamAPITokenColumn {
	return iamAPITokenColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type iamAPITokenColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c iamAPITokenColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c iamAPITokenColumn) ShouldOmitParens() bool {
	return true
}

// IamAPITokenSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type IamAPITokenSetter struct {
	ID        *string          `db:"id,pk" `
	AccountID *string          `db:"account_id" `
	CreatedAt *time.Time       `db:"created_at" `
	UpdatedAt *time.Time       `db:"updated_at" `
	Data      *json.RawMessage `db:"data" `
}

func (s IamAPITokenSetter) SetColumns() []string {
	vals := make([]string, 0, 5)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.AccountID != nil {
		vals = append(vals, "account_id")
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

func (s IamAPITokenSetter) Overwrite(t *IamAPIToken) {
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

func (s *IamAPITokenSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return IamAPITokens.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

func (s IamAPITokenSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s IamAPITokenSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 5)

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

// FindIamAPIToken retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindIamAPIToken(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*IamAPIToken, error) {
	if len(cols) == 0 {
		return IamAPITokens.Query(
			sm.Where(IamAPITokens.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return IamAPITokens.Query(
		sm.Where(IamAPITokens.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(IamAPITokens.Columns.Only(cols...)),
	).One(ctx, exec)
}

// IamAPITokenExists checks the presence of a single record by primary key
func IamAPITokenExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return IamAPITokens.Query(
		sm.Where(IamAPITokens.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after IamAPIToken is retrieved from the database
func (o *IamAPIToken) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamAPITokens.AfterSelectHooks.RunHooks(ctx, exec, IamAPITokenSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = IamAPITokens.AfterInsertHooks.RunHooks(ctx, exec, IamAPITokenSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = IamAPITokens.AfterUpdateHooks.RunHooks(ctx, exec, IamAPITokenSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = IamAPITokens.AfterDeleteHooks.RunHooks(ctx, exec, IamAPITokenSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = IamAPITokens.AfterMergeHooks.RunHooks(ctx, exec, IamAPITokenSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the IamAPIToken
func (o *IamAPIToken) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *IamAPIToken) pkEQ() dialect.Expression {
	return psql.Quote("iam_api_tokens", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the IamAPIToken
func (o *IamAPIToken) Update(ctx context.Context, exec bob.Executor, s *IamAPITokenSetter) error {
	v, err := IamAPITokens.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single IamAPIToken record with an executor
func (o *IamAPIToken) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := IamAPITokens.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the IamAPIToken using the executor
func (o *IamAPIToken) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := IamAPITokens.Query(
		sm.Where(IamAPITokens.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after IamAPITokenSlice is retrieved from the database
func (o IamAPITokenSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamAPITokens.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = IamAPITokens.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = IamAPITokens.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = IamAPITokens.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = IamAPITokens.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o IamAPITokenSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("iam_api_tokens", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o IamAPITokenSlice) copyMatchingRows(from ...*IamAPIToken) {
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
func (o IamAPITokenSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamAPITokens.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamAPIToken:
				o.copyMatchingRows(retrieved)
			case []*IamAPIToken:
				o.copyMatchingRows(retrieved...)
			case IamAPITokenSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamAPIToken or a slice of IamAPIToken
				// then run the AfterUpdateHooks on the slice
				_, err = IamAPITokens.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o IamAPITokenSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamAPITokens.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamAPIToken:
				o.copyMatchingRows(retrieved)
			case []*IamAPIToken:
				o.copyMatchingRows(retrieved...)
			case IamAPITokenSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamAPIToken or a slice of IamAPIToken
				// then run the AfterDeleteHooks on the slice
				_, err = IamAPITokens.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o IamAPITokenSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamAPITokens.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamAPIToken:
				o.copyMatchingRows(retrieved)
			case []*IamAPIToken:
				o.copyMatchingRows(retrieved...)
			case IamAPITokenSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamAPIToken or a slice of IamAPIToken
				// then run the AfterMergeHooks on the slice
				_, err = IamAPITokens.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o IamAPITokenSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals IamAPITokenSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamAPITokens.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o IamAPITokenSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamAPITokens.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o IamAPITokenSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := IamAPITokens.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type iamAPITokenWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	AccountID psql.WhereMod[Q, string]
	CreatedAt psql.WhereMod[Q, time.Time]
	UpdatedAt psql.WhereMod[Q, time.Time]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (iamAPITokenWhere[Q]) AliasedAs(alias string) iamAPITokenWhere[Q] {
	return buildIamAPITokenWhere[Q](buildIamAPITokenColumns(alias))
}

func buildIamAPITokenWhere[Q psql.Filterable](cols iamAPITokenColumns) iamAPITokenWhere[Q] {
	return iamAPITokenWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		AccountID: psql.Where[Q, string](cols.AccountID.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
