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

// IamTenant is an object representing the database table.
type IamTenant struct {
	ID             string          `db:"id,pk" `
	Slug           string          `db:"slug" `
	OwnerAccountID string          `db:"owner_account_id" `
	CreatedAt      time.Time       `db:"created_at" `
	UpdatedAt      time.Time       `db:"updated_at" `
	Data           json.RawMessage `db:"data" `
}

// IamTenantSlice is an alias for a slice of pointers to IamTenant.
// This should almost always be used instead of []*IamTenant.
type IamTenantSlice []*IamTenant

// IamTenants contains methods to work with the iam_tenants table
var IamTenants = psql.NewTablex[*IamTenant, IamTenantSlice, *IamTenantSetter]("", "iam_tenants", buildIamTenantColumns("iam_tenants"))

// IamTenantsQuery is a query on the iam_tenants table
type IamTenantsQuery = *psql.ViewQuery[*IamTenant, IamTenantSlice]

func buildIamTenantColumns(tableName string) iamTenantColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "slug", "owner_account_id", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return iamTenantColumns{
		ColumnsExpr:    columnsExpr,
		tableAlias:     tableName,
		ID:             buildIamTenantColumn(tableName, "id"),
		Slug:           buildIamTenantColumn(tableName, "slug"),
		OwnerAccountID: buildIamTenantColumn(tableName, "owner_account_id"),
		CreatedAt:      buildIamTenantColumn(tableName, "created_at"),
		UpdatedAt:      buildIamTenantColumn(tableName, "updated_at"),
		Data:           buildIamTenantColumn(tableName, "data"),
	}
}

type iamTenantColumns struct {
	expr.ColumnsExpr
	tableAlias     string
	ID             iamTenantColumn
	Slug           iamTenantColumn
	OwnerAccountID iamTenantColumn
	CreatedAt      iamTenantColumn
	UpdatedAt      iamTenantColumn
	Data           iamTenantColumn
}

// Alias returns the current table alias for the columns set.
func (c iamTenantColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (iamTenantColumns) AliasedAs(tableName string) iamTenantColumns {
	return buildIamTenantColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c iamTenantColumns) Unqualified() iamTenantColumns {
	return buildIamTenantColumns("")
}

func buildIamTenantColumn(alias, name string) iamTenantColumn {
	return iamTenantColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type iamTenantColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c iamTenantColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c iamTenantColumn) ShouldOmitParens() bool {
	return true
}

// IamTenantSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type IamTenantSetter struct {
	ID             *string          `db:"id,pk" `
	Slug           *string          `db:"slug" `
	OwnerAccountID *string          `db:"owner_account_id" `
	CreatedAt      *time.Time       `db:"created_at" `
	UpdatedAt      *time.Time       `db:"updated_at" `
	Data           *json.RawMessage `db:"data" `
}

func (s IamTenantSetter) SetColumns() []string {
	vals := make([]string, 0, 6)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.Slug != nil {
		vals = append(vals, "slug")
	}
	if s.OwnerAccountID != nil {
		vals = append(vals, "owner_account_id")
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

func (s IamTenantSetter) Overwrite(t *IamTenant) {
	if s.ID != nil {
		t.ID = func() string {
			if s.ID == nil {
				return *new(string)
			}
			return *s.ID
		}()
	}
	if s.Slug != nil {
		t.Slug = func() string {
			if s.Slug == nil {
				return *new(string)
			}
			return *s.Slug
		}()
	}
	if s.OwnerAccountID != nil {
		t.OwnerAccountID = func() string {
			if s.OwnerAccountID == nil {
				return *new(string)
			}
			return *s.OwnerAccountID
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

func (s *IamTenantSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return IamTenants.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

		if s.Slug != nil {
			vals[1] = psql.Arg(func() string {
				if s.Slug == nil {
					return *new(string)
				}
				return *s.Slug
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.OwnerAccountID != nil {
			vals[2] = psql.Arg(func() string {
				if s.OwnerAccountID == nil {
					return *new(string)
				}
				return *s.OwnerAccountID
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

func (s IamTenantSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s IamTenantSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 6)

	if s.ID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "id")...),
			psql.Arg(s.ID),
		}})
	}

	if s.Slug != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "slug")...),
			psql.Arg(s.Slug),
		}})
	}

	if s.OwnerAccountID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "owner_account_id")...),
			psql.Arg(s.OwnerAccountID),
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

