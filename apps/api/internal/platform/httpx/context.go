package httpx

import (
	"context"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
)

type contextKey uint8

const (
	requestIDKey contextKey = iota
	principalKey
)

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey).(string)
	return requestID
}

func WithPrincipal(ctx context.Context, principal identity.Principal) context.Context {
	return context.WithValue(ctx, principalKey, principal)
}

func Principal(ctx context.Context) (identity.Principal, bool) {
	principal, ok := ctx.Value(principalKey).(identity.Principal)
	return principal, ok
}
