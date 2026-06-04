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

// RegistrationRequest is an object representing the database table.
type RegistrationRequest struct {
	ID        string          `db:"id,pk" `
	Email     string          `db:"email" `
	Status    string          `db:"status" `
	CreatedAt time.Time       `db:"created_at" `
	UpdatedAt time.Time       `db:"updated_at" `
	Data      json.RawMessage `db:"data" `
}

// RegistrationRequestSlice is an alias for a slice of pointers to RegistrationRequest.
// This should almost always be used instead of []*RegistrationRequest.
type RegistrationRequestSlice []*RegistrationRequest

// RegistrationRequests contains methods to work with the registration_requests table
var RegistrationRequests = psql.NewTablex[*RegistrationRequest, RegistrationRequestSlice, *RegistrationRequestSetter]("", "registration_requests", buildRegistrationRequestColumns("registration_requests"))

// RegistrationRequestsQuery is a query on the registration_requests table
type RegistrationRequestsQuery = *psql.ViewQuery[*RegistrationRequest, RegistrationRequestSlice]

func buildRegistrationRequestColumns(tableName string) registrationRequestColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "email", "status", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return registrationRequestColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildRegistrationRequestColumn(tableName, "id"),
		Email:       buildRegistrationRequestColumn(tableName, "email"),
		Status:      buildRegistrationRequestColumn(tableName, "status"),
		CreatedAt:   buildRegistrationRequestColumn(tableName, "created_at"),
		UpdatedAt:   buildRegistrationRequestColumn(tableName, "updated_at"),
		Data:        buildRegistrationRequestColumn(tableName, "data"),
	}
}

type registrationRequestColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         registrationRequestColumn
	Email      registrationRequestColumn
	Status     registrationRequestColumn
	CreatedAt  registrationRequestColumn
	UpdatedAt  registrationRequestColumn
	Data       registrationRequestColumn
}

// Alias returns the current table alias for the columns set.
func (c registrationRequestColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (registrationRequestColumns) AliasedAs(tableName string) registrationRequestColumns {
	return buildRegistrationRequestColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c registrationRequestColumns) Unqualified() registrationRequestColumns {
	return buildRegistrationRequestColumns("")
}

func buildRegistrationRequestColumn(alias, name string) registrationRequestColumn {
	return registrationRequestColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type registrationRequestColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c registrationRequestColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c registrationRequestColumn) ShouldOmitParens() bool {
	return true
}

// RegistrationRequestSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type RegistrationRequestSetter struct {
	ID        *string          `db:"id,pk" `
	Email     *string          `db:"email" `
	Status    *string          `db:"status" `
	CreatedAt *time.Time       `db:"created_at" `
	UpdatedAt *time.Time       `db:"updated_at" `
	Data      *json.RawMessage `db:"data" `
}

