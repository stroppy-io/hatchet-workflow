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

// IamRole is an object representing the database table.
type IamRole struct {
	ID        string          `db:"id,pk" `
	TenantID  string          `db:"tenant_id" `
	CreatedAt time.Time       `db:"created_at" `
	UpdatedAt time.Time       `db:"updated_at" `
	Data      json.RawMessage `db:"data" `
}

// IamRoleSlice is an alias for a slice of pointers to IamRole.
// This should almost always be used instead of []*IamRole.
type IamRoleSlice []*IamRole

// IamRoles contains methods to work with the iam_roles table
var IamRoles = psql.NewTablex[*IamRole, IamRoleSlice, *IamRoleSetter]("", "iam_roles", buildIamRoleColumns("iam_roles"))

// IamRolesQuery is a query on the iam_roles table
type IamRolesQuery = *psql.ViewQuery[*IamRole, IamRoleSlice]

func buildIamRoleColumns(tableName string) iamRoleColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "tenant_id", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return iamRoleColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildIamRoleColumn(tableName, "id"),
		TenantID:    buildIamRoleColumn(tableName, "tenant_id"),
		CreatedAt:   buildIamRoleColumn(tableName, "created_at"),
		UpdatedAt:   buildIamRoleColumn(tableName, "updated_at"),
		Data:        buildIamRoleColumn(tableName, "data"),
	}
}

type iamRoleColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         iamRoleColumn
	TenantID   iamRoleColumn
	CreatedAt  iamRoleColumn
	UpdatedAt  iamRoleColumn
	Data       iamRoleColumn
}

// Alias returns the current table alias for the columns set.
func (c iamRoleColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (iamRoleColumns) AliasedAs(tableName string) iamRoleColumns {
	return buildIamRoleColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c iamRoleColumns) Unqualified() iamRoleColumns {
	return buildIamRoleColumns("")
}

func buildIamRoleColumn(alias, name string) iamRoleColumn {
	return iamRoleColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type iamRoleColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c iamRoleColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c iamRoleColumn) ShouldOmitParens() bool {
	return true
}

// IamRoleSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type IamRoleSetter struct {
	ID        *string          `db:"id,pk" `
	TenantID  *string          `db:"tenant_id" `
	CreatedAt *time.Time       `db:"created_at" `
	UpdatedAt *time.Time       `db:"updated_at" `
	Data      *json.RawMessage `db:"data" `
}

func (s IamRoleSetter) SetColumns() []string {
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

func (s IamRoleSetter) Overwrite(t *IamRole) {
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

func (s *IamRoleSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return IamRoles.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

func (s IamRoleSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s IamRoleSetter) Expressions(prefix ...string) []bob.Expression {
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

// FindIamRole retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindIamRole(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*IamRole, error) {
	if len(cols) == 0 {
		return IamRoles.Query(
			sm.Where(IamRoles.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return IamRoles.Query(
		sm.Where(IamRoles.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(IamRoles.Columns.Only(cols...)),
	).One(ctx, exec)
}

// IamRoleExists checks the presence of a single record by primary key
func IamRoleExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return IamRoles.Query(
		sm.Where(IamRoles.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after IamRole is retrieved from the database
func (o *IamRole) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamRoles.AfterSelectHooks.RunHooks(ctx, exec, IamRoleSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = IamRoles.AfterInsertHooks.RunHooks(ctx, exec, IamRoleSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = IamRoles.AfterUpdateHooks.RunHooks(ctx, exec, IamRoleSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = IamRoles.AfterDeleteHooks.RunHooks(ctx, exec, IamRoleSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = IamRoles.AfterMergeHooks.RunHooks(ctx, exec, IamRoleSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the IamRole
func (o *IamRole) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *IamRole) pkEQ() dialect.Expression {
	return psql.Quote("iam_roles", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the IamRole
func (o *IamRole) Update(ctx context.Context, exec bob.Executor, s *IamRoleSetter) error {
	v, err := IamRoles.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single IamRole record with an executor
func (o *IamRole) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := IamRoles.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the IamRole using the executor
func (o *IamRole) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := IamRoles.Query(
		sm.Where(IamRoles.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after IamRoleSlice is retrieved from the database
func (o IamRoleSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamRoles.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = IamRoles.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = IamRoles.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = IamRoles.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = IamRoles.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o IamRoleSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("iam_roles", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o IamRoleSlice) copyMatchingRows(from ...*IamRole) {
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
func (o IamRoleSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamRoles.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamRole:
				o.copyMatchingRows(retrieved)
			case []*IamRole:
				o.copyMatchingRows(retrieved...)
			case IamRoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamRole or a slice of IamRole
				// then run the AfterUpdateHooks on the slice
				_, err = IamRoles.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o IamRoleSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamRoles.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamRole:
				o.copyMatchingRows(retrieved)
			case []*IamRole:
				o.copyMatchingRows(retrieved...)
			case IamRoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamRole or a slice of IamRole
				// then run the AfterDeleteHooks on the slice
				_, err = IamRoles.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o IamRoleSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamRoles.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamRole:
				o.copyMatchingRows(retrieved)
			case []*IamRole:
				o.copyMatchingRows(retrieved...)
			case IamRoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamRole or a slice of IamRole
				// then run the AfterMergeHooks on the slice
				_, err = IamRoles.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o IamRoleSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals IamRoleSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamRoles.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o IamRoleSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamRoles.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o IamRoleSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := IamRoles.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type iamRoleWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	TenantID  psql.WhereMod[Q, string]
	CreatedAt psql.WhereMod[Q, time.Time]
	UpdatedAt psql.WhereMod[Q, time.Time]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (iamRoleWhere[Q]) AliasedAs(alias string) iamRoleWhere[Q] {
	return buildIamRoleWhere[Q](buildIamRoleColumns(alias))
}

func buildIamRoleWhere[Q psql.Filterable](cols iamRoleColumns) iamRoleWhere[Q] {
	return iamRoleWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		TenantID:  psql.Where[Q, string](cols.TenantID.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
