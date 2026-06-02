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

// IamIdentityProvider is an object representing the database table.
type IamIdentityProvider struct {
	ID        string          `db:"id,pk" `
	Enabled   bool            `db:"enabled" `
	CreatedAt time.Time       `db:"created_at" `
	UpdatedAt time.Time       `db:"updated_at" `
	Data      json.RawMessage `db:"data" `
}

// IamIdentityProviderSlice is an alias for a slice of pointers to IamIdentityProvider.
// This should almost always be used instead of []*IamIdentityProvider.
type IamIdentityProviderSlice []*IamIdentityProvider

// IamIdentityProviders contains methods to work with the iam_identity_providers table
var IamIdentityProviders = psql.NewTablex[*IamIdentityProvider, IamIdentityProviderSlice, *IamIdentityProviderSetter]("", "iam_identity_providers", buildIamIdentityProviderColumns("iam_identity_providers"))

// IamIdentityProvidersQuery is a query on the iam_identity_providers table
type IamIdentityProvidersQuery = *psql.ViewQuery[*IamIdentityProvider, IamIdentityProviderSlice]

func buildIamIdentityProviderColumns(tableName string) iamIdentityProviderColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "enabled", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return iamIdentityProviderColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildIamIdentityProviderColumn(tableName, "id"),
		Enabled:     buildIamIdentityProviderColumn(tableName, "enabled"),
		CreatedAt:   buildIamIdentityProviderColumn(tableName, "created_at"),
		UpdatedAt:   buildIamIdentityProviderColumn(tableName, "updated_at"),
		Data:        buildIamIdentityProviderColumn(tableName, "data"),
	}
}

type iamIdentityProviderColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         iamIdentityProviderColumn
	Enabled    iamIdentityProviderColumn
	CreatedAt  iamIdentityProviderColumn
	UpdatedAt  iamIdentityProviderColumn
	Data       iamIdentityProviderColumn
}

// Alias returns the current table alias for the columns set.
func (c iamIdentityProviderColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (iamIdentityProviderColumns) AliasedAs(tableName string) iamIdentityProviderColumns {
	return buildIamIdentityProviderColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c iamIdentityProviderColumns) Unqualified() iamIdentityProviderColumns {
	return buildIamIdentityProviderColumns("")
}

func buildIamIdentityProviderColumn(alias, name string) iamIdentityProviderColumn {
	return iamIdentityProviderColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type iamIdentityProviderColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c iamIdentityProviderColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c iamIdentityProviderColumn) ShouldOmitParens() bool {
	return true
}

// IamIdentityProviderSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type IamIdentityProviderSetter struct {
	ID        *string          `db:"id,pk" `
	Enabled   *bool            `db:"enabled" `
	CreatedAt *time.Time       `db:"created_at" `
	UpdatedAt *time.Time       `db:"updated_at" `
	Data      *json.RawMessage `db:"data" `
}

