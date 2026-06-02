// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"io"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/bob/expr"
)

// IdentitySecret is an object representing the database table.
type IdentitySecret struct {
	Namespace string `db:"namespace,pk" `
	Key       string `db:"key,pk" `
	Value     string `db:"value" `
}

// IdentitySecretSlice is an alias for a slice of pointers to IdentitySecret.
// This should almost always be used instead of []*IdentitySecret.
type IdentitySecretSlice []*IdentitySecret

// IdentitySecrets contains methods to work with the identity_secrets table
var IdentitySecrets = psql.NewTablex[*IdentitySecret, IdentitySecretSlice, *IdentitySecretSetter]("", "identity_secrets", buildIdentitySecretColumns("identity_secrets"))

// IdentitySecretsQuery is a query on the identity_secrets table
type IdentitySecretsQuery = *psql.ViewQuery[*IdentitySecret, IdentitySecretSlice]

func buildIdentitySecretColumns(tableName string) identitySecretColumns {
	columnsExpr := expr.NewColumnsExpr(
		"namespace", "key", "value",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return identitySecretColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		Namespace:   buildIdentitySecretColumn(tableName, "namespace"),
		Key:         buildIdentitySecretColumn(tableName, "key"),
		Value:       buildIdentitySecretColumn(tableName, "value"),
	}
}

type identitySecretColumns struct {
	expr.ColumnsExpr
	tableAlias string
	Namespace  identitySecretColumn
	Key        identitySecretColumn
	Value      identitySecretColumn
}

// Alias returns the current table alias for the columns set.
func (c identitySecretColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (identitySecretColumns) AliasedAs(tableName string) identitySecretColumns {
	return buildIdentitySecretColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c identitySecretColumns) Unqualified() identitySecretColumns {
	return buildIdentitySecretColumns("")
}

func buildIdentitySecretColumn(alias, name string) identitySecretColumn {
	return identitySecretColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type identitySecretColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c identitySecretColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c identitySecretColumn) ShouldOmitParens() bool {
	return true
}

// IdentitySecretSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type IdentitySecretSetter struct {
	Namespace *string `db:"namespace,pk" `
	Key       *string `db:"key,pk" `
	Value     *string `db:"value" `
}

func (s IdentitySecretSetter) SetColumns() []string {
	vals := make([]string, 0, 3)
	if s.Namespace != nil {
		vals = append(vals, "namespace")
	}
	if s.Key != nil {
		vals = append(vals, "key")
	}
	if s.Value != nil {
		vals = append(vals, "value")
	}
	return vals
}

func (s IdentitySecretSetter) Overwrite(t *IdentitySecret) {
	if s.Namespace != nil {
		t.Namespace = func() string {
			if s.Namespace == nil {
				return *new(string)
			}
			return *s.Namespace
		}()
	}
	if s.Key != nil {
		t.Key = func() string {
			if s.Key == nil {
				return *new(string)
			}
			return *s.Key
		}()
	}
	if s.Value != nil {
		t.Value = func() string {
			if s.Value == nil {
				return *new(string)
			}
			return *s.Value
		}()
	}
}

func (s *IdentitySecretSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return IdentitySecrets.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 3)
		if s.Namespace != nil {
			vals[0] = psql.Arg(func() string {
				if s.Namespace == nil {
					return *new(string)
				}
				return *s.Namespace
			}())
		} else {
			vals[0] = psql.Raw("DEFAULT")
		}

		if s.Key != nil {
			vals[1] = psql.Arg(func() string {
				if s.Key == nil {
					return *new(string)
				}
				return *s.Key
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.Value != nil {
			vals[2] = psql.Arg(func() string {
				if s.Value == nil {
					return *new(string)
				}
				return *s.Value
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s IdentitySecretSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s IdentitySecretSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 3)

	if s.Namespace != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "namespace")...),
			psql.Arg(s.Namespace),
		}})
	}

	if s.Key != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "key")...),
			psql.Arg(s.Key),
		}})
	}

	if s.Value != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "value")...),
			psql.Arg(s.Value),
		}})
	}

	return exprs
}

// FindIdentitySecret retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindIdentitySecret(ctx context.Context, exec bob.Executor, NamespacePK string, KeyPK string, cols ...string) (*IdentitySecret, error) {
	if len(cols) == 0 {
		return IdentitySecrets.Query(
			sm.Where(IdentitySecrets.Columns.Namespace.EQ(psql.Arg(NamespacePK))),
			sm.Where(IdentitySecrets.Columns.Key.EQ(psql.Arg(KeyPK))),
		).One(ctx, exec)
	}

	return IdentitySecrets.Query(
		sm.Where(IdentitySecrets.Columns.Namespace.EQ(psql.Arg(NamespacePK))),
		sm.Where(IdentitySecrets.Columns.Key.EQ(psql.Arg(KeyPK))),
		sm.Columns(IdentitySecrets.Columns.Only(cols...)),
	).One(ctx, exec)
}

