package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
)

// QueryBuilder wraps squirrel.StatementBuilderType and provides a DB handle
// for building and executing queries in a robust, testable way.
type QueryBuilder struct {
	DB       *sqlx.DB
	Squirrel squirrel.StatementBuilderType
}

type Eq = squirrel.Eq
type NotEq = squirrel.NotEq

type Sqlizer = squirrel.Sqlizer
type ILike = squirrel.ILike
type And = squirrel.And

// PlaceholderDollar := squirrel.Dollar

// NewQueryBuilder creates a new QueryBuilder with the given sqlx.DB
func NewQueryBuilder(db *sql.DB) *QueryBuilder {
	sqlxDB := sqlx.NewDb(db, "postgres")
	return &QueryBuilder{
		DB:       sqlxDB,
		Squirrel: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// SelectBuilder returns a new squirrel.SelectBuilder
func (qb *QueryBuilder) SelectBuilder() squirrel.SelectBuilder {
	return qb.Squirrel.Select()
}

// InsertBuilder returns a new squirrel.InsertBuilder
func (qb *QueryBuilder) InsertBuilder(table string) squirrel.InsertBuilder {
	return qb.Squirrel.Insert(table)
}

// UpdateBuilder returns a new squirrel.UpdateBuilder
func (qb *QueryBuilder) UpdateBuilder(table string) squirrel.UpdateBuilder {
	return qb.Squirrel.Update(table)
}

// DeleteBuilder returns a new squirrel.DeleteBuilder
func (qb *QueryBuilder) DeleteBuilder(table string) squirrel.DeleteBuilder {
	return qb.Squirrel.Delete(table)
}

// Exec executes a query built with squirrel and returns sql.Result
func (qb *QueryBuilder) Exec(ctx context.Context, builder Sqlizer) (sql.Result, error) {
	query, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build SQL: %w", err)
	}
	return qb.DB.ExecContext(ctx, query, args...)
}

// QueryRowx runs a query and returns a single row (sqlx.Row)
func (qb *QueryBuilder) QueryRowx(ctx context.Context, builder Sqlizer) *sqlx.Row {
	query, args, err := builder.ToSql()
	if err != nil {
		return qb.DB.QueryRowxContext(ctx, "SELECT 1 WHERE 1=0") // always error
	}
	return qb.DB.QueryRowxContext(ctx, query, args...)
}

// Queryx runs a query and returns multiple rows (sqlx.Rows)
func (qb *QueryBuilder) Queryx(ctx context.Context, builder Sqlizer) (*sqlx.Rows, error) {
	query, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build SQL: %w", err)
	}
	return qb.DB.QueryxContext(ctx, query, args...)
}

// Get is a helper for SELECT ... LIMIT 1 into dest
func (qb *QueryBuilder) Get(ctx context.Context, dest interface{}, builder Sqlizer) error {
	query, args, err := builder.ToSql()

	if err != nil {
		return fmt.Errorf("failed to build SQL: %s %w", query, err)
	}
	return qb.DB.GetContext(ctx, dest, query, args...)
}

/*
*
// Select is a helper for SELECT ... into dest (slice)
//  Type check: dest must be a pointer to a slice
//   destType := reflect.TypeOf(dest)
//   if destType.Kind() != reflect.Ptr || destType.Elem().Kind() != reflect.Slice {
//       return fmt.Errorf("Select: dest must be a pointer to a slice, got %T", dest)
//   }
*/
func (qb *QueryBuilder) Select(ctx context.Context, dest interface{}, builder Sqlizer) error {
	query, args, err := builder.ToSql()
	fmt.Printf("running Query: %s | args: %v", query, args)
	if err != nil {
		return fmt.Errorf("failed to build SQL: %s %w", query, err)
	}
	return qb.DB.SelectContext(ctx, dest, query, args...)
}
