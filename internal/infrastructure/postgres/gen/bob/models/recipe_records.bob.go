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

// RecipeRecord is an object representing the database table.
type RecipeRecord struct {
	ID        string          `db:"id,pk" `
	TenantID  string          `db:"tenant_id" `
	Name      string          `db:"name" `
	Version   int32           `db:"version" `
	CreatedAt time.Time       `db:"created_at" `
	UpdatedAt time.Time       `db:"updated_at" `
	SourceRef string          `db:"source_ref" `
	Data      json.RawMessage `db:"data" `
}

// RecipeRecordSlice is an alias for a slice of pointers to RecipeRecord.
// This should almost always be used instead of []*RecipeRecord.
type RecipeRecordSlice []*RecipeRecord

// RecipeRecords contains methods to work with the recipe_records table
var RecipeRecords = psql.NewTablex[*RecipeRecord, RecipeRecordSlice, *RecipeRecordSetter]("", "recipe_records", buildRecipeRecordColumns("recipe_records"))

// RecipeRecordsQuery is a query on the recipe_records table
type RecipeRecordsQuery = *psql.ViewQuery[*RecipeRecord, RecipeRecordSlice]

func buildRecipeRecordColumns(tableName string) recipeRecordColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "tenant_id", "name", "version", "created_at", "updated_at", "source_ref", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return recipeRecordColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildRecipeRecordColumn(tableName, "id"),
		TenantID:    buildRecipeRecordColumn(tableName, "tenant_id"),
		Name:        buildRecipeRecordColumn(tableName, "name"),
		Version:     buildRecipeRecordColumn(tableName, "version"),
		CreatedAt:   buildRecipeRecordColumn(tableName, "created_at"),
		UpdatedAt:   buildRecipeRecordColumn(tableName, "updated_at"),
		SourceRef:   buildRecipeRecordColumn(tableName, "source_ref"),
		Data:        buildRecipeRecordColumn(tableName, "data"),
	}
}

type recipeRecordColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         recipeRecordColumn
	TenantID   recipeRecordColumn
	Name       recipeRecordColumn
	Version    recipeRecordColumn
	CreatedAt  recipeRecordColumn
	UpdatedAt  recipeRecordColumn
	SourceRef  recipeRecordColumn
	Data       recipeRecordColumn
}

// Alias returns the current table alias for the columns set.
func (c recipeRecordColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (recipeRecordColumns) AliasedAs(tableName string) recipeRecordColumns {
	return buildRecipeRecordColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c recipeRecordColumns) Unqualified() recipeRecordColumns {
	return buildRecipeRecordColumns("")
}

func buildRecipeRecordColumn(alias, name string) recipeRecordColumn {
	return recipeRecordColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type recipeRecordColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c recipeRecordColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c recipeRecordColumn) ShouldOmitParens() bool {
	return true
}

// RecipeRecordSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type RecipeRecordSetter struct {
	ID        *string          `db:"id,pk" `
	TenantID  *string          `db:"tenant_id" `
	Name      *string          `db:"name" `
	Version   *int32           `db:"version" `
	CreatedAt *time.Time       `db:"created_at" `
	UpdatedAt *time.Time       `db:"updated_at" `
	SourceRef *string          `db:"source_ref" `
	Data      *json.RawMessage `db:"data" `
}

func (s RecipeRecordSetter) SetColumns() []string {
	vals := make([]string, 0, 8)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.TenantID != nil {
		vals = append(vals, "tenant_id")
	}
	if s.Name != nil {
		vals = append(vals, "name")
	}
	if s.Version != nil {
		vals = append(vals, "version")
	}
	if s.CreatedAt != nil {
		vals = append(vals, "created_at")
	}
	if s.UpdatedAt != nil {
		vals = append(vals, "updated_at")
	}
	if s.SourceRef != nil {
		vals = append(vals, "source_ref")
	}
	if s.Data != nil {
		vals = append(vals, "data")
	}
	return vals
}

