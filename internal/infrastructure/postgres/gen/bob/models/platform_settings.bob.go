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

// PlatformSetting is an object representing the database table.
type PlatformSetting struct {
	ID        string          `db:"id,pk" `
	UpdatedAt time.Time       `db:"updated_at" `
	Data      json.RawMessage `db:"data" `
}

// PlatformSettingSlice is an alias for a slice of pointers to PlatformSetting.
// This should almost always be used instead of []*PlatformSetting.
type PlatformSettingSlice []*PlatformSetting

// PlatformSettings contains methods to work with the platform_settings table
var PlatformSettings = psql.NewTablex[*PlatformSetting, PlatformSettingSlice, *PlatformSettingSetter]("", "platform_settings", buildPlatformSettingColumns("platform_settings"))

// PlatformSettingsQuery is a query on the platform_settings table
type PlatformSettingsQuery = *psql.ViewQuery[*PlatformSetting, PlatformSettingSlice]

func buildPlatformSettingColumns(tableName string) platformSettingColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "updated_at", "data",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return platformSettingColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildPlatformSettingColumn(tableName, "id"),
		UpdatedAt:   buildPlatformSettingColumn(tableName, "updated_at"),
		Data:        buildPlatformSettingColumn(tableName, "data"),
	}
}

type platformSettingColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         platformSettingColumn
	UpdatedAt  platformSettingColumn
	Data       platformSettingColumn
}

// Alias returns the current table alias for the columns set.
func (c platformSettingColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (platformSettingColumns) AliasedAs(tableName string) platformSettingColumns {
	return buildPlatformSettingColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c platformSettingColumns) Unqualified() platformSettingColumns {
	return buildPlatformSettingColumns("")
}

func buildPlatformSettingColumn(alias, name string) platformSettingColumn {
	return platformSettingColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type platformSettingColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c platformSettingColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c platformSettingColumn) ShouldOmitParens() bool {
	return true
}

// PlatformSettingSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type PlatformSettingSetter struct {
	ID        *string          `db:"id,pk" `
	UpdatedAt *time.Time       `db:"updated_at" `
	Data      *json.RawMessage `db:"data" `
}

func (s PlatformSettingSetter) SetColumns() []string {
	vals := make([]string, 0, 3)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.UpdatedAt != nil {
		vals = append(vals, "updated_at")
	}
	if s.Data != nil {
		vals = append(vals, "data")
	}
	return vals
}

func (s PlatformSettingSetter) Overwrite(t *PlatformSetting) {
	if s.ID != nil {
		t.ID = func() string {
			if s.ID == nil {
				return *new(string)
			}
			return *s.ID
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

func (s *PlatformSettingSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return PlatformSettings.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

		if s.UpdatedAt != nil {
			vals[1] = psql.Arg(func() time.Time {
				if s.UpdatedAt == nil {
					return *new(time.Time)
				}
				return *s.UpdatedAt
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.Data != nil {
			vals[2] = psql.Arg(func() json.RawMessage {
				if s.Data == nil {
					return *new(json.RawMessage)
				}
				return *s.Data
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s PlatformSettingSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s PlatformSettingSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 3)

	if s.ID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "id")...),
			psql.Arg(s.ID),
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

// FindPlatformSetting retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindPlatformSetting(ctx context.Context, exec bob.Executor, IDPK string, cols ...string) (*PlatformSetting, error) {
	if len(cols) == 0 {
		return PlatformSettings.Query(
			sm.Where(PlatformSettings.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return PlatformSettings.Query(
		sm.Where(PlatformSettings.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(PlatformSettings.Columns.Only(cols...)),
	).One(ctx, exec)
}

// PlatformSettingExists checks the presence of a single record by primary key
func PlatformSettingExists(ctx context.Context, exec bob.Executor, IDPK string) (bool, error) {
	return PlatformSettings.Query(
		sm.Where(PlatformSettings.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after PlatformSetting is retrieved from the database
func (o *PlatformSetting) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = PlatformSettings.AfterSelectHooks.RunHooks(ctx, exec, PlatformSettingSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = PlatformSettings.AfterInsertHooks.RunHooks(ctx, exec, PlatformSettingSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = PlatformSettings.AfterUpdateHooks.RunHooks(ctx, exec, PlatformSettingSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = PlatformSettings.AfterDeleteHooks.RunHooks(ctx, exec, PlatformSettingSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = PlatformSettings.AfterMergeHooks.RunHooks(ctx, exec, PlatformSettingSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the PlatformSetting
func (o *PlatformSetting) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *PlatformSetting) pkEQ() dialect.Expression {
	return psql.Quote("platform_settings", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the PlatformSetting
func (o *PlatformSetting) Update(ctx context.Context, exec bob.Executor, s *PlatformSettingSetter) error {
	v, err := PlatformSettings.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single PlatformSetting record with an executor
func (o *PlatformSetting) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := PlatformSettings.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the PlatformSetting using the executor
func (o *PlatformSetting) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := PlatformSettings.Query(
		sm.Where(PlatformSettings.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after PlatformSettingSlice is retrieved from the database
func (o PlatformSettingSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = PlatformSettings.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = PlatformSettings.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = PlatformSettings.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = PlatformSettings.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = PlatformSettings.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o PlatformSettingSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("platform_settings", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o PlatformSettingSlice) copyMatchingRows(from ...*PlatformSetting) {
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
func (o PlatformSettingSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return PlatformSettings.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *PlatformSetting:
				o.copyMatchingRows(retrieved)
			case []*PlatformSetting:
				o.copyMatchingRows(retrieved...)
			case PlatformSettingSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a PlatformSetting or a slice of PlatformSetting
				// then run the AfterUpdateHooks on the slice
				_, err = PlatformSettings.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o PlatformSettingSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return PlatformSettings.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *PlatformSetting:
				o.copyMatchingRows(retrieved)
			case []*PlatformSetting:
				o.copyMatchingRows(retrieved...)
			case PlatformSettingSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a PlatformSetting or a slice of PlatformSetting
				// then run the AfterDeleteHooks on the slice
				_, err = PlatformSettings.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o PlatformSettingSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return PlatformSettings.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *PlatformSetting:
				o.copyMatchingRows(retrieved)
			case []*PlatformSetting:
				o.copyMatchingRows(retrieved...)
			case PlatformSettingSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a PlatformSetting or a slice of PlatformSetting
				// then run the AfterMergeHooks on the slice
				_, err = PlatformSettings.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o PlatformSettingSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals PlatformSettingSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := PlatformSettings.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o PlatformSettingSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := PlatformSettings.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o PlatformSettingSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := PlatformSettings.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type platformSettingWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, string]
	UpdatedAt psql.WhereMod[Q, time.Time]
	Data      psql.WhereMod[Q, json.RawMessage]
}

func (platformSettingWhere[Q]) AliasedAs(alias string) platformSettingWhere[Q] {
	return buildPlatformSettingWhere[Q](buildPlatformSettingColumns(alias))
}

func buildPlatformSettingWhere[Q psql.Filterable](cols platformSettingColumns) platformSettingWhere[Q] {
	return platformSettingWhere[Q]{
		ID:        psql.Where[Q, string](cols.ID.Expression),
		UpdatedAt: psql.Where[Q, time.Time](cols.UpdatedAt.Expression),
		Data:      psql.Where[Q, json.RawMessage](cols.Data.Expression),
	}
}
