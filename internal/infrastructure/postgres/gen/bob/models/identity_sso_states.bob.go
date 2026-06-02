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

// IdentitySsoState is an object representing the database table.
type IdentitySsoState struct {
	State        string    `db:"state,pk" `
	ProviderID   string    `db:"provider_id" `
	CodeVerifier string    `db:"code_verifier" `
	Nonce        string    `db:"nonce" `
	ExpiresAt    time.Time `db:"expires_at" `
}

// IdentitySsoStateSlice is an alias for a slice of pointers to IdentitySsoState.
// This should almost always be used instead of []*IdentitySsoState.
type IdentitySsoStateSlice []*IdentitySsoState

// IdentitySsoStates contains methods to work with the identity_sso_states table
var IdentitySsoStates = psql.NewTablex[*IdentitySsoState, IdentitySsoStateSlice, *IdentitySsoStateSetter]("", "identity_sso_states", buildIdentitySsoStateColumns("identity_sso_states"))

// IdentitySsoStatesQuery is a query on the identity_sso_states table
type IdentitySsoStatesQuery = *psql.ViewQuery[*IdentitySsoState, IdentitySsoStateSlice]

func buildIdentitySsoStateColumns(tableName string) identitySsoStateColumns {
	columnsExpr := expr.NewColumnsExpr(
		"state", "provider_id", "code_verifier", "nonce", "expires_at",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return identitySsoStateColumns{
		ColumnsExpr:  columnsExpr,
		tableAlias:   tableName,
		State:        buildIdentitySsoStateColumn(tableName, "state"),
		ProviderID:   buildIdentitySsoStateColumn(tableName, "provider_id"),
		CodeVerifier: buildIdentitySsoStateColumn(tableName, "code_verifier"),
		Nonce:        buildIdentitySsoStateColumn(tableName, "nonce"),
		ExpiresAt:    buildIdentitySsoStateColumn(tableName, "expires_at"),
	}
}

type identitySsoStateColumns struct {
	expr.ColumnsExpr
	tableAlias   string
	State        identitySsoStateColumn
	ProviderID   identitySsoStateColumn
	CodeVerifier identitySsoStateColumn
	Nonce        identitySsoStateColumn
	ExpiresAt    identitySsoStateColumn
}

// Alias returns the current table alias for the columns set.
func (c identitySsoStateColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (identitySsoStateColumns) AliasedAs(tableName string) identitySsoStateColumns {
	return buildIdentitySsoStateColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c identitySsoStateColumns) Unqualified() identitySsoStateColumns {
	return buildIdentitySsoStateColumns("")
}

func buildIdentitySsoStateColumn(alias, name string) identitySsoStateColumn {
	return identitySsoStateColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type identitySsoStateColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c identitySsoStateColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c identitySsoStateColumn) ShouldOmitParens() bool {
	return true
}

// IdentitySsoStateSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type IdentitySsoStateSetter struct {
	State        *string    `db:"state,pk" `
	ProviderID   *string    `db:"provider_id" `
	CodeVerifier *string    `db:"code_verifier" `
	Nonce        *string    `db:"nonce" `
	ExpiresAt    *time.Time `db:"expires_at" `
}

func (s IdentitySsoStateSetter) SetColumns() []string {
	vals := make([]string, 0, 5)
	if s.State != nil {
		vals = append(vals, "state")
	}
	if s.ProviderID != nil {
		vals = append(vals, "provider_id")
	}
	if s.CodeVerifier != nil {
		vals = append(vals, "code_verifier")
	}
	if s.Nonce != nil {
		vals = append(vals, "nonce")
	}
	if s.ExpiresAt != nil {
		vals = append(vals, "expires_at")
	}
	return vals
}

func (s IdentitySsoStateSetter) Overwrite(t *IdentitySsoState) {
	if s.State != nil {
		t.State = func() string {
			if s.State == nil {
				return *new(string)
			}
			return *s.State
		}()
	}
	if s.ProviderID != nil {
		t.ProviderID = func() string {
			if s.ProviderID == nil {
				return *new(string)
			}
			return *s.ProviderID
		}()
	}
	if s.CodeVerifier != nil {
		t.CodeVerifier = func() string {
			if s.CodeVerifier == nil {
				return *new(string)
			}
			return *s.CodeVerifier
		}()
	}
	if s.Nonce != nil {
		t.Nonce = func() string {
			if s.Nonce == nil {
				return *new(string)
			}
			return *s.Nonce
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

func (s *IdentitySsoStateSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return IdentitySsoStates.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 5)
		if s.State != nil {
			vals[0] = psql.Arg(func() string {
				if s.State == nil {
					return *new(string)
				}
				return *s.State
			}())
		} else {
			vals[0] = psql.Raw("DEFAULT")
		}

		if s.ProviderID != nil {
			vals[1] = psql.Arg(func() string {
				if s.ProviderID == nil {
					return *new(string)
				}
				return *s.ProviderID
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.CodeVerifier != nil {
			vals[2] = psql.Arg(func() string {
				if s.CodeVerifier == nil {
					return *new(string)
				}
				return *s.CodeVerifier
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		if s.Nonce != nil {
			vals[3] = psql.Arg(func() string {
				if s.Nonce == nil {
					return *new(string)
				}
				return *s.Nonce
			}())
		} else {
			vals[3] = psql.Raw("DEFAULT")
		}

		if s.ExpiresAt != nil {
			vals[4] = psql.Arg(func() time.Time {
				if s.ExpiresAt == nil {
					return *new(time.Time)
				}
				return *s.ExpiresAt
			}())
		} else {
			vals[4] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s IdentitySsoStateSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s IdentitySsoStateSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 5)

	if s.State != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "state")...),
			psql.Arg(s.State),
		}})
	}

	if s.ProviderID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "provider_id")...),
			psql.Arg(s.ProviderID),
		}})
	}

	if s.CodeVerifier != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "code_verifier")...),
			psql.Arg(s.CodeVerifier),
		}})
	}

	if s.Nonce != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "nonce")...),
			psql.Arg(s.Nonce),
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

