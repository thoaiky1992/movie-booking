// Package payment provides PSP adapters. MockGateway is for local development
// and tests; a production adapter (Stripe, VNPay, MoMo, ...) implements the same
// port and must pass IdempotencyKey through to the provider.
package payment

import (
	"context"
	"fmt"
	"sync"

	"movie_booking/internal/domain"
	"movie_booking/internal/usecase"
)

type MockGateway struct {
	mu         sync.Mutex
	declineAll bool
	charges    map[string]string // idempotency key -> provider ref
	refunds    map[string]struct{}
}

func NewMockGateway(declineAll bool) *MockGateway {
	return &MockGateway{declineAll: declineAll, charges: map[string]string{}, refunds: map[string]struct{}{}}
}

var _ usecase.PaymentGateway = (*MockGateway)(nil)

func (g *MockGateway) Charge(_ context.Context, req usecase.ChargeRequest) (usecase.ChargeResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if ref, ok := g.charges[req.IdempotencyKey]; ok {
		return usecase.ChargeResult{ProviderRef: ref}, nil // replay returns the original result
	}
	if g.declineAll {
		return usecase.ChargeResult{}, domain.ErrPaymentDeclined
	}
	ref := fmt.Sprintf("mock_ch_%s", req.IdempotencyKey)
	g.charges[req.IdempotencyKey] = ref
	return usecase.ChargeResult{ProviderRef: ref}, nil
}

func (g *MockGateway) Refund(_ context.Context, req usecase.RefundRequest) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.refunds[req.IdempotencyKey] = struct{}{}
	return nil
}
