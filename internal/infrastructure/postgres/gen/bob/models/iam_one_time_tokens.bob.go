// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
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

// IamOneTimeToken is an object representing the database table.
type IamOneTimeToken struct {
	Token     string    `db:"token,pk" `
	Purpose   string    `db:"purpose" `
	AccountID string    `db:"account_id" `
	ExpiresAt time.Time `db:"expires_at" `
	CreatedAt time.Time `db:"created_at" `
}

// IamOneTimeTokenSlice is an alias for a slice of pointers to IamOneTimeToken.
// This should almost always be used instead of []*IamOneTimeToken.
type IamOneTimeTokenSlice []*IamOneTimeToken

// IamOneTimeTokens contains methods to work with the iam_one_time_tokens table
var IamOneTimeTokens = psql.NewTablex[*IamOneTimeToken, IamOneTimeTokenSlice, *IamOneTimeTokenSetter]("", "iam_one_time_tokens", buildIamOneTimeTokenColumns("iam_one_time_tokens"))

// IamOneTimeTokensQuery is a query on the iam_one_time_tokens table
type IamOneTimeTokensQuery = *psql.ViewQuery[*IamOneTimeToken, IamOneTimeTokenSlice]

func buildIamOneTimeTokenColumns(tableName string) iamOneTimeTokenColumns {
	columnsExpr := expr.NewColumnsExpr(
		"token", "purpose", "account_id", "expires_at", "created_at",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return iamOneTimeTokenColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		Token:       buildIamOneTimeTokenColumn(tableName, "token"),
		Purpose:     buildIamOneTimeTokenColumn(tableName, "purpose"),
		AccountID:   buildIamOneTimeTokenColumn(tableName, "account_id"),
		ExpiresAt:   buildIamOneTimeTokenColumn(tableName, "expires_at"),
		CreatedAt:   buildIamOneTimeTokenColumn(tableName, "created_at"),
	}
}

type iamOneTimeTokenColumns struct {
	expr.ColumnsExpr
	tableAlias string
	Token      iamOneTimeTokenColumn
	Purpose    iamOneTimeTokenColumn
	AccountID  iamOneTimeTokenColumn
	ExpiresAt  iamOneTimeTokenColumn
	CreatedAt  iamOneTimeTokenColumn
}

// Alias returns the current table alias for the columns set.
func (c iamOneTimeTokenColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (iamOneTimeTokenColumns) AliasedAs(tableName string) iamOneTimeTokenColumns {
	return buildIamOneTimeTokenColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c iamOneTimeTokenColumns) Unqualified() iamOneTimeTokenColumns {
	return buildIamOneTimeTokenColumns("")
}

func buildIamOneTimeTokenColumn(alias, name string) iamOneTimeTokenColumn {
	return iamOneTimeTokenColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type iamOneTimeTokenColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c iamOneTimeTokenColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c iamOneTimeTokenColumn) ShouldOmitParens() bool {
	return true
}

// IamOneTimeTokenSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type IamOneTimeTokenSetter struct {
	Token     *string    `db:"token,pk" `
	Purpose   *string    `db:"purpose" `
	AccountID *string    `db:"account_id" `
	ExpiresAt *time.Time `db:"expires_at" `
	CreatedAt *time.Time `db:"created_at" `
}

func (s IamOneTimeTokenSetter) SetColumns() []string {
	vals := make([]string, 0, 5)
	if s.Token != nil {
		vals = append(vals, "token")
	}
	if s.Purpose != nil {
		vals = append(vals, "purpose")
	}
	if s.AccountID != nil {
		vals = append(vals, "account_id")
	}
	if s.ExpiresAt != nil {
		vals = append(vals, "expires_at")
	}
	if s.CreatedAt != nil {
		vals = append(vals, "created_at")
	}
	return vals
}

