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

// IdentityRefreshSession is an object representing the database table.
type IdentityRefreshSession struct {
	ID        string    `db:"id,pk" `
	AccountID string    `db:"account_id" `
	ExpiresAt time.Time `db:"expires_at" `
}

// IdentityRefreshSessionSlice is an alias for a slice of pointers to IdentityRefreshSession.
// This should almost always be used instead of []*IdentityRefreshSession.
type IdentityRefreshSessionSlice []*IdentityRefreshSession

// IdentityRefreshSessions contains methods to work with the identity_refresh_sessions table
var IdentityRefreshSessions = psql.NewTablex[*IdentityRefreshSession, IdentityRefreshSessionSlice, *IdentityRefreshSessionSetter]("", "identity_refresh_sessions", buildIdentityRefreshSessionColumns("identity_refresh_sessions"))

// IdentityRefreshSessionsQuery is a query on the identity_refresh_sessions table
type IdentityRefreshSessionsQuery = *psql.ViewQuery[*IdentityRefreshSession, IdentityRefreshSessionSlice]

func buildIdentityRefreshSessionColumns(tableName string) identityRefreshSessionColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "account_id", "expires_at",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return identityRefreshSessionColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildIdentityRefreshSessionColumn(tableName, "id"),
		AccountID:   buildIdentityRefreshSessionColumn(tableName, "account_id"),
		ExpiresAt:   buildIdentityRefreshSessionColumn(tableName, "expires_at"),
	}
}

type identityRefreshSessionColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         identityRefreshSessionColumn
	AccountID  identityRefreshSessionColumn
	ExpiresAt  identityRefreshSessionColumn
}

// Alias returns the current table alias for the columns set.
func (c identityRefreshSessionColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (identityRefreshSessionColumns) AliasedAs(tableName string) identityRefreshSessionColumns {
	return buildIdentityRefreshSessionColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c identityRefreshSessionColumns) Unqualified() identityRefreshSessionColumns {
	return buildIdentityRefreshSessionColumns("")
}

func buildIdentityRefreshSessionColumn(alias, name string) identityRefreshSessionColumn {
	return identityRefreshSessionColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type identityRefreshSessionColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c identityRefreshSessionColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c identityRefreshSessionColumn) ShouldOmitParens() bool {
	return true
}

// IdentityRefreshSessionSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type IdentityRefreshSessionSetter struct {
	ID        *string    `db:"id,pk" `
	AccountID *string    `db:"account_id" `
	ExpiresAt *time.Time `db:"expires_at" `
}

func (s IdentityRefreshSessionSetter) SetColumns() []string {
	vals := make([]string, 0, 3)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.AccountID != nil {
		vals = append(vals, "account_id")
	}
	if s.ExpiresAt != nil {
		vals = append(vals, "expires_at")
	}
	return vals
}

func (s IdentityRefreshSessionSetter) Overwrite(t *IdentityRefreshSession) {
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
	if s.ExpiresAt != nil {
		t.ExpiresAt = func() time.Time {
			if s.ExpiresAt == nil {
				return *new(time.Time)
			}
			return *s.ExpiresAt
		}()
	}
}

func (s *IdentityRefreshSessionSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return IdentityRefreshSessions.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 3)
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

		if s.ExpiresAt != nil {
			vals[2] = psql.Arg(func() time.Time {
				if s.ExpiresAt == nil {
					return *new(time.Time)
				}
				return *s.ExpiresAt
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s IdentityRefreshSessionSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s IdentityRefreshSessionSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 3)

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

	if s.ExpiresAt != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "expires_at")...),
			psql.Arg(s.ExpiresAt),
		}})
	}

	return exprs
}

