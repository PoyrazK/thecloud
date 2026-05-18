// Package core provides core business services.
package core

import (
	"context"
)

// txKey is the context key for storing/retrieving a transaction.
type txKey struct{}

// WithTransaction stores a pgx.Tx in the context for transaction-aware repositories.
func WithTransaction(ctx context.Context, tx interface{}) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// TransactionFromContext retrieves a pgx.Tx from the context, or nil if not present.
func TransactionFromContext(ctx context.Context) interface{} {
	return ctx.Value(txKey{})
}