// FindIamTenant retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindIamTenant(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*IamTenant, error) {
	if len(cols) == 0 {
		return IamTenants.Query(
			sm.Where(IamTenants.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return IamTenants.Query(
		sm.Where(IamTenants.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(IamTenants.Columns.Only(cols...)),
	).One(ctx, exec)
}

// IamTenantExists checks the presence of a single record by primary key
func IamTenantExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return IamTenants.Query(
		sm.Where(IamTenants.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after IamTenant is retrieved from the database
func (o *IamTenant) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamTenants.AfterSelectHooks.RunHooks(ctx, exec, IamTenantSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = IamTenants.AfterInsertHooks.RunHooks(ctx, exec, IamTenantSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = IamTenants.AfterUpdateHooks.RunHooks(ctx, exec, IamTenantSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = IamTenants.AfterDeleteHooks.RunHooks(ctx, exec, IamTenantSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = IamTenants.AfterMergeHooks.RunHooks(ctx, exec, IamTenantSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the IamTenant
func (o *IamTenant) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *IamTenant) pkEQ() dialect.Expression {
	return psql.Quote("iam_tenants", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the IamTenant
func (o *IamTenant) Update(ctx context.Context, exec bob.Executor, s *IamTenantSetter) error {
	v, err := IamTenants.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single IamTenant record with an executor
func (o *IamTenant) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := IamTenants.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the IamTenant using the executor
func (o *IamTenant) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := IamTenants.Query(
		sm.Where(IamTenants.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after IamTenantSlice is retrieved from the database
func (o IamTenantSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamTenants.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = IamTenants.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = IamTenants.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = IamTenants.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = IamTenants.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o IamTenantSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("iam_tenants", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o IamTenantSlice) copyMatchingRows(from ...*IamTenant) {
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
func (o IamTenantSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamTenants.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamTenant:
				o.copyMatchingRows(retrieved)
			case []*IamTenant:
				o.copyMatchingRows(retrieved...)
			case IamTenantSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamTenant or a slice of IamTenant
				// then run the AfterUpdateHooks on the slice
				_, err = IamTenants.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o IamTenantSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamTenants.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamTenant:
				o.copyMatchingRows(retrieved)
			case []*IamTenant:
				o.copyMatchingRows(retrieved...)
			case IamTenantSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamTenant or a slice of IamTenant
				// then run the AfterDeleteHooks on the slice
				_, err = IamTenants.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o IamTenantSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamTenants.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamTenant:
				o.copyMatchingRows(retrieved)
			case []*IamTenant:
				o.copyMatchingRows(retrieved...)
			case IamTenantSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamTenant or a slice of IamTenant
				// then run the AfterMergeHooks on the slice
				_, err = IamTenants.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o IamTenantSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals IamTenantSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamTenants.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o IamTenantSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamTenants.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o IamTenantSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := IamTenants.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type iamTenantWhere[Q psql.Filterable] struct {
	ID             psql.WhereMod[Q, string]
	Slug           psql.WhereMod[Q, string]
	OwnerAccountID psql.WhereMod[Q, string]
	CreatedAt      psql.WhereMod[Q, time.Time]
	UpdatedAt      psql.WhereMod[Q, time.Time]
	Data           psql.WhereMod[Q, json.RawMessage]
}

func (iamTenantWhere[Q]) AliasedAs(alias string) iamTenantWhere[Q] {
	return buildIamTenantWhere[Q](buildIamTenantColumns(alias))
}

func buildIamTenantWhere[Q psql.Filterable](cols iamTenantColumns) iamTenantWhere[Q] {
	return iamTenantWhere[Q]{
		ID:             psql.Where[Q, string](cols.ID.Expression),
		Slug:           psql.Where[Q, string](cols.Slug.Expression),
		OwnerAccountID: psql.Where[Q, string](cols.OwnerAccountID.Expression),
		CreatedAt:      psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt:      psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:           psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
