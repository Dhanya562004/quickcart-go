package payment

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrPaymentNotFound is returned when querying a non-existent payment record.
	ErrPaymentNotFound = errors.New("payment record not found")

	// ErrInvalidPaymentState indicates an invalid state transition.
	ErrInvalidPaymentState = errors.New("invalid payment state transition")

	// ErrPaymentAlreadyProcessed indicates payment is already in terminal captured state.
	ErrPaymentAlreadyProcessed = errors.New("payment already captured")

	// ErrPaymentAlreadyRefunded indicates payment is already refunded.
	ErrPaymentAlreadyRefunded = errors.New("payment already refunded")

	// ErrMaxRetriesExceeded indicates that payment retry attempts have been exhausted.
	ErrMaxRetriesExceeded = errors.New("maximum payment retry attempts exceeded")

	// ErrTransientGatewayError represents a recoverable payment gateway failure.
	ErrTransientGatewayError = errors.New("transient payment gateway failure")

	// ErrTerminalGatewayError represents an unrecoverable failure (e.g. invalid credentials or stolen card).
	ErrTerminalGatewayError = errors.New("terminal payment gateway failure")
)

// PaymentStatus represents the lifecycle state of a payment.
type PaymentStatus string

const (
	StatusPending    PaymentStatus = "PENDING"
	StatusAuthorized PaymentStatus = "AUTHORIZED"
	StatusCaptured   PaymentStatus = "CAPTURED"
	StatusFailed     PaymentStatus = "FAILED"
	StatusRefunded   PaymentStatus = "REFUNDED"
)

// PaymentMethod specifies the customer's payment medium.
type PaymentMethod string

const (
	MethodUPI        PaymentMethod = "UPI"
	MethodCard       PaymentMethod = "CARD"
	MethodNetbanking PaymentMethod = "NETBANKING"
	MethodWallet     PaymentMethod = "WALLET"
)

// Payment maps directly to the payments table.
type Payment struct {
	ID                   uuid.UUID     `json:"id"`
	OrderID              uuid.UUID     `json:"order_id"`
	Attempt              int           `json:"attempt"`
	IdempotencyKey       string        `json:"idempotency_key"`
	AmountPaise          int64         `json:"amount_paise"`
	Currency             string        `json:"currency"`
	Status               PaymentStatus `json:"status"`
	PaymentMethod        PaymentMethod `json:"payment_method"`
	TransactionReference *string       `json:"transaction_reference,omitempty"`
	FailureReason        *string       `json:"failure_reason,omitempty"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
}

// Refund maps directly to the refunds table.
type Refund struct {
	ID          uuid.UUID `json:"id"`
	PaymentID   uuid.UUID `json:"payment_id"`
	AmountPaise int64     `json:"amount_paise"`
	Reason      string    `json:"reason"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}