func (s RecipeRecordSetter) Overwrite(t *RecipeRecord) {
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
	if s.Name != nil {
		t.Name = func() string {
			if s.Name == nil {
				return *new(string)
			}
			return *s.Name
		}()
	}
	if s.Version != nil {
		t.Version = func() int32 {
			if s.Version == nil {
				return *new(int32)
			}
			return *s.Version
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
	if s.SourceRef != nil {
		t.SourceRef = func() string {
			if s.SourceRef == nil {
				return *new(string)
			}
			return *s.SourceRef
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

func (s *RecipeRecordSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return RecipeRecords.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 8)
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

		if s.Name != nil {
			vals[2] = psql.Arg(func() string {
				if s.Name == nil {
					return *new(string)
				}
				return *s.Name
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		if s.Version != nil {
			vals[3] = psql.Arg(func() int32 {
				if s.Version == nil {
					return *new(int32)
				}
				return *s.Version
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

		if s.SourceRef != nil {
			vals[6] = psql.Arg(func() string {
				if s.SourceRef == nil {
					return *new(string)
				}
				return *s.SourceRef
			}())
		} else {
			vals[6] = psql.Raw("DEFAULT")
		}

		if s.Data != nil {
			vals[7] = psql.Arg(func() json.RawMessage {
				if s.Data == nil {
					return *new(json.RawMessage)
				}
				return *s.Data
			}())
		} else {
			vals[7] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s RecipeRecordSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s RecipeRecordSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 8)

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

	if s.Name != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "name")...),
			psql.Arg(s.Name),
		}})
	}

	if s.Version != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "version")...),
			psql.Arg(s.Version),
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

	if s.SourceRef != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "source_ref")...),
			psql.Arg(s.SourceRef),
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

// FindRecipeRecord retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindRecipeRecord(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*RecipeRecord, error) {
	if len(cols) == 0 {
		return RecipeRecords.Query(
			sm.Where(RecipeRecords.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return RecipeRecords.Query(
		sm.Where(RecipeRecords.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(RecipeRecords.Columns.Only(cols...)),
	).One(ctx, exec)
}

// RecipeRecordExists checks the presence of a single record by primary key
func RecipeRecordExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return RecipeRecords.Query(
		sm.Where(RecipeRecords.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after RecipeRecord is retrieved from the database
func (o *RecipeRecord) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = RecipeRecords.AfterSelectHooks.RunHooks(ctx, exec, RecipeRecordSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = RecipeRecords.AfterInsertHooks.RunHooks(ctx, exec, RecipeRecordSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = RecipeRecords.AfterUpdateHooks.RunHooks(ctx, exec, RecipeRecordSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = RecipeRecords.AfterDeleteHooks.RunHooks(ctx, exec, RecipeRecordSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = RecipeRecords.AfterMergeHooks.RunHooks(ctx, exec, RecipeRecordSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the RecipeRecord
func (o *RecipeRecord) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *RecipeRecord) pkEQ() dialect.Expression {
	return psql.Quote("recipe_records", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the RecipeRecord
func (o *RecipeRecord) Update(ctx context.Context, exec bob.Executor, s *RecipeRecordSetter) error {
	v, err := RecipeRecords.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single RecipeRecord record with an executor
func (o *RecipeRecord) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := RecipeRecords.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the RecipeRecord using the executor
func (o *RecipeRecord) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := RecipeRecords.Query(
		sm.Where(RecipeRecords.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after RecipeRecordSlice is retrieved from the database
func (o RecipeRecordSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = RecipeRecords.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = RecipeRecords.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = RecipeRecords.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = RecipeRecords.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = RecipeRecords.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o RecipeRecordSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("recipe_records", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o RecipeRecordSlice) copyMatchingRows(from ...*RecipeRecord) {
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
func (o RecipeRecordSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return RecipeRecords.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *RecipeRecord:
				o.copyMatchingRows(retrieved)
			case []*RecipeRecord:
				o.copyMatchingRows(retrieved...)
			case RecipeRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a RecipeRecord or a slice of RecipeRecord
				// then run the AfterUpdateHooks on the slice
				_, err = RecipeRecords.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o RecipeRecordSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return RecipeRecords.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *RecipeRecord:
				o.copyMatchingRows(retrieved)
			case []*RecipeRecord:
				o.copyMatchingRows(retrieved...)
			case RecipeRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a RecipeRecord or a slice of RecipeRecord
				// then run the AfterDeleteHooks on the slice
				_, err = RecipeRecords.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o RecipeRecordSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return RecipeRecords.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *RecipeRecord:
				o.copyMatchingRows(retrieved)
			case []*RecipeRecord:
				o.copyMatchingRows(retrieved...)
			case RecipeRecordSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a RecipeRecord or a slice of RecipeRecord
				// then run the AfterMergeHooks on the slice
				_, err = RecipeRecords.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o RecipeRecordSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals RecipeRecordSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := RecipeRecords.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o RecipeRecordSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := RecipeRecords.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o RecipeRecordSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := RecipeRecords.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type recipeRecordWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	TenantID  psql.WhereMod[Q, string]
	Name      psql.WhereMod[Q, string]
	Version   psql.WhereMod[Q, int32]
	CreatedAt psql.WhereMod[Q, time.Time]
	UpdatedAt psql.WhereMod[Q, time.Time]
	SourceRef psql.WhereMod[Q, string]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (recipeRecordWhere[Q]) AliasedAs(alias string) recipeRecordWhere[Q] {
	return buildRecipeRecordWhere[Q](buildRecipeRecordColumns(alias))
}

func buildRecipeRecordWhere[Q psql.Filterable](cols recipeRecordColumns) recipeRecordWhere[Q] {
	return recipeRecordWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		TenantID:  psql.Where[Q, string](cols.TenantID.Expression),
		Name:      psql.Where[Q, string](cols.Name.Expression),
		Version:   psql.Where[Q, int32](cols.Version.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		SourceRef: psql.Where[Q, string](cols.SourceRef.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
