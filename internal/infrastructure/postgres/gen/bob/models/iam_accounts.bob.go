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

// IamAccount is an object representing the database table.
type IamAccount struct {
	ID        string          `db:"id,pk" `
	Email     string          `db:"email" `
	Nickname  string          `db:"nickname" `
	CreatedAt time.Time       `db:"created_at" `
	UpdatedAt time.Time       `db:"updated_at" `
	Data      json.RawMessage `db:"data" `
}

// IamAccountSlice is an alias for a slice of pointers to IamAccount.
// This should almost always be used instead of []*IamAccount.
type IamAccountSlice []*IamAccount

// IamAccounts contains methods to work with the iam_accounts table
var IamAccounts = psql.NewTablex[*IamAccount, IamAccountSlice, *IamAccountSetter]("", "iam_accounts", buildIamAccountColumns("iam_accounts"))

// IamAccountsQuery is a query on the iam_accounts table
type IamAccountsQuery = *psql.ViewQuery[*IamAccount, IamAccountSlice]

func buildIamAccountColumns(tableName string) iamAccountColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "email", "nickname", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return iamAccountColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildIamAccountColumn(tableName, "id"),
		Email:       buildIamAccountColumn(tableName, "email"),
		Nickname:    buildIamAccountColumn(tableName, "nickname"),
		CreatedAt:   buildIamAccountColumn(tableName, "created_at"),
		UpdatedAt:   buildIamAccountColumn(tableName, "updated_at"),
		Data:        buildIamAccountColumn(tableName, "data"),
	}
}

type iamAccountColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         iamAccountColumn
	Email      iamAccountColumn
	Nickname   iamAccountColumn
	CreatedAt  iamAccountColumn
	UpdatedAt  iamAccountColumn
	Data       iamAccountColumn
}

// Alias returns the current table alias for the columns set.
func (c iamAccountColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (iamAccountColumns) AliasedAs(tableName string) iamAccountColumns {
	return buildIamAccountColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c iamAccountColumns) Unqualified() iamAccountColumns {
	return buildIamAccountColumns("")
}

func buildIamAccountColumn(alias, name string) iamAccountColumn {
	return iamAccountColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type iamAccountColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c iamAccountColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c iamAccountColumn) ShouldOmitParens() bool {
	return true
}

// IamAccountSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type IamAccountSetter struct {
	ID        *string          `db:"id,pk" `
	Email     *string          `db:"email" `
	Nickname  *string          `db:"nickname" `
	CreatedAt *time.Time       `db:"created_at" `
	UpdatedAt *time.Time       `db:"updated_at" `
	Data      *json.RawMessage `db:"data" `
}

func (s IamAccountSetter) SetColumns() []string {
	vals := make([]string, 0, 6)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.Email != nil {
		vals = append(vals, "email")
	}
	if s.Nickname != nil {
		vals = append(vals, "nickname")
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

func (s IamAccountSetter) Overwrite(t *IamAccount) {
	if s.ID != nil {
		t.ID = func() string {
			if s.ID == nil {
				return *new(string)
			}
			return *s.ID
		}()
	}
	if s.Email != nil {
		t.Email = func() string {
			if s.Email == nil {
				return *new(string)
			}
			return *s.Email
		}()
	}
	if s.Nickname != nil {
		t.Nickname = func() string {
			if s.Nickname == nil {
				return *new(string)
			}
			return *s.Nickname
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

func (s *IamAccountSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return IamAccounts.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

		if s.Email != nil {
			vals[1] = psql.Arg(func() string {
				if s.Email == nil {
					return *new(string)
				}
				return *s.Email
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.Nickname != nil {
			vals[2] = psql.Arg(func() string {
				if s.Nickname == nil {
					return *new(string)
				}
				return *s.Nickname
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

func (s IamAccountSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s IamAccountSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 6)

	if s.ID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "id")...),
			psql.Arg(s.ID),
		}})
	}

	if s.Email != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "email")...),
			psql.Arg(s.Email),
		}})
	}

	if s.Nickname != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "nickname")...),
			psql.Arg(s.Nickname),
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

// FindIamAccount retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindIamAccount(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*IamAccount, error) {
	if len(cols) == 0 {
		return IamAccounts.Query(
			sm.Where(IamAccounts.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return IamAccounts.Query(
		sm.Where(IamAccounts.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(IamAccounts.Columns.Only(cols...)),
	).One(ctx, exec)
}

// IamAccountExists checks the presence of a single record by primary key
func IamAccountExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return IamAccounts.Query(
		sm.Where(IamAccounts.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after IamAccount is retrieved from the database
func (o *IamAccount) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamAccounts.AfterSelectHooks.RunHooks(ctx, exec, IamAccountSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = IamAccounts.AfterInsertHooks.RunHooks(ctx, exec, IamAccountSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = IamAccounts.AfterUpdateHooks.RunHooks(ctx, exec, IamAccountSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = IamAccounts.AfterDeleteHooks.RunHooks(ctx, exec, IamAccountSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = IamAccounts.AfterMergeHooks.RunHooks(ctx, exec, IamAccountSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the IamAccount
func (o *IamAccount) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *IamAccount) pkEQ() dialect.Expression {
	return psql.Quote("iam_accounts", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the IamAccount
func (o *IamAccount) Update(ctx context.Context, exec bob.Executor, s *IamAccountSetter) error {
	v, err := IamAccounts.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single IamAccount record with an executor
func (o *IamAccount) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := IamAccounts.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the IamAccount using the executor
func (o *IamAccount) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := IamAccounts.Query(
		sm.Where(IamAccounts.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after IamAccountSlice is retrieved from the database
func (o IamAccountSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamAccounts.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = IamAccounts.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = IamAccounts.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = IamAccounts.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = IamAccounts.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o IamAccountSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("iam_accounts", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o IamAccountSlice) copyMatchingRows(from ...*IamAccount) {
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
func (o IamAccountSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamAccounts.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamAccount:
				o.copyMatchingRows(retrieved)
			case []*IamAccount:
				o.copyMatchingRows(retrieved...)
			case IamAccountSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamAccount or a slice of IamAccount
				// then run the AfterUpdateHooks on the slice
				_, err = IamAccounts.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o IamAccountSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamAccounts.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamAccount:
				o.copyMatchingRows(retrieved)
			case []*IamAccount:
				o.copyMatchingRows(retrieved...)
			case IamAccountSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamAccount or a slice of IamAccount
				// then run the AfterDeleteHooks on the slice
				_, err = IamAccounts.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o IamAccountSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamAccounts.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamAccount:
				o.copyMatchingRows(retrieved)
			case []*IamAccount:
				o.copyMatchingRows(retrieved...)
			case IamAccountSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamAccount or a slice of IamAccount
				// then run the AfterMergeHooks on the slice
				_, err = IamAccounts.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o IamAccountSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals IamAccountSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamAccounts.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o IamAccountSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamAccounts.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o IamAccountSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := IamAccounts.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type iamAccountWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	Email     psql.WhereMod[Q, string]
	Nickname  psql.WhereMod[Q, string]
	CreatedAt psql.WhereMod[Q, time.Time]
	UpdatedAt psql.WhereMod[Q, time.Time]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (iamAccountWhere[Q]) AliasedAs(alias string) iamAccountWhere[Q] {
	return buildIamAccountWhere[Q](buildIamAccountColumns(alias))
}

func buildIamAccountWhere[Q psql.Filterable](cols iamAccountColumns) iamAccountWhere[Q] {
	return iamAccountWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		Email:     psql.Where[Q, string](cols.Email.Expression),
		Nickname:  psql.Where[Q, string](cols.Nickname.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