func (s IamOneTimeTokenSetter) Overwrite(t *IamOneTimeToken) {
	if s.Token != nil {
		t.Token = func() string {
			if s.Token == nil {
				return *new(string)
			}
			return *s.Token
		}()
	}
	if s.Purpose != nil {
		t.Purpose = func() string {
			if s.Purpose == nil {
				return *new(string)
			}
			return *s.Purpose
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
	if s.ExpiresAt != nil {
		t.ExpiresAt = func() time.Time {
			if s.ExpiresAt == nil {
				return *new(time.Time)
			}
			return *s.ExpiresAt
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
}

func (s *IamOneTimeTokenSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return IamOneTimeTokens.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 5)
		if s.Token != nil {
			vals[0] = psql.Arg(func() string {
				if s.Token == nil {
					return *new(string)
				}
				return *s.Token
			}())
		} else {
			vals[0] = psql.Raw("DEFAULT")
		}

		if s.Purpose != nil {
			vals[1] = psql.Arg(func() string {
				if s.Purpose == nil {
					return *new(string)
				}
				return *s.Purpose
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.AccountID != nil {
			vals[2] = psql.Arg(func() string {
				if s.AccountID == nil {
					return *new(string)
				}
				return *s.AccountID
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		if s.ExpiresAt != nil {
			vals[3] = psql.Arg(func() time.Time {
				if s.ExpiresAt == nil {
					return *new(time.Time)
				}
				return *s.ExpiresAt
			}())
		} else {
			vals[3] = psql.Raw("DEFAULT")
		}

		if s.CreatedAt != nil {
			vals[4] = psql.Arg(func() time.Time {
				if s.CreatedAt == nil {
					return *new(time.Time)
				}
				return *s.CreatedAt
			}())
		} else {
			vals[4] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s IamOneTimeTokenSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s IamOneTimeTokenSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 5)

	if s.Token != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "token")...),
			psql.Arg(s.Token),
		}})
	}

	if s.Purpose != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "purpose")...),
			psql.Arg(s.Purpose),
		}})
	}

	if s.AccountID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "account_id")...),
			psql.Arg(s.AccountID),
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

	return exprs
}

// FindIamOneTimeToken retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindIamOneTimeToken(ctx context.Context, exec bob.Executor, TokenPK string, cols ...string) (*IamOneTimeToken, error) {
	if len(cols) == 0 {
		return IamOneTimeTokens.Query(
			sm.Where(IamOneTimeTokens.Columns.Token.EQ(psql.Arg(TokenPK))),
		).One(ctx, exec)
	}

	return IamOneTimeTokens.Query(
		sm.Where(IamOneTimeTokens.Columns.Token.EQ(psql.Arg(TokenPK))),
		sm.Columns(IamOneTimeTokens.Columns.Only(cols...)),
	).One(ctx, exec)
}

