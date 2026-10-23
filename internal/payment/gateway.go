package payment

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Clock allows mocking time and sleep durations in unit tests without introducing real-world delays.
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

// RealClock implements Clock using the standard time package.
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}

func (RealClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

// PaymentGateway abstracts third-party payment provider communication (e.g. Razorpay, Juspay, Stripe).
// Notice: Authorize accepts a stableIdempotencyKey derived deterministically from the order ID,
// ensuring that retrying after a network timeout cannot result in double debiting.
type PaymentGateway interface {
	// Authorize places a pre-authorization hold on customer funds using a stable idempotency key.
	Authorize(ctx context.Context, orderID uuid.UUID, amountPaise int64, stableIdempotencyKey string) (reference string, err error)

	// Capture completes the transaction and transfers funds.
	Capture(ctx context.Context, orderID uuid.UUID, reference string) error

	// Refund reverses the transaction idempotently.
	Refund(ctx context.Context, paymentID uuid.UUID, amountPaise int64, reason string) (refundRef string, err error)
}
