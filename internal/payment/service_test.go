package payment_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dhanya562004/quickcart-go/internal/payment"
)

// mockClock records all Sleep durations without actually pausing execution.
type mockClock struct {
	sleptDurations []time.Duration
}

func (m *mockClock) Now() time.Time {
	return time.Now()
}

func (m *mockClock) Sleep(d time.Duration) {
	m.sleptDurations = append(m.sleptDurations, d)
}

func TestPaymentService_ProcessPayment_FirstAttemptSuccess(t *testing.T) {
	repo := payment.NewFakeRepository()
	gateway := payment.NewFakeGateway()
	clock := &mockClock{}

	svc := payment.NewService(repo, gateway, clock, payment.DefaultRetryPolicy(), nil)

	orderID := uuid.New()
	p, err := svc.ProcessPayment(context.Background(), payment.ProcessPaymentRequest{
		OrderID:     orderID,
		AmountPaise: 49900,
		Method:      payment.MethodUPI,
	})

	require.NoError(t, err)
	assert.Equal(t, payment.StatusCaptured, p.Status)
	assert.Equal(t, 1, p.Attempt)
	assert.NotEmpty(t, p.TransactionReference)
	assert.Empty(t, clock.sleptDurations, "no backoff expected on first-attempt success")
}

func TestPaymentService_ProcessPayment_RetryOnTransientFailure(t *testing.T) {
	repo := payment.NewFakeRepository()
	gateway := payment.NewFakeGateway()
	gateway.FailuresToInject = 1 // Fail first attempt, succeed on second attempt
	clock := &mockClock{}

	svc := payment.NewService(repo, gateway, clock, payment.DefaultRetryPolicy(), nil)

	orderID := uuid.New()
	p, err := svc.ProcessPayment(context.Background(), payment.ProcessPaymentRequest{
		OrderID:     orderID,
		AmountPaise: 29900,
		Method:      payment.MethodCard,
	})

	require.NoError(t, err)
	assert.Equal(t, payment.StatusCaptured, p.Status)
	assert.Equal(t, 2, p.Attempt, "should succeed on attempt 2")
	assert.Len(t, clock.sleptDurations, 1, "exactly 1 backoff sleep expected")
}

func TestPaymentService_ProcessPayment_TerminalFailureAbortsImmediately(t *testing.T) {
	repo := payment.NewFakeRepository()
	gateway := payment.NewFakeGateway()
	gateway.TerminalFail = true // Unrecoverable failure
	clock := &mockClock{}

	svc := payment.NewService(repo, gateway, clock, payment.DefaultRetryPolicy(), nil)

	orderID := uuid.New()
	p, err := svc.ProcessPayment(context.Background(), payment.ProcessPaymentRequest{
		OrderID:     orderID,
		AmountPaise: 19900,
		Method:      payment.MethodNetbanking,
	})

	require.ErrorIs(t, err, payment.ErrTerminalGatewayError)
	assert.Equal(t, payment.StatusFailed, p.Status)
	assert.Equal(t, 1, p.Attempt)
	assert.Empty(t, clock.sleptDurations, "terminal failure must not trigger backoff retries")
}

func TestPaymentService_ProcessPayment_MaxRetriesExceeded(t *testing.T) {
	repo := payment.NewFakeRepository()
	gateway := payment.NewFakeGateway()
	gateway.FailuresToInject = 5 // More failures than MaxAttempts (3)
	clock := &mockClock{}

	svc := payment.NewService(repo, gateway, clock, payment.DefaultRetryPolicy(), nil)

	orderID := uuid.New()
	_, err := svc.ProcessPayment(context.Background(), payment.ProcessPaymentRequest{
		OrderID:     orderID,
		AmountPaise: 99900,
		Method:      payment.MethodUPI,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, payment.ErrMaxRetriesExceeded)
	assert.Len(t, clock.sleptDurations, 2, "must sleep before attempt 2 and attempt 3")
}

func TestPaymentService_Refund_Idempotency(t *testing.T) {
	repo := payment.NewFakeRepository()
	gateway := payment.NewFakeGateway()
	clock := &mockClock{}

	svc := payment.NewService(repo, gateway, clock, payment.DefaultRetryPolicy(), nil)

	orderID := uuid.New()
	captured, err := svc.ProcessPayment(context.Background(), payment.ProcessPaymentRequest{
		OrderID:     orderID,
		AmountPaise: 35000,
	})
	require.NoError(t, err)

	// First refund call
	ref1, err := svc.RefundPayment(context.Background(), captured.ID, "Customer request")
	require.NoError(t, err)
	assert.NotEmpty(t, ref1.ID)

	// Second refund call (must be strictly idempotent)
	ref2, err := svc.RefundPayment(context.Background(), captured.ID, "Customer request duplicate")
	require.NoError(t, err)
	assert.Equal(t, ref1.ID, ref2.ID, "duplicate refund call must return identical refund record")
	assert.Len(t, gateway.Refunds, 1, "gateway must only be invoked once")
}
