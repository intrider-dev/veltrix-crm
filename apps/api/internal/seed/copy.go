package seed

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type generatedRows struct {
	count   int64
	current int64
	row     func(int64) []any
	err     error
}

func (rows *generatedRows) Next() bool {
	if rows.err != nil || rows.current >= rows.count {
		return false
	}
	rows.current++
	return true
}

func (rows *generatedRows) Values() ([]any, error) {
	if rows.current == 0 || rows.current > rows.count {
		return nil, fmt.Errorf("seed row source is not positioned on a row")
	}
	return rows.row(rows.current - 1), nil
}

func (rows *generatedRows) Err() error {
	return rows.err
}

func copyGenerated(
	ctx context.Context,
	tx pgx.Tx,
	table pgx.Identifier,
	columns []string,
	count int64,
	row func(int64) []any,
) error {
	if count == 0 {
		return nil
	}
	copied, err := tx.CopyFrom(ctx, table, columns, &generatedRows{count: count, row: row})
	if err != nil {
		return fmt.Errorf("copy %s: %w", table.Sanitize(), err)
	}
	if copied != count {
		return fmt.Errorf("copy %s: expected %d rows, copied %d", table.Sanitize(), count, copied)
	}
	return nil
}
