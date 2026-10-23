package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

// RetryPolicy defines backoff and timeout parameters for payment operations.
type RetryPolicy struct {
	MaxAttempts    int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	AttemptTimeout time.Duration
}

// DefaultRetryPolicy provides standard production-grade backoff parameters.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:    3,
		BaseDelay:      100 * time.Millisecond,
		MaxDelay:       2 * time.Second,
		AttemptTimeout: 2 * time.Second,
	}
}

// Service manages payment execution, retries, and idempotent refunds.
type Service struct {
	repo    Repository
	gateway PaymentGateway
	clock   Clock
	policy  RetryPolicy
	logger  *slog.Logger
}

// NewService constructs a payment Service.
func NewService(repo Repository, gateway PaymentGateway, clock Clock, policy RetryPolicy, logger *slog.Logger) *Service {
	if clock == nil {
		clock = RealClock{}
	}
	if policy.MaxAttempts <= 0 {
		policy = DefaultRetryPolicy()
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		repo:    repo,
		gateway: gateway,
		clock:   clock,
		policy:  policy,
		logger:  logger,
	}
}

// ProcessPaymentRequest specifies the input for executing a customer payment.
type ProcessPaymentRequest struct {
	OrderID     uuid.UUID
	AmountPaise int64
	Currency    string
	Method      PaymentMethod
}

// ProcessPayment coordinates authorization and capture with exponential backoff and full jitter.
// Critical Guarantee:
// - Generates a STABLE idempotency key: fmt.Sprintf("pay_order_%s", req.OrderID)
// - Every retry attempt uses this identical idempotency key to prevent double charging.
// - Each distinct attempt is logged to the payments table with an incremented attempt counter (order_id, attempt).
func (s *Service) ProcessPayment(ctx context.Context, req ProcessPaymentRequest) (Payment, error) {
	if req.AmountPaise <= 0 {
		return Payment{}, errors.New("amount must be greater than zero")
	}
	if req.Currency == "" {
		req.Currency = "INR"
	}
	if req.Method == "" {
		req.Method = MethodUPI
	}

	stableKey := fmt.Sprintf("pay_order_%s", req.OrderID.String())

	var lastErr error
	for attempt := 1; attempt <= s.policy.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return Payment{}, ctx.Err()
		default:
		}

		s.logger.Info("initiating payment attempt",
			"order_id", req.OrderID,
			"attempt", attempt,
			"amount_paise", req.AmountPaise,
			"idempotency_key", stableKey,
		)

		// 1. Record attempt in database with PENDING status
		pRecord, err := s.repo.CreateAttempt(ctx, Payment{
			OrderID:        req.OrderID,
			Attempt:        attempt,
			IdempotencyKey: stableKey,
			AmountPaise:    req.AmountPaise,
			Currency:       req.Currency,
			Status:         StatusPending,
			PaymentMethod:  req.Method,
		})
		if err != nil {
			s.logger.Error("failed to record payment attempt in database",
				"order_id", req.OrderID,
				"attempt", attempt,
				"error", err,
			)
			return Payment{}, fmt.Errorf("failed to create payment attempt record: %w", err)
		}

		// 2. Authorize with PaymentGateway using per-attempt timeout and stable idempotency key
		attemptCtx, attemptCancel := context.WithTimeout(ctx, s.policy.AttemptTimeout)
		authRef, authErr := s.gateway.Authorize(attemptCtx, req.OrderID, req.AmountPaise, stableKey)
		attemptCancel()

		if authErr == nil {
			// Pre-authorization succeeded! Now Capture the funds
			captureCtx, captureCancel := context.WithTimeout(ctx, s.policy.AttemptTimeout)
			captureErr := s.gateway.Capture(captureCtx, req.OrderID, authRef)
			captureCancel()

			if captureErr == nil {
				// Success! Transition to CAPTURED
				capturedPayment, uErr := s.repo.UpdateStatus(ctx, pRecord.ID, StatusCaptured, &authRef, nil)
				if uErr != nil {
					s.logger.Error("failed to update payment status to CAPTURED", "payment_id", pRecord.ID, "error", uErr)
					return capturedPayment, nil // Funds were captured successfully
				}
				s.logger.Info("payment successfully captured",
					"order_id", req.OrderID,
					"payment_id", pRecord.ID,
					"reference", authRef,
				)
				return capturedPayment, nil
			}

			// Capture failed
			failReason := captureErr.Error()
			_, _ = s.repo.UpdateStatus(ctx, pRecord.ID, StatusFailed, &authRef, &failReason)
			lastErr = captureErr
		} else {
			// Authorization failed
			failReason := authErr.Error()
			pRecord, _ = s.repo.UpdateStatus(ctx, pRecord.ID, StatusFailed, nil, &failReason)
			lastErr = authErr

			// If error is terminal (unrecoverable card decline, insufficient funds), do not retry
			if errors.Is(authErr, ErrTerminalGatewayError) {
				s.logger.Warn("terminal payment error encountered; aborting retries",
					"order_id", req.OrderID,
					"error", authErr,
				)
				return pRecord, authErr
			}
		}

		// Exponential backoff with Full Jitter before next attempt
		if attempt < s.policy.MaxAttempts {
			delay := s.calculateBackoff(attempt)
			s.logger.Warn("payment attempt failed; backing off before retry",
				"order_id", req.OrderID,
				"attempt", attempt,
				"delay", delay,
				"error", lastErr,
			)
			s.clock.Sleep(delay)
		}
	}

	return Payment{}, fmt.Errorf("%w: %v", ErrMaxRetriesExceeded, lastErr)
}

