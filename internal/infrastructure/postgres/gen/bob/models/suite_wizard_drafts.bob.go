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

// SuiteWizardDraft is an object representing the database table.
type SuiteWizardDraft struct {
	ID        string          `db:"id,pk" `
	TenantID  string          `db:"tenant_id" `
	CreatedAt time.Time       `db:"created_at" `
	UpdatedAt time.Time       `db:"updated_at" `
	Data      json.RawMessage `db:"data" `
}

// SuiteWizardDraftSlice is an alias for a slice of pointers to SuiteWizardDraft.
// This should almost always be used instead of []*SuiteWizardDraft.
type SuiteWizardDraftSlice []*SuiteWizardDraft

// SuiteWizardDrafts contains methods to work with the suite_wizard_drafts table
var SuiteWizardDrafts = psql.NewTablex[*SuiteWizardDraft, SuiteWizardDraftSlice, *SuiteWizardDraftSetter]("", "suite_wizard_drafts", buildSuiteWizardDraftColumns("suite_wizard_drafts"))

// SuiteWizardDraftsQuery is a query on the suite_wizard_drafts table
type SuiteWizardDraftsQuery = *psql.ViewQuery[*SuiteWizardDraft, SuiteWizardDraftSlice]

func buildSuiteWizardDraftColumns(tableName string) suiteWizardDraftColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "tenant_id", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return suiteWizardDraftColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildSuiteWizardDraftColumn(tableName, "id"),
		TenantID:    buildSuiteWizardDraftColumn(tableName, "tenant_id"),
		CreatedAt:   buildSuiteWizardDraftColumn(tableName, "created_at"),
		UpdatedAt:   buildSuiteWizardDraftColumn(tableName, "updated_at"),
		Data:        buildSuiteWizardDraftColumn(tableName, "data"),
	}
}

type suiteWizardDraftColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         suiteWizardDraftColumn
	TenantID   suiteWizardDraftColumn
	CreatedAt  suiteWizardDraftColumn
	UpdatedAt  suiteWizardDraftColumn
	Data       suiteWizardDraftColumn
}

// Alias returns the current table alias for the columns set.
func (c suiteWizardDraftColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (suiteWizardDraftColumns) AliasedAs(tableName string) suiteWizardDraftColumns {
	return buildSuiteWizardDraftColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c suiteWizardDraftColumns) Unqualified() suiteWizardDraftColumns {
	return buildSuiteWizardDraftColumns("")
}

func buildSuiteWizardDraftColumn(alias, name string) suiteWizardDraftColumn {
	return suiteWizardDraftColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type suiteWizardDraftColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c suiteWizardDraftColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c suiteWizardDraftColumn) ShouldOmitParens() bool {
	return true
}

// SuiteWizardDraftSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type SuiteWizardDraftSetter struct {
	ID        *string          `db:"id,pk" `
	TenantID  *string          `db:"tenant_id" `
	CreatedAt *time.Time       `db:"created_at" `
	UpdatedAt *time.Time       `db:"updated_at" `
	Data      *json.RawMessage `db:"data" `
}

