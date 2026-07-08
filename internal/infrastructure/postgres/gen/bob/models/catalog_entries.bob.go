// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/aarondl/opt/null"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/bob/expr"
)

// CatalogEntry is an object representing the database table.
type CatalogEntry struct {
	ID            string           `db:"id,pk" `
	Level         string           `db:"level" `
	TenantID      string           `db:"tenant_id" `
	Kind          string           `db:"kind" `
	Slug          string           `db:"slug" `
	Version       int32            `db:"version" `
	Origin        string           `db:"origin" `
	SourceEntryID null.Val[string] `db:"source_entry_id" `
	CreatedAt     time.Time        `db:"created_at" `
	UpdatedAt     time.Time        `db:"updated_at" `
	Data          json.RawMessage  `db:"data" `
}

// CatalogEntrySlice is an alias for a slice of pointers to CatalogEntry.
// This should almost always be used instead of []*CatalogEntry.
type CatalogEntrySlice []*CatalogEntry

// CatalogEntries contains methods to work with the catalog_entries table
var CatalogEntries = psql.NewTablex[*CatalogEntry, CatalogEntrySlice, *CatalogEntrySetter]("", "catalog_entries", buildCatalogEntryColumns("catalog_entries"))

// CatalogEntriesQuery is a query on the catalog_entries table
type CatalogEntriesQuery = *psql.ViewQuery[*CatalogEntry, CatalogEntrySlice]

func buildCatalogEntryColumns(tableName string) catalogEntryColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "level", "tenant_id", "kind", "slug", "version", "origin", "source_entry_id", "created_at", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return catalogEntryColumns{
		ColumnsExpr:   columnsExpr,
		tableAlias:    tableName,
		ID:            buildCatalogEntryColumn(tableName, "id"),
		Level:         buildCatalogEntryColumn(tableName, "level"),
		TenantID:      buildCatalogEntryColumn(tableName, "tenant_id"),
		Kind:          buildCatalogEntryColumn(tableName, "kind"),
		Slug:          buildCatalogEntryColumn(tableName, "slug"),
		Version:       buildCatalogEntryColumn(tableName, "version"),
		Origin:        buildCatalogEntryColumn(tableName, "origin"),
		SourceEntryID: buildCatalogEntryColumn(tableName, "source_entry_id"),
		CreatedAt:     buildCatalogEntryColumn(tableName, "created_at"),
		UpdatedAt:     buildCatalogEntryColumn(tableName, "updated_at"),
		Data:          buildCatalogEntryColumn(tableName, "data"),
	}
}

type catalogEntryColumns struct {
	expr.ColumnsExpr
	tableAlias    string
	ID            catalogEntryColumn
	Level         catalogEntryColumn
	TenantID      catalogEntryColumn
	Kind          catalogEntryColumn
	Slug          catalogEntryColumn
	Version       catalogEntryColumn
	Origin        catalogEntryColumn
	SourceEntryID catalogEntryColumn
	CreatedAt     catalogEntryColumn
	UpdatedAt     catalogEntryColumn
	Data          catalogEntryColumn
}

// Alias returns the current table alias for the columns set.
func (c catalogEntryColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (catalogEntryColumns) AliasedAs(tableName string) catalogEntryColumns {
	return buildCatalogEntryColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c catalogEntryColumns) Unqualified() catalogEntryColumns {
	return buildCatalogEntryColumns("")
}

func buildCatalogEntryColumn(alias, name string) catalogEntryColumn {
	return catalogEntryColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type catalogEntryColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c catalogEntryColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c catalogEntryColumn) ShouldOmitParens() bool {
	return true
}

// CatalogEntrySetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type CatalogEntrySetter struct {
	ID            *string           `db:"id,pk" `
	Level         *string           `db:"level" `
	TenantID      *string           `db:"tenant_id" `
	Kind          *string           `db:"kind" `
	Slug          *string           `db:"slug" `
	Version       *int32            `db:"version" `
	Origin        *string           `db:"origin" `
	SourceEntryID *null.Val[string] `db:"source_entry_id" `
	CreatedAt     *time.Time        `db:"created_at" `
	UpdatedAt     *time.Time        `db:"updated_at" `
	Data          *json.RawMessage  `db:"data" `
}

