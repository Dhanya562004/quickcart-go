package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxFunc defines the callback signature executed inside a managed transaction.
type TxFunc func(tx pgx.Tx) error

// WithTx executes the supplied fn within a PostgreSQL transaction managed by pgx.
// If fn returns an error or panics, the transaction is automatically rolled back.
// If fn returns nil, the transaction is committed.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn TxFunc) (err error) {
	if pool == nil {
		return fmt.Errorf("cannot begin transaction: database pool is nil")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // re-throw panic after rollback
		} else if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
				err = fmt.Errorf("transaction rollback failed: %v (original error: %w)", rbErr, err)
			}
		} else {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				err = fmt.Errorf("failed to commit transaction: %w", commitErr)
			}
		}
	}()

	err = fn(tx)
	return err
}
