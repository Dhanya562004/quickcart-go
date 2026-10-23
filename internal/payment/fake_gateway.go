package payment

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// FakeGateway is a thread-safe mock implementation of PaymentGateway for testing retry schedules and failures.
type FakeGateway struct {
	mu sync.Mutex

	// FailuresToInject is the number of transient failures before succeeding.
	FailuresToInject int
	failureCount     int

	// TerminalFail indicates whether to inject an unrecoverable terminal error immediately.
	TerminalFail bool

	// Authorizations tracks (stableIdempotencyKey -> reference)
	Authorizations map[string]string

	// CapturedReferences tracks captured transactions
	CapturedReferences map[string]bool

	// Refunds tracks (paymentID -> refundRef)
	Refunds map[uuid.UUID]string
}

// NewFakeGateway initializes a fresh FakeGateway.
func NewFakeGateway() *FakeGateway {
	return &FakeGateway{
		Authorizations:     make(map[string]string),
		CapturedReferences: make(map[string]bool),
		Refunds:            make(map[uuid.UUID]string),
	}
}

func (f *FakeGateway) Authorize(ctx context.Context, orderID uuid.UUID, amountPaise int64, stableIdempotencyKey string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	if f.TerminalFail {
		return "", ErrTerminalGatewayError
	}

	if f.failureCount < f.FailuresToInject {
		f.failureCount++
		return "", ErrTransientGatewayError
	}

	// Idempotent authorization: return existing reference if key already seen
	if ref, exists := f.Authorizations[stableIdempotencyKey]; exists {
		return ref, nil
	}

	ref := fmt.Sprintf("txn_auth_%s", uuid.New().String()[:8])
	f.Authorizations[stableIdempotencyKey] = ref
	return ref, nil
}

func (f *FakeGateway) Capture(ctx context.Context, orderID uuid.UUID, reference string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	f.CapturedReferences[reference] = true
	return nil
}

func (f *FakeGateway) Refund(ctx context.Context, paymentID uuid.UUID, amountPaise int64, reason string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	if ref, exists := f.Refunds[paymentID]; exists {
		return ref, nil
	}

	ref := fmt.Sprintf("txn_ref_%s", uuid.New().String()[:8])
	f.Refunds[paymentID] = ref
	return ref, nil
}