func (s CatalogEntrySetter) SetColumns() []string {
	vals := make([]string, 0, 11)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.Level != nil {
		vals = append(vals, "level")
	}
	if s.TenantID != nil {
		vals = append(vals, "tenant_id")
	}
	if s.Kind != nil {
		vals = append(vals, "kind")
	}
	if s.Slug != nil {
		vals = append(vals, "slug")
	}
	if s.Version != nil {
		vals = append(vals, "version")
	}
	if s.Origin != nil {
		vals = append(vals, "origin")
	}
	if s.SourceEntryID != nil {
		vals = append(vals, "source_entry_id")
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

func (s CatalogEntrySetter) Overwrite(t *CatalogEntry) {
	if s.ID != nil {
		t.ID = func() string {
			if s.ID == nil {
				return *new(string)
			}
			return *s.ID
		}()
	}
	if s.Level != nil {
		t.Level = func() string {
			if s.Level == nil {
				return *new(string)
			}
			return *s.Level
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
	if s.Kind != nil {
		t.Kind = func() string {
			if s.Kind == nil {
				return *new(string)
			}
			return *s.Kind
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
	if s.Version != nil {
		t.Version = func() int32 {
			if s.Version == nil {
				return *new(int32)
			}
			return *s.Version
		}()
	}
	if s.Origin != nil {
		t.Origin = func() string {
			if s.Origin == nil {
				return *new(string)
			}
			return *s.Origin
		}()
	}
	if s.SourceEntryID != nil {
		t.SourceEntryID = func() null.Val[string] {
			if s.SourceEntryID == nil {
				return *new(null.Val[string])
			}
			v := s.SourceEntryID
			return *v
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

func (s *CatalogEntrySetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return CatalogEntries.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 11)
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

		if s.Level != nil {
			vals[1] = psql.Arg(func() string {
				if s.Level == nil {
					return *new(string)
				}
				return *s.Level
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

		if s.Kind != nil {
			vals[3] = psql.Arg(func() string {
				if s.Kind == nil {
					return *new(string)
				}
				return *s.Kind
			}())
		} else {
			vals[3] = psql.Raw("DEFAULT")
		}

		if s.Slug != nil {
			vals[4] = psql.Arg(func() string {
				if s.Slug == nil {
					return *new(string)
				}
				return *s.Slug
			}())
		} else {
			vals[4] = psql.Raw("DEFAULT")
		}

		if s.Version != nil {
			vals[5] = psql.Arg(func() int32 {
				if s.Version == nil {
					return *new(int32)
				}
				return *s.Version
			}())
		} else {
			vals[5] = psql.Raw("DEFAULT")
		}

		if s.Origin != nil {
			vals[6] = psql.Arg(func() string {
				if s.Origin == nil {
					return *new(string)
				}
				return *s.Origin
			}())
		} else {
			vals[6] = psql.Raw("DEFAULT")
		}

		if s.SourceEntryID != nil {
			vals[7] = psql.Arg(func() null.Val[string] {
				if s.SourceEntryID == nil {
					return *new(null.Val[string])
				}
				v := s.SourceEntryID
				return *v
			}())
		} else {
			vals[7] = psql.Raw("DEFAULT")
		}

		if s.CreatedAt != nil {
			vals[8] = psql.Arg(func() time.Time {
				if s.CreatedAt == nil {
					return *new(time.Time)
				}
				return *s.CreatedAt
			}())
		} else {
			vals[8] = psql.Raw("DEFAULT")
		}

		if s.UpdatedAt != nil {
			vals[9] = psql.Arg(func() time.Time {
				if s.UpdatedAt == nil {
					return *new(time.Time)
				}
				return *s.UpdatedAt
			}())
		} else {
			vals[9] = psql.Raw("DEFAULT")
		}

		if s.Data != nil {
			vals[10] = psql.Arg(func() json.RawMessage {
				if s.Data == nil {
					return *new(json.RawMessage)
				}
				return *s.Data
			}())
		} else {
			vals[10] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s CatalogEntrySetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s CatalogEntrySetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 11)

	if s.ID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "id")...),
			psql.Arg(s.ID),
		}})
	}

	if s.Level != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "level")...),
			psql.Arg(s.Level),
		}})
	}

	if s.TenantID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "tenant_id")...),
			psql.Arg(s.TenantID),
		}})
	}

	if s.Kind != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "kind")...),
			psql.Arg(s.Kind),
		}})
	}

	if s.Slug != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "slug")...),
			psql.Arg(s.Slug),
		}})
	}

	if s.Version != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "version")...),
			psql.Arg(s.Version),
		}})
	}

	if s.Origin != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "origin")...),
			psql.Arg(s.Origin),
		}})
	}

	if s.SourceEntryID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "source_entry_id")...),
			psql.Arg(s.SourceEntryID),
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

