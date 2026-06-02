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

// IamMembership is an object representing the database table.
type IamMembership struct {
	ID        string          `db:"id,pk" `
	AccountID string          `db:"account_id" `
	TenantID  string          `db:"tenant_id" `
	CreatedAt time.Time       `db:"created_at" `
	UpdatedAt time.Time       `db:"updated_at" `
	Data      json.RawMessage `db:"data" `
}

// IamMembershipSlice is an alias for a slice of pointers to IamMembership.
// This should almost always be used instead of []*IamMembership.
type IamMembershipSlice []*IamMembership

// IamMemberships contains methods to work with the iam_memberships table
var IamMemberships = psql.NewTablex[*IamMembership, IamMembershipSlice, *IamMembershipSetter]("", "iam_memberships", buildIamMembershipColumns("iam_memberships"))

// IamMembershipsQuery is a query on the iam_memberships table
type IamMembershipsQuery = *psql.ViewQuery[*IamMembership, IamMembershipSlice]

func buildIamMembershipColumns(tableName string) iamMembershipColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "account_id", "tenant_id", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return iamMembershipColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildIamMembershipColumn(tableName, "id"),
		AccountID:   buildIamMembershipColumn(tableName, "account_id"),
		TenantID:    buildIamMembershipColumn(tableName, "tenant_id"),
		CreatedAt:   buildIamMembershipColumn(tableName, "created_at"),
		UpdatedAt:   buildIamMembershipColumn(tableName, "updated_at"),
		Data:        buildIamMembershipColumn(tableName, "data"),
	}
}

type iamMembershipColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         iamMembershipColumn
	AccountID  iamMembershipColumn
	TenantID   iamMembershipColumn
	CreatedAt  iamMembershipColumn
	UpdatedAt  iamMembershipColumn
	Data       iamMembershipColumn
}

// Alias returns the current table alias for the columns set.
func (c iamMembershipColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (iamMembershipColumns) AliasedAs(tableName string) iamMembershipColumns {
	return buildIamMembershipColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c iamMembershipColumns) Unqualified() iamMembershipColumns {
	return buildIamMembershipColumns("")
}

func buildIamMembershipColumn(alias, name string) iamMembershipColumn {
	return iamMembershipColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type iamMembershipColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c iamMembershipColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c iamMembershipColumn) ShouldOmitParens() bool {
	return true
}

// IamMembershipSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type IamMembershipSetter struct {
	ID        *string          `db:"id,pk" `
	AccountID *string          `db:"account_id" `
	TenantID  *string          `db:"tenant_id" `
	CreatedAt *time.Time       `db:"created_at" `
	UpdatedAt *time.Time       `db:"updated_at" `
	Data      *json.RawMessage `db:"data" `
}

func (s IamMembershipSetter) SetColumns() []string {
	vals := make([]string, 0, 6)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.AccountID != nil {
		vals = append(vals, "account_id")
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

func (s IamMembershipSetter) Overwrite(t *IamMembership) {
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

func (s *IamMembershipSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return IamMemberships.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

func (s IamMembershipSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s IamMembershipSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 6)

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

// FindIamMembership retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindIamMembership(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*IamMembership, error) {
	if len(cols) == 0 {
		return IamMemberships.Query(
			sm.Where(IamMemberships.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return IamMemberships.Query(
		sm.Where(IamMemberships.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(IamMemberships.Columns.Only(cols...)),
	).One(ctx, exec)
}

// IamMembershipExists checks the presence of a single record by primary key
func IamMembershipExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return IamMemberships.Query(
		sm.Where(IamMemberships.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after IamMembership is retrieved from the database
func (o *IamMembership) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamMemberships.AfterSelectHooks.RunHooks(ctx, exec, IamMembershipSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = IamMemberships.AfterInsertHooks.RunHooks(ctx, exec, IamMembershipSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = IamMemberships.AfterUpdateHooks.RunHooks(ctx, exec, IamMembershipSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = IamMemberships.AfterDeleteHooks.RunHooks(ctx, exec, IamMembershipSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = IamMemberships.AfterMergeHooks.RunHooks(ctx, exec, IamMembershipSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the IamMembership
func (o *IamMembership) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *IamMembership) pkEQ() dialect.Expression {
	return psql.Quote("iam_memberships", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the IamMembership
func (o *IamMembership) Update(ctx context.Context, exec bob.Executor, s *IamMembershipSetter) error {
	v, err := IamMemberships.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single IamMembership record with an executor
func (o *IamMembership) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := IamMemberships.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the IamMembership using the executor
func (o *IamMembership) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := IamMemberships.Query(
		sm.Where(IamMemberships.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after IamMembershipSlice is retrieved from the database
func (o IamMembershipSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamMemberships.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = IamMemberships.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = IamMemberships.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = IamMemberships.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = IamMemberships.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o IamMembershipSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("iam_memberships", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o IamMembershipSlice) copyMatchingRows(from ...*IamMembership) {
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
func (o IamMembershipSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamMemberships.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamMembership:
				o.copyMatchingRows(retrieved)
			case []*IamMembership:
				o.copyMatchingRows(retrieved...)
			case IamMembershipSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamMembership or a slice of IamMembership
				// then run the AfterUpdateHooks on the slice
				_, err = IamMemberships.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o IamMembershipSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamMemberships.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamMembership:
				o.copyMatchingRows(retrieved)
			case []*IamMembership:
				o.copyMatchingRows(retrieved...)
			case IamMembershipSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamMembership or a slice of IamMembership
				// then run the AfterDeleteHooks on the slice
				_, err = IamMemberships.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o IamMembershipSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamMemberships.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamMembership:
				o.copyMatchingRows(retrieved)
			case []*IamMembership:
				o.copyMatchingRows(retrieved...)
			case IamMembershipSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamMembership or a slice of IamMembership
				// then run the AfterMergeHooks on the slice
				_, err = IamMemberships.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o IamMembershipSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals IamMembershipSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamMemberships.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o IamMembershipSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamMemberships.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o IamMembershipSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := IamMemberships.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type iamMembershipWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	AccountID psql.WhereMod[Q, string]
	TenantID  psql.WhereMod[Q, string]
	CreatedAt psql.WhereMod[Q, time.Time]
	UpdatedAt psql.WhereMod[Q, time.Time]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (iamMembershipWhere[Q]) AliasedAs(alias string) iamMembershipWhere[Q] {
	return buildIamMembershipWhere[Q](buildIamMembershipColumns(alias))
}

func buildIamMembershipWhere[Q psql.Filterable](cols iamMembershipColumns) iamMembershipWhere[Q] {
	return iamMembershipWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		AccountID: psql.Where[Q, string](cols.AccountID.Expression),
		TenantID:  psql.Where[Q, string](cols.TenantID.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
