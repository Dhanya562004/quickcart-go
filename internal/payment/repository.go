package payment

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Dhanya562004/quickcart-go/internal/db"
)

// Repository abstracts database operations for payments and refunds.
type Repository interface {
	CreateAttempt(ctx context.Context, p Payment) (Payment, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status PaymentStatus, ref *string, failReason *string) (Payment, error)
	GetPaymentByOrderAndAttempt(ctx context.Context, orderID uuid.UUID, attempt int) (Payment, error)
	GetLatestPaymentForOrder(ctx context.Context, orderID uuid.UUID) (Payment, error)
	CreateRefund(ctx context.Context, refund Refund) (Refund, error)
	GetRefundsByPaymentID(ctx context.Context, paymentID uuid.UUID) ([]Refund, error)
}

// pgxRepository implements Repository using sqlc generated queries.
type pgxRepository struct {
	queries db.Querier
}

// NewRepository creates a new pgx-backed payment repository.
func NewRepository(queries db.Querier) Repository {
	return &pgxRepository{queries: queries}
}

func toPGUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

func fromPGUUID(p pgtype.UUID) uuid.UUID {
	return p.Bytes
}

func toPGText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func fromPGText(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func (r *pgxRepository) CreateAttempt(ctx context.Context, p Payment) (Payment, error) {
	row, err := r.queries.CreatePaymentAttempt(ctx, db.CreatePaymentAttemptParams{
		OrderID:              toPGUUID(p.OrderID),
		Attempt:              int32(p.Attempt),
		IdempotencyKey:       p.IdempotencyKey,
		AmountPaise:          p.AmountPaise,
		Currency:             p.Currency,
		Status:               string(p.Status),
		PaymentMethod:        string(p.PaymentMethod),
		TransactionReference: toPGText(p.TransactionReference),
		FailureReason:        toPGText(p.FailureReason),
	})
	if err != nil {
		return Payment{}, err
	}
	return mapDbPaymentToModel(row), nil
}

func (r *pgxRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status PaymentStatus, ref *string, failReason *string) (Payment, error) {
	row, err := r.queries.UpdatePaymentStatus(ctx, db.UpdatePaymentStatusParams{
		ID:                   toPGUUID(id),
		Status:               string(status),
		TransactionReference: toPGText(ref),
		FailureReason:        toPGText(failReason),
	})
	if err != nil {
		return Payment{}, err
	}
	return mapDbPaymentToModel(row), nil
}

func (r *pgxRepository) GetPaymentByOrderAndAttempt(ctx context.Context, orderID uuid.UUID, attempt int) (Payment, error) {
	row, err := r.queries.GetPaymentByOrderAndAttempt(ctx, db.GetPaymentByOrderAndAttemptParams{
		OrderID: toPGUUID(orderID),
		Attempt: int32(attempt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Payment{}, ErrPaymentNotFound
		}
		return Payment{}, err
	}
	return mapDbPaymentToModel(row), nil
}

func (r *pgxRepository) GetLatestPaymentForOrder(ctx context.Context, orderID uuid.UUID) (Payment, error) {
	row, err := r.queries.GetLatestPaymentForOrder(ctx, toPGUUID(orderID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Payment{}, ErrPaymentNotFound
		}
		return Payment{}, err
	}
	return mapDbPaymentToModel(row), nil
}

func (r *pgxRepository) CreateRefund(ctx context.Context, refund Refund) (Refund, error) {
	row, err := r.queries.CreateRefund(ctx, db.CreateRefundParams{
		PaymentID:   toPGUUID(refund.PaymentID),
		AmountPaise: refund.AmountPaise,
		Reason:      refund.Reason,
		Status:      refund.Status,
	})
	if err != nil {
		return Refund{}, err
	}
	return Refund{
		ID:          fromPGUUID(row.ID),
		PaymentID:   fromPGUUID(row.PaymentID),
		AmountPaise: row.AmountPaise,
		Reason:      row.Reason,
		Status:      row.Status,
		CreatedAt:   row.CreatedAt.Time,
	}, nil
}

func (r *pgxRepository) GetRefundsByPaymentID(ctx context.Context, paymentID uuid.UUID) ([]Refund, error) {
	rows, err := r.queries.GetRefundsByPaymentID(ctx, toPGUUID(paymentID))
	if err != nil {
		return nil, err
	}
	out := make([]Refund, len(rows))
	for i, row := range rows {
		out[i] = Refund{
			ID:          fromPGUUID(row.ID),
			PaymentID:   fromPGUUID(row.PaymentID),
			AmountPaise: row.AmountPaise,
			Reason:      row.Reason,
			Status:      row.Status,
			CreatedAt:   row.CreatedAt.Time,
		}
	}
	return out, nil
}

func mapDbPaymentToModel(p db.Payment) Payment {
	return Payment{
		ID:                   fromPGUUID(p.ID),
		OrderID:              fromPGUUID(p.OrderID),
		Attempt:              int(p.Attempt),
		IdempotencyKey:       p.IdempotencyKey,
		AmountPaise:          p.AmountPaise,
		Currency:             p.Currency,
		Status:               PaymentStatus(p.Status),
		PaymentMethod:        PaymentMethod(p.PaymentMethod),
		TransactionReference: fromPGText(p.TransactionReference),
		FailureReason:        fromPGText(p.FailureReason),
		CreatedAt:            p.CreatedAt.Time,
		UpdatedAt:            p.UpdatedAt.Time,
	}
}

// FakeRepository is a thread-safe mock repository for unit testing.
type FakeRepository struct {
	mu       sync.Mutex
	payments map[uuid.UUID][]Payment
	refunds  map[uuid.UUID][]Refund
}

// NewFakeRepository initializes an empty FakeRepository.
func NewFakeRepository() *FakeRepository {
	return &FakeRepository{
		payments: make(map[uuid.UUID][]Payment),
		refunds:  make(map[uuid.UUID][]Refund),
	}
}

func (f *FakeRepository) CreateAttempt(ctx context.Context, p Payment) (Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()

	attempts := f.payments[p.OrderID]
	// Verify (order_id, attempt) uniqueness
	for _, a := range attempts {
		if a.Attempt == p.Attempt {
			return Payment{}, errors.New("unique constraint violation: uq_payment_order_attempt")
		}
	}

	f.payments[p.OrderID] = append(f.payments[p.OrderID], p)
	return p, nil
}

func (f *FakeRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status PaymentStatus, ref *string, failReason *string) (Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for orderID, list := range f.payments {
		for i, p := range list {
			if p.ID == id {
				list[i].Status = status
				list[i].TransactionReference = ref
				list[i].FailureReason = failReason
				list[i].UpdatedAt = time.Now()
				f.payments[orderID] = list
				return list[i], nil
			}
		}
	}
	return Payment{}, ErrPaymentNotFound
}

func (f *FakeRepository) GetPaymentByOrderAndAttempt(ctx context.Context, orderID uuid.UUID, attempt int) (Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, p := range f.payments[orderID] {
		if p.Attempt == attempt {
			return p, nil
		}
	}
	return Payment{}, ErrPaymentNotFound
}

func (f *FakeRepository) GetLatestPaymentForOrder(ctx context.Context, orderID uuid.UUID) (Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	list := f.payments[orderID]
	if len(list) == 0 {
		return Payment{}, ErrPaymentNotFound
	}
	return list[len(list)-1], nil
}

func (f *FakeRepository) CreateRefund(ctx context.Context, refund Refund) (Refund, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if refund.ID == uuid.Nil {
		refund.ID = uuid.New()
	}
	refund.CreatedAt = time.Now()
	f.refunds[refund.PaymentID] = append(f.refunds[refund.PaymentID], refund)
	return refund, nil
}

func (f *FakeRepository) GetRefundsByPaymentID(ctx context.Context, paymentID uuid.UUID) ([]Refund, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]Refund{}, f.refunds[paymentID]...), nil
}
