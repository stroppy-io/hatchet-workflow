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

// IamExternalIdentity is an object representing the database table.
type IamExternalIdentity struct {
	ID         string          `db:"id,pk" `
	ProviderID string          `db:"provider_id" `
	Subject    string          `db:"subject" `
	AccountID  string          `db:"account_id" `
	CreatedAt  time.Time       `db:"created_at" `
	UpdatedAt  time.Time       `db:"updated_at" `
	Data       json.RawMessage `db:"data" `
}

// IamExternalIdentitySlice is an alias for a slice of pointers to IamExternalIdentity.
// This should almost always be used instead of []*IamExternalIdentity.
type IamExternalIdentitySlice []*IamExternalIdentity

// IamExternalIdentities contains methods to work with the iam_external_identities table
var IamExternalIdentities = psql.NewTablex[*IamExternalIdentity, IamExternalIdentitySlice, *IamExternalIdentitySetter]("", "iam_external_identities", buildIamExternalIdentityColumns("iam_external_identities"))

// IamExternalIdentitiesQuery is a query on the iam_external_identities table
type IamExternalIdentitiesQuery = *psql.ViewQuery[*IamExternalIdentity, IamExternalIdentitySlice]

func buildIamExternalIdentityColumns(tableName string) iamExternalIdentityColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "provider_id", "subject", "account_id", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return iamExternalIdentityColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildIamExternalIdentityColumn(tableName, "id"),
		ProviderID:  buildIamExternalIdentityColumn(tableName, "provider_id"),
		Subject:     buildIamExternalIdentityColumn(tableName, "subject"),
		AccountID:   buildIamExternalIdentityColumn(tableName, "account_id"),
		CreatedAt:   buildIamExternalIdentityColumn(tableName, "created_at"),
		UpdatedAt:   buildIamExternalIdentityColumn(tableName, "updated_at"),
		Data:        buildIamExternalIdentityColumn(tableName, "data"),
	}
}

type iamExternalIdentityColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         iamExternalIdentityColumn
	ProviderID iamExternalIdentityColumn
	Subject    iamExternalIdentityColumn
	AccountID  iamExternalIdentityColumn
	CreatedAt  iamExternalIdentityColumn
	UpdatedAt  iamExternalIdentityColumn
	Data       iamExternalIdentityColumn
}

// Alias returns the current table alias for the columns set.
func (c iamExternalIdentityColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (iamExternalIdentityColumns) AliasedAs(tableName string) iamExternalIdentityColumns {
	return buildIamExternalIdentityColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c iamExternalIdentityColumns) Unqualified() iamExternalIdentityColumns {
	return buildIamExternalIdentityColumns("")
}

func buildIamExternalIdentityColumn(alias, name string) iamExternalIdentityColumn {
	return iamExternalIdentityColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type iamExternalIdentityColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c iamExternalIdentityColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c iamExternalIdentityColumn) ShouldOmitParens() bool {
	return true
}

// IamExternalIdentitySetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type IamExternalIdentitySetter struct {
	ID         *string          `db:"id,pk" `
	ProviderID *string          `db:"provider_id" `
	Subject    *string          `db:"subject" `
	AccountID  *string          `db:"account_id" `
	CreatedAt  *time.Time       `db:"created_at" `
	UpdatedAt  *time.Time       `db:"updated_at" `
	Data       *json.RawMessage `db:"data" `
}

func (s IamExternalIdentitySetter) SetColumns() []string {
	vals := make([]string, 0, 7)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.ProviderID != nil {
		vals = append(vals, "provider_id")
	}
	if s.Subject != nil {
		vals = append(vals, "subject")
	}
	if s.AccountID != nil {
		vals = append(vals, "account_id")
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

func (s IamExternalIdentitySetter) Overwrite(t *IamExternalIdentity) {
	if s.ID != nil {
		t.ID = func() string {
			if s.ID == nil {
				return *new(string)
			}
			return *s.ID
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
	if s.Subject != nil {
		t.Subject = func() string {
			if s.Subject == nil {
				return *new(string)
			}
			return *s.Subject
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

func (s *IamExternalIdentitySetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return IamExternalIdentities.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 7)
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

		if s.Subject != nil {
			vals[2] = psql.Arg(func() string {
				if s.Subject == nil {
					return *new(string)
				}
				return *s.Subject
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		if s.AccountID != nil {
			vals[3] = psql.Arg(func() string {
				if s.AccountID == nil {
					return *new(string)
				}
				return *s.AccountID
			}())
		} else {
			vals[3] = psql.Raw("DEFAULT")
		}

		if s.CreatedAt != nil {
			vals[4] = psql.Arg(func() time.Time {
				if s.CreatedAt == nil {
					return *new(time.Time)
				}
				return *s.CreatedAt
			}())
		} else {
			vals[4] = psql.Raw("DEFAULT")
		}

		if s.UpdatedAt != nil {
			vals[5] = psql.Arg(func() time.Time {
				if s.UpdatedAt == nil {
					return *new(time.Time)
				}
				return *s.UpdatedAt
			}())
		} else {
			vals[5] = psql.Raw("DEFAULT")
		}

		if s.Data != nil {
			vals[6] = psql.Arg(func() json.RawMessage {
				if s.Data == nil {
					return *new(json.RawMessage)
				}
				return *s.Data
			}())
		} else {
			vals[6] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s IamExternalIdentitySetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s IamExternalIdentitySetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 7)

	if s.ID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "id")...),
			psql.Arg(s.ID),
		}})
	}

	if s.ProviderID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "provider_id")...),
			psql.Arg(s.ProviderID),
		}})
	}

	if s.Subject != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "subject")...),
			psql.Arg(s.Subject),
		}})
	}

	if s.AccountID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "account_id")...),
			psql.Arg(s.AccountID),
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

