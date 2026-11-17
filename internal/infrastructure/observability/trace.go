package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

type traceIDContextKey struct{}

var fallbackTraceSequence atomic.Uint64

func NewTraceID() string {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err == nil {
		return hex.EncodeToString(random)
	}

	return fmt.Sprintf(
		"%016x%016x",
		uint64(time.Now().UnixNano()),
		fallbackTraceSequence.Add(1),
	)
}

func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDContextKey{}, traceID)
}

func TraceIDFromContext(ctx context.Context) string {
	traceID, _ := ctx.Value(traceIDContextKey{}).(string)
	return traceID
}