func (s IamIdentityProviderSetter) SetColumns() []string {
	vals := make([]string, 0, 5)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.Enabled != nil {
		vals = append(vals, "enabled")
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

func (s IamIdentityProviderSetter) Overwrite(t *IamIdentityProvider) {
	if s.ID != nil {
		t.ID = func() string {
			if s.ID == nil {
				return *new(string)
			}
			return *s.ID
		}()
	}
	if s.Enabled != nil {
		t.Enabled = func() bool {
			if s.Enabled == nil {
				return *new(bool)
			}
			return *s.Enabled
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

func (s *IamIdentityProviderSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return IamIdentityProviders.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

		if s.Enabled != nil {
			vals[1] = psql.Arg(func() bool {
				if s.Enabled == nil {
					return *new(bool)
				}
				return *s.Enabled
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

func (s IamIdentityProviderSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s IamIdentityProviderSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 5)

	if s.ID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "id")...),
			psql.Arg(s.ID),
		}})
	}

	if s.Enabled != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "enabled")...),
			psql.Arg(s.Enabled),
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

// FindIamIdentityProvider retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindIamIdentityProvider(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*IamIdentityProvider, error) {
	if len(cols) == 0 {
		return IamIdentityProviders.Query(
			sm.Where(IamIdentityProviders.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return IamIdentityProviders.Query(
		sm.Where(IamIdentityProviders.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(IamIdentityProviders.Columns.Only(cols...)),
	).One(ctx, exec)
}

// IamIdentityProviderExists checks the presence of a single record by primary key
func IamIdentityProviderExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return IamIdentityProviders.Query(
		sm.Where(IamIdentityProviders.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after IamIdentityProvider is retrieved from the database
func (o *IamIdentityProvider) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamIdentityProviders.AfterSelectHooks.RunHooks(ctx, exec, IamIdentityProviderSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = IamIdentityProviders.AfterInsertHooks.RunHooks(ctx, exec, IamIdentityProviderSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = IamIdentityProviders.AfterUpdateHooks.RunHooks(ctx, exec, IamIdentityProviderSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = IamIdentityProviders.AfterDeleteHooks.RunHooks(ctx, exec, IamIdentityProviderSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = IamIdentityProviders.AfterMergeHooks.RunHooks(ctx, exec, IamIdentityProviderSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the IamIdentityProvider
func (o *IamIdentityProvider) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *IamIdentityProvider) pkEQ() dialect.Expression {
	return psql.Quote("iam_identity_providers", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the IamIdentityProvider
func (o *IamIdentityProvider) Update(ctx context.Context, exec bob.Executor, s *IamIdentityProviderSetter) error {
	v, err := IamIdentityProviders.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single IamIdentityProvider record with an executor
func (o *IamIdentityProvider) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := IamIdentityProviders.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the IamIdentityProvider using the executor
func (o *IamIdentityProvider) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := IamIdentityProviders.Query(
		sm.Where(IamIdentityProviders.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after IamIdentityProviderSlice is retrieved from the database
func (o IamIdentityProviderSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamIdentityProviders.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = IamIdentityProviders.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = IamIdentityProviders.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = IamIdentityProviders.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = IamIdentityProviders.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o IamIdentityProviderSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("iam_identity_providers", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o IamIdentityProviderSlice) copyMatchingRows(from ...*IamIdentityProvider) {
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
func (o IamIdentityProviderSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamIdentityProviders.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamIdentityProvider:
				o.copyMatchingRows(retrieved)
			case []*IamIdentityProvider:
				o.copyMatchingRows(retrieved...)
			case IamIdentityProviderSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamIdentityProvider or a slice of IamIdentityProvider
				// then run the AfterUpdateHooks on the slice
				_, err = IamIdentityProviders.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o IamIdentityProviderSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamIdentityProviders.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamIdentityProvider:
				o.copyMatchingRows(retrieved)
			case []*IamIdentityProvider:
				o.copyMatchingRows(retrieved...)
			case IamIdentityProviderSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamIdentityProvider or a slice of IamIdentityProvider
				// then run the AfterDeleteHooks on the slice
				_, err = IamIdentityProviders.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o IamIdentityProviderSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamIdentityProviders.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamIdentityProvider:
				o.copyMatchingRows(retrieved)
			case []*IamIdentityProvider:
				o.copyMatchingRows(retrieved...)
			case IamIdentityProviderSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamIdentityProvider or a slice of IamIdentityProvider
				// then run the AfterMergeHooks on the slice
				_, err = IamIdentityProviders.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o IamIdentityProviderSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals IamIdentityProviderSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamIdentityProviders.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o IamIdentityProviderSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamIdentityProviders.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o IamIdentityProviderSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := IamIdentityProviders.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type iamIdentityProviderWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	Enabled   psql.WhereMod[Q, bool]
	CreatedAt psql.WhereMod[Q, time.Time]
	UpdatedAt psql.WhereMod[Q, time.Time]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (iamIdentityProviderWhere[Q]) AliasedAs(alias string) iamIdentityProviderWhere[Q] {
	return buildIamIdentityProviderWhere[Q](buildIamIdentityProviderColumns(alias))
}

func buildIamIdentityProviderWhere[Q psql.Filterable](cols iamIdentityProviderColumns) iamIdentityProviderWhere[Q] {
	return iamIdentityProviderWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		Enabled:   psql.Where[Q, bool](cols.Enabled.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