// RefundPayment processes a refund idempotently.
// If the payment is already refunded, it returns the existing refund record without re-calling the gateway.
func (s *Service) RefundPayment(ctx context.Context, paymentID uuid.UUID, reason string) (Refund, error) {
	if reason == "" {
		reason = "Customer cancellation or out-of-stock reconciliation"
	}

	// 1. Check existing refunds for idempotency
	existingRefunds, err := s.repo.GetRefundsByPaymentID(ctx, paymentID)
	if err == nil && len(existingRefunds) > 0 {
		s.logger.Info("idempotent refund: returning existing refund record",
			"payment_id", paymentID,
			"refund_id", existingRefunds[0].ID,
		)
		return existingRefunds[0], nil
	}

	// 2. Call PaymentGateway to process refund
	refundRef, rErr := s.gateway.Refund(ctx, paymentID, 0, reason)
	if rErr != nil {
		s.logger.Error("gateway refund failed", "payment_id", paymentID, "error", rErr)
		return Refund{}, fmt.Errorf("gateway refund failed: %w", rErr)
	}

	// 3. Record refund in database
	refund, err := s.repo.CreateRefund(ctx, Refund{
		PaymentID:   paymentID,
		AmountPaise: 0, // In production, retrieved from original payment
		Reason:      reason,
		Status:      "COMPLETED",
	})
	if err != nil {
		s.logger.Error("failed to record refund in database", "payment_id", paymentID, "error", err)
		return Refund{}, err
	}

	// 4. Update payment status to REFUNDED
	_, _ = s.repo.UpdateStatus(ctx, paymentID, StatusRefunded, &refundRef, nil)

	s.logger.Info("payment successfully refunded",
		"payment_id", paymentID,
		"refund_id", refund.ID,
		"refund_ref", refundRef,
	)
	return refund, nil
}

// calculateBackoff computes exponential backoff with full jitter: Uniform(0, min(MaxDelay, BaseDelay * 2^(attempt-1)))
func (s *Service) calculateBackoff(attempt int) time.Duration {
	multiplier := 1 << (attempt - 1)
	maxForAttempt := s.policy.BaseDelay * time.Duration(multiplier)
	if maxForAttempt > s.policy.MaxDelay {
		maxForAttempt = s.policy.MaxDelay
	}

	// Full jitter: random duration between 0 and maxForAttempt
	if maxForAttempt <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(maxForAttempt)))
}