func (s RegistrationRequestSetter) SetColumns() []string {
	vals := make([]string, 0, 6)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.Email != nil {
		vals = append(vals, "email")
	}
	if s.Status != nil {
		vals = append(vals, "status")
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

func (s RegistrationRequestSetter) Overwrite(t *RegistrationRequest) {
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
	if s.Status != nil {
		t.Status = func() string {
			if s.Status == nil {
				return *new(string)
			}
			return *s.Status
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

func (s *RegistrationRequestSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return RegistrationRequests.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

		if s.Status != nil {
			vals[2] = psql.Arg(func() string {
				if s.Status == nil {
					return *new(string)
				}
				return *s.Status
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

func (s RegistrationRequestSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s RegistrationRequestSetter) Expressions(prefix ...string) []bob.Expression {
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

	if s.Status != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "status")...),
			psql.Arg(s.Status),
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

// FindRegistrationRequest retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindRegistrationRequest(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*RegistrationRequest, error) {
	if len(cols) == 0 {
		return RegistrationRequests.Query(
			sm.Where(RegistrationRequests.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return RegistrationRequests.Query(
		sm.Where(RegistrationRequests.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(RegistrationRequests.Columns.Only(cols...)),
	).One(ctx, exec)
}

// RegistrationRequestExists checks the presence of a single record by primary key
func RegistrationRequestExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return RegistrationRequests.Query(
		sm.Where(RegistrationRequests.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after RegistrationRequest is retrieved from the database
func (o *RegistrationRequest) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = RegistrationRequests.AfterSelectHooks.RunHooks(ctx, exec, RegistrationRequestSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = RegistrationRequests.AfterInsertHooks.RunHooks(ctx, exec, RegistrationRequestSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = RegistrationRequests.AfterUpdateHooks.RunHooks(ctx, exec, RegistrationRequestSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = RegistrationRequests.AfterDeleteHooks.RunHooks(ctx, exec, RegistrationRequestSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = RegistrationRequests.AfterMergeHooks.RunHooks(ctx, exec, RegistrationRequestSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the RegistrationRequest
func (o *RegistrationRequest) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *RegistrationRequest) pkEQ() dialect.Expression {
	return psql.Quote("registration_requests", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the RegistrationRequest
func (o *RegistrationRequest) Update(ctx context.Context, exec bob.Executor, s *RegistrationRequestSetter) error {
	v, err := RegistrationRequests.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single RegistrationRequest record with an executor
func (o *RegistrationRequest) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := RegistrationRequests.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the RegistrationRequest using the executor
func (o *RegistrationRequest) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := RegistrationRequests.Query(
		sm.Where(RegistrationRequests.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after RegistrationRequestSlice is retrieved from the database
func (o RegistrationRequestSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = RegistrationRequests.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = RegistrationRequests.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = RegistrationRequests.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = RegistrationRequests.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = RegistrationRequests.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o RegistrationRequestSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("registration_requests", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o RegistrationRequestSlice) copyMatchingRows(from ...*RegistrationRequest) {
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
func (o RegistrationRequestSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return RegistrationRequests.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *RegistrationRequest:
				o.copyMatchingRows(retrieved)
			case []*RegistrationRequest:
				o.copyMatchingRows(retrieved...)
			case RegistrationRequestSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a RegistrationRequest or a slice of RegistrationRequest
				// then run the AfterUpdateHooks on the slice
				_, err = RegistrationRequests.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o RegistrationRequestSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return RegistrationRequests.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *RegistrationRequest:
				o.copyMatchingRows(retrieved)
			case []*RegistrationRequest:
				o.copyMatchingRows(retrieved...)
			case RegistrationRequestSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a RegistrationRequest or a slice of RegistrationRequest
				// then run the AfterDeleteHooks on the slice
				_, err = RegistrationRequests.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o RegistrationRequestSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return RegistrationRequests.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *RegistrationRequest:
				o.copyMatchingRows(retrieved)
			case []*RegistrationRequest:
				o.copyMatchingRows(retrieved...)
			case RegistrationRequestSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a RegistrationRequest or a slice of RegistrationRequest
				// then run the AfterMergeHooks on the slice
				_, err = RegistrationRequests.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o RegistrationRequestSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals RegistrationRequestSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := RegistrationRequests.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o RegistrationRequestSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := RegistrationRequests.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o RegistrationRequestSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := RegistrationRequests.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type registrationRequestWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	Email     psql.WhereMod[Q, string]
	Status    psql.WhereMod[Q, string]
	CreatedAt psql.WhereMod[Q, time.Time]
	UpdatedAt psql.WhereMod[Q, time.Time]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (registrationRequestWhere[Q]) AliasedAs(alias string) registrationRequestWhere[Q] {
	return buildRegistrationRequestWhere[Q](buildRegistrationRequestColumns(alias))
}

func buildRegistrationRequestWhere[Q psql.Filterable](cols registrationRequestColumns) registrationRequestWhere[Q] {
	return registrationRequestWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		Email:     psql.Where[Q, string](cols.Email.Expression),
		Status:    psql.Where[Q, string](cols.Status.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
