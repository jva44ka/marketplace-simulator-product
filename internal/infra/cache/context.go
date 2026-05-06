package cache

import "context"

type transactionIdKey struct{}

// WithTransactionId attaches the caller's known xmin to the context.
// Set by the gRPC handler when the request carries an optional transaction_id;
// read by CachedProductRepository to decide whether the cached entry is fresh enough.
func WithTransactionId(ctx context.Context, id uint32) context.Context {
	return context.WithValue(ctx, transactionIdKey{}, id)
}

// TransactionIdFromContext returns the xmin stored by WithTransactionId.
// ok is false when the caller did not supply a transaction_id.
func TransactionIdFromContext(ctx context.Context) (id uint32, ok bool) {
	id, ok = ctx.Value(transactionIdKey{}).(uint32)
	return
}