// IamOneTimeTokenExists checks the presence of a single record by primary key
func IamOneTimeTokenExists(ctx context.Context, exec bob.Executor, TokenPK string) (bool, error) {
	return IamOneTimeTokens.Query(
		sm.Where(IamOneTimeTokens.Columns.Token.EQ(psql.Arg(TokenPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after IamOneTimeToken is retrieved from the database
func (o *IamOneTimeToken) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamOneTimeTokens.AfterSelectHooks.RunHooks(ctx, exec, IamOneTimeTokenSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = IamOneTimeTokens.AfterInsertHooks.RunHooks(ctx, exec, IamOneTimeTokenSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = IamOneTimeTokens.AfterUpdateHooks.RunHooks(ctx, exec, IamOneTimeTokenSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = IamOneTimeTokens.AfterDeleteHooks.RunHooks(ctx, exec, IamOneTimeTokenSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = IamOneTimeTokens.AfterMergeHooks.RunHooks(ctx, exec, IamOneTimeTokenSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the IamOneTimeToken
func (o *IamOneTimeToken) primaryKeyVals() bob.Expression {
	return psql.Arg(o.Token)
}

func (o *IamOneTimeToken) pkEQ() dialect.Expression {
	return psql.Quote("iam_one_time_tokens", "token").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the IamOneTimeToken
func (o *IamOneTimeToken) Update(ctx context.Context, exec bob.Executor, s *IamOneTimeTokenSetter) error {
	v, err := IamOneTimeTokens.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single IamOneTimeToken record with an executor
func (o *IamOneTimeToken) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := IamOneTimeTokens.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the IamOneTimeToken using the executor
func (o *IamOneTimeToken) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := IamOneTimeTokens.Query(
		sm.Where(IamOneTimeTokens.Columns.Token.EQ(psql.Arg(o.Token))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after IamOneTimeTokenSlice is retrieved from the database
func (o IamOneTimeTokenSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamOneTimeTokens.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = IamOneTimeTokens.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = IamOneTimeTokens.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = IamOneTimeTokens.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = IamOneTimeTokens.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o IamOneTimeTokenSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("iam_one_time_tokens", "token").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o IamOneTimeTokenSlice) copyMatchingRows(from ...*IamOneTimeToken) {
	for i, old := range o {
		for _, new := range from {
			if new.Token != old.Token {
				continue
			}

			o[i] = new
			break
		}
	}
}

// UpdateMod modifies an update query with "WHERE primary_key IN (o...)"
func (o IamOneTimeTokenSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamOneTimeTokens.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamOneTimeToken:
				o.copyMatchingRows(retrieved)
			case []*IamOneTimeToken:
				o.copyMatchingRows(retrieved...)
			case IamOneTimeTokenSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamOneTimeToken or a slice of IamOneTimeToken
				// then run the AfterUpdateHooks on the slice
				_, err = IamOneTimeTokens.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o IamOneTimeTokenSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamOneTimeTokens.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamOneTimeToken:
				o.copyMatchingRows(retrieved)
			case []*IamOneTimeToken:
				o.copyMatchingRows(retrieved...)
			case IamOneTimeTokenSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamOneTimeToken or a slice of IamOneTimeToken
				// then run the AfterDeleteHooks on the slice
				_, err = IamOneTimeTokens.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o IamOneTimeTokenSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamOneTimeTokens.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamOneTimeToken:
				o.copyMatchingRows(retrieved)
			case []*IamOneTimeToken:
				o.copyMatchingRows(retrieved...)
			case IamOneTimeTokenSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamOneTimeToken or a slice of IamOneTimeToken
				// then run the AfterMergeHooks on the slice
				_, err = IamOneTimeTokens.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o IamOneTimeTokenSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals IamOneTimeTokenSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamOneTimeTokens.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o IamOneTimeTokenSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamOneTimeTokens.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o IamOneTimeTokenSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := IamOneTimeTokens.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type iamOneTimeTokenWhere[Q psql.Filterable] struct {
	Token     psql.WhereMod[Q, string]
	Purpose   psql.WhereMod[Q, string]
	AccountID psql.WhereMod[Q, string]
	ExpiresAt psql.WhereMod[Q, time.Time]
	CreatedAt psql.WhereMod[Q, time.Time]
}

func (iamOneTimeTokenWhere[Q]) AliasedAs(alias string) iamOneTimeTokenWhere[Q] {
	return buildIamOneTimeTokenWhere[Q](buildIamOneTimeTokenColumns(alias))
}

func buildIamOneTimeTokenWhere[Q psql.Filterable](cols iamOneTimeTokenColumns) iamOneTimeTokenWhere[Q] {
	return iamOneTimeTokenWhere[Q]{
		Token:     psql.Where[Q, string](cols.Token.Expression),
		Purpose:   psql.Where[Q, string](cols.Purpose.Expression),
		AccountID: psql.Where[Q, string](cols.AccountID.Expression),
		ExpiresAt: psql.Where[Q, time.Time](cols.ExpiresAt.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
	}
}