// FindIamExternalIdentity retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindIamExternalIdentity(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*IamExternalIdentity, error) {
	if len(cols) == 0 {
		return IamExternalIdentities.Query(
			sm.Where(IamExternalIdentities.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return IamExternalIdentities.Query(
		sm.Where(IamExternalIdentities.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(IamExternalIdentities.Columns.Only(cols...)),
	).One(ctx, exec)
}

// IamExternalIdentityExists checks the presence of a single record by primary key
func IamExternalIdentityExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return IamExternalIdentities.Query(
		sm.Where(IamExternalIdentities.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after IamExternalIdentity is retrieved from the database
func (o *IamExternalIdentity) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamExternalIdentities.AfterSelectHooks.RunHooks(ctx, exec, IamExternalIdentitySlice{o})
	case bob.QueryTypeInsert:
		ctx, err = IamExternalIdentities.AfterInsertHooks.RunHooks(ctx, exec, IamExternalIdentitySlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = IamExternalIdentities.AfterUpdateHooks.RunHooks(ctx, exec, IamExternalIdentitySlice{o})
	case bob.QueryTypeDelete:
		ctx, err = IamExternalIdentities.AfterDeleteHooks.RunHooks(ctx, exec, IamExternalIdentitySlice{o})
	case bob.QueryTypeMerge:
		ctx, err = IamExternalIdentities.AfterMergeHooks.RunHooks(ctx, exec, IamExternalIdentitySlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the IamExternalIdentity
func (o *IamExternalIdentity) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *IamExternalIdentity) pkEQ() dialect.Expression {
	return psql.Quote("iam_external_identities", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the IamExternalIdentity
func (o *IamExternalIdentity) Update(ctx context.Context, exec bob.Executor, s *IamExternalIdentitySetter) error {
	v, err := IamExternalIdentities.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single IamExternalIdentity record with an executor
func (o *IamExternalIdentity) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := IamExternalIdentities.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the IamExternalIdentity using the executor
func (o *IamExternalIdentity) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := IamExternalIdentities.Query(
		sm.Where(IamExternalIdentities.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after IamExternalIdentitySlice is retrieved from the database
func (o IamExternalIdentitySlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = IamExternalIdentities.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = IamExternalIdentities.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = IamExternalIdentities.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = IamExternalIdentities.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = IamExternalIdentities.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o IamExternalIdentitySlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("iam_external_identities", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o IamExternalIdentitySlice) copyMatchingRows(from ...*IamExternalIdentity) {
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
func (o IamExternalIdentitySlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamExternalIdentities.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamExternalIdentity:
				o.copyMatchingRows(retrieved)
			case []*IamExternalIdentity:
				o.copyMatchingRows(retrieved...)
			case IamExternalIdentitySlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamExternalIdentity or a slice of IamExternalIdentity
				// then run the AfterUpdateHooks on the slice
				_, err = IamExternalIdentities.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o IamExternalIdentitySlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamExternalIdentities.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamExternalIdentity:
				o.copyMatchingRows(retrieved)
			case []*IamExternalIdentity:
				o.copyMatchingRows(retrieved...)
			case IamExternalIdentitySlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamExternalIdentity or a slice of IamExternalIdentity
				// then run the AfterDeleteHooks on the slice
				_, err = IamExternalIdentities.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o IamExternalIdentitySlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return IamExternalIdentities.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *IamExternalIdentity:
				o.copyMatchingRows(retrieved)
			case []*IamExternalIdentity:
				o.copyMatchingRows(retrieved...)
			case IamExternalIdentitySlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a IamExternalIdentity or a slice of IamExternalIdentity
				// then run the AfterMergeHooks on the slice
				_, err = IamExternalIdentities.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o IamExternalIdentitySlice) UpdateAll(ctx context.Context, exec bob.Executor, vals IamExternalIdentitySetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamExternalIdentities.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o IamExternalIdentitySlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := IamExternalIdentities.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o IamExternalIdentitySlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := IamExternalIdentities.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type iamExternalIdentityWhere[Q psql.Filterable] struct {
	ID         psql.WhereMod[Q, string]
	ProviderID psql.WhereMod[Q, string]
	Subject    psql.WhereMod[Q, string]
	AccountID  psql.WhereMod[Q, string]
	CreatedAt  psql.WhereMod[Q, time.Time]
	UpdatedAt  psql.WhereMod[Q, time.Time]
	Data       psql.WhereMod[Q, json.RawMessage]
}

func (iamExternalIdentityWhere[Q]) AliasedAs(alias string) iamExternalIdentityWhere[Q] {
	return buildIamExternalIdentityWhere[Q](buildIamExternalIdentityColumns(alias))
}

func buildIamExternalIdentityWhere[Q psql.Filterable](cols iamExternalIdentityColumns) iamExternalIdentityWhere[Q] {
	return iamExternalIdentityWhere[Q]{
		ID:         psql.Where[Q, string](cols.ID.Expression),
		ProviderID: psql.Where[Q, string](cols.ProviderID.Expression),
		Subject:    psql.Where[Q, string](cols.Subject.Expression),
		AccountID:  psql.Where[Q, string](cols.AccountID.Expression),
		CreatedAt:  psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt:  psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:       psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