// FindIdentityRefreshSession retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindIdentityRefreshSession(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*IdentityRefreshSession, error) {
	if len(cols) == 0 {
		return IdentityRefreshSessions.Query(
			sm.Where(IdentityRefreshSessions.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return IdentityRefreshSessions.Query(
		sm.Where(IdentityRefreshSessions.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(IdentityRefreshSessions.Columns.Only(cols...)),
	).One(ctx, exec)
}

// IdentityRefreshSessionExists checks the presence of a single record by primary key
func IdentityRefreshSessionExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return IdentityRefreshSessions.Query(
		sm.Where(IdentityRefreshSessions.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after IdentityRefreshSession is retrieved from the database
func (o *IdentityRefreshSession) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IdentityRefreshSessions.AfterSelectHooks.RunHooks(ctx, exec, IdentityRefreshSessionSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = IdentityRefreshSessions.AfterInsertHooks.RunHooks(ctx, exec, IdentityRefreshSessionSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = IdentityRefreshSessions.AfterUpdateHooks.RunHooks(ctx, exec, IdentityRefreshSessionSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = IdentityRefreshSessions.AfterDeleteHooks.RunHooks(ctx, exec, IdentityRefreshSessionSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = IdentityRefreshSessions.AfterMergeHooks.RunHooks(ctx, exec, IdentityRefreshSessionSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the IdentityRefreshSession
func (o *IdentityRefreshSession) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *IdentityRefreshSession) pkEQ() dialect.Expression {
	return psql.Quote("identity_refresh_sessions", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the IdentityRefreshSession
func (o *IdentityRefreshSession) Update(ctx context.Context, exec bob.Executor, s *IdentityRefreshSessionSetter) error {
	v, err := IdentityRefreshSessions.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single IdentityRefreshSession record with an executor
func (o *IdentityRefreshSession) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := IdentityRefreshSessions.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the IdentityRefreshSession using the executor
func (o *IdentityRefreshSession) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := IdentityRefreshSessions.Query(
		sm.Where(IdentityRefreshSessions.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after IdentityRefreshSessionSlice is retrieved from the database
func (o IdentityRefreshSessionSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IdentityRefreshSessions.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = IdentityRefreshSessions.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = IdentityRefreshSessions.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = IdentityRefreshSessions.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = IdentityRefreshSessions.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o IdentityRefreshSessionSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("identity_refresh_sessions", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o IdentityRefreshSessionSlice) copyMatchingRows(from ...*IdentityRefreshSession) {
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
func (o IdentityRefreshSessionSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IdentityRefreshSessions.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IdentityRefreshSession:
				o.copyMatchingRows(retrieved)
			case []*IdentityRefreshSession:
				o.copyMatchingRows(retrieved...)
			case IdentityRefreshSessionSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IdentityRefreshSession or a slice of IdentityRefreshSession
				// then run the AfterUpdateHooks on the slice
				_, err = IdentityRefreshSessions.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o IdentityRefreshSessionSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IdentityRefreshSessions.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IdentityRefreshSession:
				o.copyMatchingRows(retrieved)
			case []*IdentityRefreshSession:
				o.copyMatchingRows(retrieved...)
			case IdentityRefreshSessionSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IdentityRefreshSession or a slice of IdentityRefreshSession
				// then run the AfterDeleteHooks on the slice
				_, err = IdentityRefreshSessions.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o IdentityRefreshSessionSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IdentityRefreshSessions.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IdentityRefreshSession:
				o.copyMatchingRows(retrieved)
			case []*IdentityRefreshSession:
				o.copyMatchingRows(retrieved...)
			case IdentityRefreshSessionSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IdentityRefreshSession or a slice of IdentityRefreshSession
				// then run the AfterMergeHooks on the slice
				_, err = IdentityRefreshSessions.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o IdentityRefreshSessionSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals IdentityRefreshSessionSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IdentityRefreshSessions.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o IdentityRefreshSessionSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IdentityRefreshSessions.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o IdentityRefreshSessionSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := IdentityRefreshSessions.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type identityRefreshSessionWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	AccountID psql.WhereMod[Q, string]
	ExpiresAt psql.WhereMod[Q, time.Time]
}

func (identityRefreshSessionWhere[Q]) AliasedAs(alias string) identityRefreshSessionWhere[Q] {
	return buildIdentityRefreshSessionWhere[Q](buildIdentityRefreshSessionColumns(alias))
}

func buildIdentityRefreshSessionWhere[Q psql.Filterable](cols identityRefreshSessionColumns) identityRefreshSessionWhere[Q] {
	return identityRefreshSessionWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		AccountID: psql.Where[Q, string](cols.AccountID.Expression),
		ExpiresAt: psql.Where[Q, time.Time](cols.ExpiresAt.Expression),
	}
}
