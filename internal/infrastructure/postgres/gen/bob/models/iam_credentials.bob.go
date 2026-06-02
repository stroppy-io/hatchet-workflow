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

// IamCredential is an object representing the database table.
type IamCredential struct {
	AccountID string    `db:"account_id,pk" `
	Hash      string    `db:"hash" `
	UpdatedAt time.Time `db:"updated_at" `
}

// IamCredentialSlice is an alias for a slice of pointers to IamCredential.
// This should almost always be used instead of []*IamCredential.
type IamCredentialSlice []*IamCredential

// IamCredentials contains methods to work with the iam_credentials table
var IamCredentials = psql.NewTablex[*IamCredential, IamCredentialSlice, *IamCredentialSetter]("", "iam_credentials", buildIamCredentialColumns("iam_credentials"))

// IamCredentialsQuery is a query on the iam_credentials table
type IamCredentialsQuery = *psql.ViewQuery[*IamCredential, IamCredentialSlice]

func buildIamCredentialColumns(tableName string) iamCredentialColumns {
	columnsExpr := expr.NewColumnsExpr(
		"account_id", "hash", "updated_at",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return iamCredentialColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		AccountID:   buildIamCredentialColumn(tableName, "account_id"),
		Hash:        buildIamCredentialColumn(tableName, "hash"),
		UpdatedAt:   buildIamCredentialColumn(tableName, "updated_at"),
	}
}

type iamCredentialColumns struct {
	expr.ColumnsExpr
	tableAlias string
	AccountID  iamCredentialColumn
	Hash       iamCredentialColumn
	UpdatedAt  iamCredentialColumn
}

// Alias returns the current table alias for the columns set.
func (c iamCredentialColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (iamCredentialColumns) AliasedAs(tableName string) iamCredentialColumns {
	return buildIamCredentialColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c iamCredentialColumns) Unqualified() iamCredentialColumns {
	return buildIamCredentialColumns("")
}

func buildIamCredentialColumn(alias, name string) iamCredentialColumn {
	return iamCredentialColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type iamCredentialColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c iamCredentialColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c iamCredentialColumn) ShouldOmitParens() bool {
	return true
}

// IamCredentialSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type IamCredentialSetter struct {
	AccountID *string    `db:"account_id,pk" `
	Hash      *string    `db:"hash" `
	UpdatedAt *time.Time `db:"updated_at" `
}

func (s IamCredentialSetter) SetColumns() []string {
	vals := make([]string, 0, 3)
	if s.AccountID != nil {
		vals = append(vals, "account_id")
	}
	if s.Hash != nil {
		vals = append(vals, "hash")
	}
	if s.UpdatedAt != nil {
		vals = append(vals, "updated_at")
	}
	return vals
}

func (s IamCredentialSetter) Overwrite(t *IamCredential) {
	if s.AccountID != nil {
		t.AccountID = func() string {
			if s.AccountID == nil {
				return *new(string)
			}
			return *s.AccountID
		}()
	}
	if s.Hash != nil {
		t.Hash = func() string {
			if s.Hash == nil {
				return *new(string)
			}
			return *s.Hash
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

func (s *IamCredentialSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return IamCredentials.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 3)
		if s.AccountID != nil {
			vals[0] = psql.Arg(func() string {
				if s.AccountID == nil {
					return *new(string)
				}
				return *s.AccountID
			}())
		} else {
			vals[0] = psql.Raw("DEFAULT")
		}

		if s.Hash != nil {
			vals[1] = psql.Arg(func() string {
				if s.Hash == nil {
					return *new(string)
				}
				return *s.Hash
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.UpdatedAt != nil {
			vals[2] = psql.Arg(func() time.Time {
				if s.UpdatedAt == nil {
					return *new(time.Time)
				}
				return *s.UpdatedAt
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s IamCredentialSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s IamCredentialSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 3)

	if s.AccountID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "account_id")...),
			psql.Arg(s.AccountID),
		}})
	}

	if s.Hash != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "hash")...),
			psql.Arg(s.Hash),
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