func (s SuiteWizardDraftSetter) SetColumns() []string {
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

func (s SuiteWizardDraftSetter) Overwrite(t *SuiteWizardDraft) {
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

func (s *SuiteWizardDraftSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return SuiteWizardDrafts.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

func (s SuiteWizardDraftSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s SuiteWizardDraftSetter) Expressions(prefix ...string) []bob.Expression {
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

// FindSuiteWizardDraft retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindSuiteWizardDraft(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*SuiteWizardDraft, error) {
	if len(cols) == 0 {
		return SuiteWizardDrafts.Query(
			sm.Where(SuiteWizardDrafts.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return SuiteWizardDrafts.Query(
		sm.Where(SuiteWizardDrafts.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(SuiteWizardDrafts.Columns.Only(cols...)),
	).One(ctx, exec)
}

// SuiteWizardDraftExists checks the presence of a single record by primary key
func SuiteWizardDraftExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return SuiteWizardDrafts.Query(
		sm.Where(SuiteWizardDrafts.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after SuiteWizardDraft is retrieved from the database
func (o *SuiteWizardDraft) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = SuiteWizardDrafts.AfterSelectHooks.RunHooks(ctx, exec, SuiteWizardDraftSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = SuiteWizardDrafts.AfterInsertHooks.RunHooks(ctx, exec, SuiteWizardDraftSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = SuiteWizardDrafts.AfterUpdateHooks.RunHooks(ctx, exec, SuiteWizardDraftSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = SuiteWizardDrafts.AfterDeleteHooks.RunHooks(ctx, exec, SuiteWizardDraftSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = SuiteWizardDrafts.AfterMergeHooks.RunHooks(ctx, exec, SuiteWizardDraftSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the SuiteWizardDraft
func (o *SuiteWizardDraft) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *SuiteWizardDraft) pkEQ() dialect.Expression {
	return psql.Quote("suite_wizard_drafts", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the SuiteWizardDraft
func (o *SuiteWizardDraft) Update(ctx context.Context, exec bob.Executor, s *SuiteWizardDraftSetter) error {
	v, err := SuiteWizardDrafts.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single SuiteWizardDraft record with an executor
func (o *SuiteWizardDraft) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := SuiteWizardDrafts.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the SuiteWizardDraft using the executor
func (o *SuiteWizardDraft) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := SuiteWizardDrafts.Query(
		sm.Where(SuiteWizardDrafts.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after SuiteWizardDraftSlice is retrieved from the database
func (o SuiteWizardDraftSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = SuiteWizardDrafts.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = SuiteWizardDrafts.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = SuiteWizardDrafts.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = SuiteWizardDrafts.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = SuiteWizardDrafts.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o SuiteWizardDraftSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("suite_wizard_drafts", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o SuiteWizardDraftSlice) copyMatchingRows(from ...*SuiteWizardDraft) {
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
func (o SuiteWizardDraftSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return SuiteWizardDrafts.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *SuiteWizardDraft:
				o.copyMatchingRows(retrieved)
			case []*SuiteWizardDraft:
				o.copyMatchingRows(retrieved...)
			case SuiteWizardDraftSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a SuiteWizardDraft or a slice of SuiteWizardDraft
				// then run the AfterUpdateHooks on the slice
				_, err = SuiteWizardDrafts.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o SuiteWizardDraftSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return SuiteWizardDrafts.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *SuiteWizardDraft:
				o.copyMatchingRows(retrieved)
			case []*SuiteWizardDraft:
				o.copyMatchingRows(retrieved...)
			case SuiteWizardDraftSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a SuiteWizardDraft or a slice of SuiteWizardDraft
				// then run the AfterDeleteHooks on the slice
				_, err = SuiteWizardDrafts.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o SuiteWizardDraftSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return SuiteWizardDrafts.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *SuiteWizardDraft:
				o.copyMatchingRows(retrieved)
			case []*SuiteWizardDraft:
				o.copyMatchingRows(retrieved...)
			case SuiteWizardDraftSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a SuiteWizardDraft or a slice of SuiteWizardDraft
				// then run the AfterMergeHooks on the slice
				_, err = SuiteWizardDrafts.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o SuiteWizardDraftSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals SuiteWizardDraftSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := SuiteWizardDrafts.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o SuiteWizardDraftSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := SuiteWizardDrafts.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o SuiteWizardDraftSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := SuiteWizardDrafts.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type suiteWizardDraftWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	TenantID  psql.WhereMod[Q, string]
	CreatedAt psql.WhereMod[Q, time.Time]
	UpdatedAt psql.WhereMod[Q, time.Time]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (suiteWizardDraftWhere[Q]) AliasedAs(alias string) suiteWizardDraftWhere[Q] {
	return buildSuiteWizardDraftWhere[Q](buildSuiteWizardDraftColumns(alias))
}

func buildSuiteWizardDraftWhere[Q psql.Filterable](cols suiteWizardDraftColumns) suiteWizardDraftWhere[Q] {
	return suiteWizardDraftWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		TenantID:  psql.Where[Q, string](cols.TenantID.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