// FindCatalogEntry retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindCatalogEntry(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*CatalogEntry, error) {
	if len(cols) == 0 {
		return CatalogEntries.Query(
			sm.Where(CatalogEntries.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return CatalogEntries.Query(
		sm.Where(CatalogEntries.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(CatalogEntries.Columns.Only(cols...)),
	).One(ctx, exec)
}

// CatalogEntryExists checks the presence of a single record by primary key
func CatalogEntryExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return CatalogEntries.Query(
		sm.Where(CatalogEntries.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after CatalogEntry is retrieved from the database
func (o *CatalogEntry) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = CatalogEntries.AfterSelectHooks.RunHooks(ctx, exec, CatalogEntrySlice{o})
	case bob.QueryTypeInsert:
		ctx, err = CatalogEntries.AfterInsertHooks.RunHooks(ctx, exec, CatalogEntrySlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = CatalogEntries.AfterUpdateHooks.RunHooks(ctx, exec, CatalogEntrySlice{o})
	case bob.QueryTypeDelete:
		ctx, err = CatalogEntries.AfterDeleteHooks.RunHooks(ctx, exec, CatalogEntrySlice{o})
	case bob.QueryTypeMerge:
		ctx, err = CatalogEntries.AfterMergeHooks.RunHooks(ctx, exec, CatalogEntrySlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the CatalogEntry
func (o *CatalogEntry) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *CatalogEntry) pkEQ() dialect.Expression {
	return psql.Quote("catalog_entries", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the CatalogEntry
func (o *CatalogEntry) Update(ctx context.Context, exec bob.Executor, s *CatalogEntrySetter) error {
	v, err := CatalogEntries.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single CatalogEntry record with an executor
func (o *CatalogEntry) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := CatalogEntries.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the CatalogEntry using the executor
func (o *CatalogEntry) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := CatalogEntries.Query(
		sm.Where(CatalogEntries.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after CatalogEntrySlice is retrieved from the database
func (o CatalogEntrySlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = CatalogEntries.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = CatalogEntries.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = CatalogEntries.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = CatalogEntries.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = CatalogEntries.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o CatalogEntrySlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("catalog_entries", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o CatalogEntrySlice) copyMatchingRows(from ...*CatalogEntry) {
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
func (o CatalogEntrySlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return CatalogEntries.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *CatalogEntry:
				o.copyMatchingRows(retrieved)
			case []*CatalogEntry:
				o.copyMatchingRows(retrieved...)
			case CatalogEntrySlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a CatalogEntry or a slice of CatalogEntry
				// then run the AfterUpdateHooks on the slice
				_, err = CatalogEntries.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o CatalogEntrySlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return CatalogEntries.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *CatalogEntry:
				o.copyMatchingRows(retrieved)
			case []*CatalogEntry:
				o.copyMatchingRows(retrieved...)
			case CatalogEntrySlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a CatalogEntry or a slice of CatalogEntry
				// then run the AfterDeleteHooks on the slice
				_, err = CatalogEntries.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o CatalogEntrySlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return CatalogEntries.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *CatalogEntry:
				o.copyMatchingRows(retrieved)
			case []*CatalogEntry:
				o.copyMatchingRows(retrieved...)
			case CatalogEntrySlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a CatalogEntry or a slice of CatalogEntry
				// then run the AfterMergeHooks on the slice
				_, err = CatalogEntries.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o CatalogEntrySlice) UpdateAll(ctx context.Context, exec bob.Executor, vals CatalogEntrySetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := CatalogEntries.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o CatalogEntrySlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := CatalogEntries.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o CatalogEntrySlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := CatalogEntries.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type catalogEntryWhere[Q psql.Filterable] struct {
	ID            psql.WhereMod[Q, string]
	Level         psql.WhereMod[Q, string]
	TenantID      psql.WhereMod[Q, string]
	Kind          psql.WhereMod[Q, string]
	Slug          psql.WhereMod[Q, string]
	Version       psql.WhereMod[Q, int32]
	Origin        psql.WhereMod[Q, string]
	SourceEntryID psql.WhereNullMod[Q, string]
	CreatedAt     psql.WhereMod[Q, time.Time]
	UpdatedAt     psql.WhereMod[Q, time.Time]
	Data          psql.WhereMod[Q, json.RawMessage]
}

func (catalogEntryWhere[Q]) AliasedAs(alias string) catalogEntryWhere[Q] {
	return buildCatalogEntryWhere[Q](buildCatalogEntryColumns(alias))
}

func buildCatalogEntryWhere[Q psql.Filterable](cols catalogEntryColumns) catalogEntryWhere[Q] {
	return catalogEntryWhere[Q]{
		ID:            psql.Where[Q, string](cols.ID.Expression),
		Level:         psql.Where[Q, string](cols.Level.Expression),
		TenantID:      psql.Where[Q, string](cols.TenantID.Expression),
		Kind:          psql.Where[Q, string](cols.Kind.Expression),
		Slug:          psql.Where[Q, string](cols.Slug.Expression),
		Version:       psql.Where[Q, int32](cols.Version.Expression),
		Origin:        psql.Where[Q, string](cols.Origin.Expression),
		SourceEntryID: psql.WhereNull[Q, string](cols.SourceEntryID.Expression),
		CreatedAt:     psql.Where[Q, time.Time](cols.CreatedAt.Expression),
		UpdatedAt:     psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:          psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