// FindIamCredential retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindIamCredential(ctx context.Context, exec bob.Executor, AccountIDPK string, cols ...string) (*IamCredential, error) {
	if len(cols) == 0 {
		return IamCredentials.Query(
			sm.Where(IamCredentials.Columns.AccountID.EQ(psql.Arg(AccountIDPK))),
		).One(ctx, exec)
	}

	return IamCredentials.Query(
		sm.Where(IamCredentials.Columns.AccountID.EQ(psql.Arg(AccountIDPK))),
		sm.Columns(IamCredentials.Columns.Only(cols...)),
	).One(ctx, exec)
}

// IamCredentialExists checks the presence of a single record by primary key
func IamCredentialExists(ctx context.Context, exec bob.Executor, AccountIDPK string) (bool, error) {
	return IamCredentials.Query(
		sm.Where(IamCredentials.Columns.AccountID.EQ(psql.Arg(AccountIDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after IamCredential is retrieved from the database
func (o *IamCredential) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamCredentials.AfterSelectHooks.RunHooks(ctx, exec, IamCredentialSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = IamCredentials.AfterInsertHooks.RunHooks(ctx, exec, IamCredentialSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = IamCredentials.AfterUpdateHooks.RunHooks(ctx, exec, IamCredentialSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = IamCredentials.AfterDeleteHooks.RunHooks(ctx, exec, IamCredentialSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = IamCredentials.AfterMergeHooks.RunHooks(ctx, exec, IamCredentialSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the IamCredential
func (o *IamCredential) primaryKeyVals() bob.Expression {
	return psql.Arg(o.AccountID)
}

func (o *IamCredential) pkEQ() dialect.Expression {
	return psql.Quote("iam_credentials", "account_id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the IamCredential
func (o *IamCredential) Update(ctx context.Context, exec bob.Executor, s *IamCredentialSetter) error {
	v, err := IamCredentials.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single IamCredential record with an executor
func (o *IamCredential) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := IamCredentials.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the IamCredential using the executor
func (o *IamCredential) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := IamCredentials.Query(
		sm.Where(IamCredentials.Columns.AccountID.EQ(psql.Arg(o.AccountID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after IamCredentialSlice is retrieved from the database
func (o IamCredentialSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamCredentials.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = IamCredentials.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = IamCredentials.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = IamCredentials.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = IamCredentials.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o IamCredentialSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("iam_credentials", "account_id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o IamCredentialSlice) copyMatchingRows(from ...*IamCredential) {
	for i, old := range o {
		for _, new := range from {
			if new.AccountID != old.AccountID {
				continue
			}

			o[i] = new
			break
		}
	}
}

// UpdateMod modifies an update query with "WHERE primary_key IN (o...)"
func (o IamCredentialSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamCredentials.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamCredential:
				o.copyMatchingRows(retrieved)
			case []*IamCredential:
				o.copyMatchingRows(retrieved...)
			case IamCredentialSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamCredential or a slice of IamCredential
				// then run the AfterUpdateHooks on the slice
				_, err = IamCredentials.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o IamCredentialSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamCredentials.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamCredential:
				o.copyMatchingRows(retrieved)
			case []*IamCredential:
				o.copyMatchingRows(retrieved...)
			case IamCredentialSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamCredential or a slice of IamCredential
				// then run the AfterDeleteHooks on the slice
				_, err = IamCredentials.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o IamCredentialSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamCredentials.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamCredential:
				o.copyMatchingRows(retrieved)
			case []*IamCredential:
				o.copyMatchingRows(retrieved...)
			case IamCredentialSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamCredential or a slice of IamCredential
				// then run the AfterMergeHooks on the slice
				_, err = IamCredentials.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o IamCredentialSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals IamCredentialSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamCredentials.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o IamCredentialSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamCredentials.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o IamCredentialSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := IamCredentials.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type iamCredentialWhere[Q psql.Filterable] struct {
	AccountID psql.WhereMod[Q, string]
	Hash      psql.WhereMod[Q, string]
	UpdatedAt psql.WhereMod[Q, time.Time]
}

func (iamCredentialWhere[Q]) AliasedAs(alias string) iamCredentialWhere[Q] {
	return buildIamCredentialWhere[Q](buildIamCredentialColumns(alias))
}

func buildIamCredentialWhere[Q psql.Filterable](cols iamCredentialColumns) iamCredentialWhere[Q] {
	return iamCredentialWhere[Q]{
		AccountID: psql.Where[Q, string](cols.AccountID.Expression),
		Hash:      psql.Where[Q, string](cols.Hash.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
	}
}