// IdentitySecretExists checks the presence of a single record by primary key
func IdentitySecretExists(ctx context.Context, exec bob.Executor, NamespacePK string, KeyPK string) (bool, error) {
	return IdentitySecrets.Query(
		sm.Where(IdentitySecrets.Columns.Namespace.EQ(psql.Arg(NamespacePK))),
		sm.Where(IdentitySecrets.Columns.Key.EQ(psql.Arg(KeyPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after IdentitySecret is retrieved from the database
func (o *IdentitySecret) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IdentitySecrets.AfterSelectHooks.RunHooks(ctx, exec, IdentitySecretSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = IdentitySecrets.AfterInsertHooks.RunHooks(ctx, exec, IdentitySecretSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = IdentitySecrets.AfterUpdateHooks.RunHooks(ctx, exec, IdentitySecretSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = IdentitySecrets.AfterDeleteHooks.RunHooks(ctx, exec, IdentitySecretSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = IdentitySecrets.AfterMergeHooks.RunHooks(ctx, exec, IdentitySecretSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the IdentitySecret
func (o *IdentitySecret) primaryKeyVals() bob.Expression {
	return psql.ArgGroup(
		o.Namespace,
		o.Key,
	)
}

func (o *IdentitySecret) pkEQ() dialect.Expression {
	return psql.Group(psql.Quote("identity_secrets", "namespace"), psql.Quote("identity_secrets", "key")).EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the IdentitySecret
func (o *IdentitySecret) Update(ctx context.Context, exec bob.Executor, s *IdentitySecretSetter) error {
	v, err := IdentitySecrets.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single IdentitySecret record with an executor
func (o *IdentitySecret) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := IdentitySecrets.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the IdentitySecret using the executor
func (o *IdentitySecret) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := IdentitySecrets.Query(
		sm.Where(IdentitySecrets.Columns.Namespace.EQ(psql.Arg(o.Namespace))),
		sm.Where(IdentitySecrets.Columns.Key.EQ(psql.Arg(o.Key))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after IdentitySecretSlice is retrieved from the database
func (o IdentitySecretSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IdentitySecrets.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = IdentitySecrets.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = IdentitySecrets.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = IdentitySecrets.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = IdentitySecrets.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o IdentitySecretSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Group(psql.Quote("identity_secrets", "namespace"), psql.Quote("identity_secrets", "key")).In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o IdentitySecretSlice) copyMatchingRows(from ...*IdentitySecret) {
	for i, old := range o {
		for _, new := range from {
			if new.Namespace != old.Namespace {
				continue
			}
			if new.Key != old.Key {
				continue
			}

			o[i] = new
			break
		}
	}
}

// UpdateMod modifies an update query with "WHERE primary_key IN (o...)"
func (o IdentitySecretSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IdentitySecrets.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IdentitySecret:
				o.copyMatchingRows(retrieved)
			case []*IdentitySecret:
				o.copyMatchingRows(retrieved...)
			case IdentitySecretSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IdentitySecret or a slice of IdentitySecret
				// then run the AfterUpdateHooks on the slice
				_, err = IdentitySecrets.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o IdentitySecretSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IdentitySecrets.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IdentitySecret:
				o.copyMatchingRows(retrieved)
			case []*IdentitySecret:
				o.copyMatchingRows(retrieved...)
			case IdentitySecretSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IdentitySecret or a slice of IdentitySecret
				// then run the AfterDeleteHooks on the slice
				_, err = IdentitySecrets.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o IdentitySecretSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IdentitySecrets.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IdentitySecret:
				o.copyMatchingRows(retrieved)
			case []*IdentitySecret:
				o.copyMatchingRows(retrieved...)
			case IdentitySecretSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IdentitySecret or a slice of IdentitySecret
				// then run the AfterMergeHooks on the slice
				_, err = IdentitySecrets.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o IdentitySecretSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals IdentitySecretSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IdentitySecrets.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o IdentitySecretSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IdentitySecrets.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o IdentitySecretSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := IdentitySecrets.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type identitySecretWhere[Q psql.Filterable] struct {
	Namespace psql.WhereMod[Q, string]
	Key       psql.WhereMod[Q, string]
	Value     psql.WhereMod[Q, string]
}

func (identitySecretWhere[Q]) AliasedAs(alias string) identitySecretWhere[Q] {
	return buildIdentitySecretWhere[Q](buildIdentitySecretColumns(alias))
}

func buildIdentitySecretWhere[Q psql.Filterable](cols identitySecretColumns) identitySecretWhere[Q] {
	return identitySecretWhere[Q]{
		Namespace: psql.Where[Q, string](cols.Namespace.Expression),
		Key:       psql.Where[Q, string](cols.Key.Expression),
		Value:     psql.Where[Q, string](cols.Value.Expression),
	}
}