// FindIdentitySsoState retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindIdentitySsoState(ctx context.Context, exec bob.Executor, StatePK string, cols ...string) (*IdentitySsoState, error) {
	if len(cols) == 0 {
		return IdentitySsoStates.Query(
			sm.Where(IdentitySsoStates.Columns.State.EQ(psql.Arg(StatePK))),
		).One(ctx, exec)
	}

	return IdentitySsoStates.Query(
		sm.Where(IdentitySsoStates.Columns.State.EQ(psql.Arg(StatePK))),
		sm.Columns(IdentitySsoStates.Columns.Only(cols...)),
	).One(ctx, exec)
}

// IdentitySsoStateExists checks the presence of a single record by primary key
func IdentitySsoStateExists(ctx context.Context, exec bob.Executor, StatePK string) (bool, error) {
	return IdentitySsoStates.Query(
		sm.Where(IdentitySsoStates.Columns.State.EQ(psql.Arg(StatePK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after IdentitySsoState is retrieved from the database
func (o *IdentitySsoState) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IdentitySsoStates.AfterSelectHooks.RunHooks(ctx, exec, IdentitySsoStateSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = IdentitySsoStates.AfterInsertHooks.RunHooks(ctx, exec, IdentitySsoStateSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = IdentitySsoStates.AfterUpdateHooks.RunHooks(ctx, exec, IdentitySsoStateSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = IdentitySsoStates.AfterDeleteHooks.RunHooks(ctx, exec, IdentitySsoStateSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = IdentitySsoStates.AfterMergeHooks.RunHooks(ctx, exec, IdentitySsoStateSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the IdentitySsoState
func (o *IdentitySsoState) primaryKeyVals() bob.Expression {
	return psql.Arg(o.State)
}

func (o *IdentitySsoState) pkEQ() dialect.Expression {
	return psql.Quote("identity_sso_states", "state").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the IdentitySsoState
func (o *IdentitySsoState) Update(ctx context.Context, exec bob.Executor, s *IdentitySsoStateSetter) error {
	v, err := IdentitySsoStates.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single IdentitySsoState record with an executor
func (o *IdentitySsoState) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := IdentitySsoStates.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the IdentitySsoState using the executor
func (o *IdentitySsoState) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := IdentitySsoStates.Query(
		sm.Where(IdentitySsoStates.Columns.State.EQ(psql.Arg(o.State))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after IdentitySsoStateSlice is retrieved from the database
func (o IdentitySsoStateSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IdentitySsoStates.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = IdentitySsoStates.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = IdentitySsoStates.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = IdentitySsoStates.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = IdentitySsoStates.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o IdentitySsoStateSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("identity_sso_states", "state").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o IdentitySsoStateSlice) copyMatchingRows(from ...*IdentitySsoState) {
	for i, old := range o {
		for _, new := range from {
			if new.State != old.State {
				continue
			}

			o[i] = new
			break
		}
	}
}

// UpdateMod modifies an update query with "WHERE primary_key IN (o...)"
func (o IdentitySsoStateSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IdentitySsoStates.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IdentitySsoState:
				o.copyMatchingRows(retrieved)
			case []*IdentitySsoState:
				o.copyMatchingRows(retrieved...)
			case IdentitySsoStateSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IdentitySsoState or a slice of IdentitySsoState
				// then run the AfterUpdateHooks on the slice
				_, err = IdentitySsoStates.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o IdentitySsoStateSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IdentitySsoStates.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IdentitySsoState:
				o.copyMatchingRows(retrieved)
			case []*IdentitySsoState:
				o.copyMatchingRows(retrieved...)
			case IdentitySsoStateSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IdentitySsoState or a slice of IdentitySsoState
				// then run the AfterDeleteHooks on the slice
				_, err = IdentitySsoStates.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o IdentitySsoStateSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IdentitySsoStates.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IdentitySsoState:
				o.copyMatchingRows(retrieved)
			case []*IdentitySsoState:
				o.copyMatchingRows(retrieved...)
			case IdentitySsoStateSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IdentitySsoState or a slice of IdentitySsoState
				// then run the AfterMergeHooks on the slice
				_, err = IdentitySsoStates.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o IdentitySsoStateSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals IdentitySsoStateSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IdentitySsoStates.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o IdentitySsoStateSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IdentitySsoStates.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o IdentitySsoStateSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := IdentitySsoStates.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type identitySsoStateWhere[Q psql.Filterable] struct {
	State        psql.WhereMod[Q, string]
	ProviderID   psql.WhereMod[Q, string]
	CodeVerifier psql.WhereMod[Q, string]
	Nonce        psql.WhereMod[Q, string]
	ExpiresAt    psql.WhereMod[Q, time.Time]
}

func (identitySsoStateWhere[Q]) AliasedAs(alias string) identitySsoStateWhere[Q] {
	return buildIdentitySsoStateWhere[Q](buildIdentitySsoStateColumns(alias))
}

func buildIdentitySsoStateWhere[Q psql.Filterable](cols identitySsoStateColumns) identitySsoStateWhere[Q] {
	return identitySsoStateWhere[Q]{
		State:        psql.Where[Q, string](cols.State.Expression),
		ProviderID:   psql.Where[Q, string](cols.ProviderID.Expression),
		CodeVerifier: psql.Where[Q, string](cols.CodeVerifier.Expression),
		Nonce:        psql.Where[Q, string](cols.Nonce.Expression),
		ExpiresAt:    psql.Where[Q, time.Time](cols.ExpiresAt.Expression),
	}
}
