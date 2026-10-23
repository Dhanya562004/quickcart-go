package order

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidStateTransition = errors.New("invalid order status transition")
	ErrOrderCannotBeCancelled = errors.New("order can only be cancelled when in PLACED or CONFIRMED status")
)

// AllowedTransitions maps each origin state to its valid destination states.
var AllowedTransitions = map[string]map[string]bool{
	StatusPlaced: {
		StatusConfirmed: true,
		StatusCancelled: true,
		StatusFailed:    true,
	},
	StatusConfirmed: {
		StatusPacked:    true,
		StatusCancelled: true,
	},
	StatusPacked: {
		StatusOutForDelivery: true,
	},
	StatusOutForDelivery: {
		StatusDelivered: true,
	},
	StatusDelivered: {},
	StatusCancelled: {},
	StatusFailed:    {},
}

// CanTransition validates whether moving from 'current' to 'next' is permitted.
func CanTransition(current, next string) bool {
	destinations, exists := AllowedTransitions[current]
	if !exists {
		return false
	}
	return destinations[next]
}

// ValidateTransition returns a descriptive error if the transition is illegal.
func ValidateTransition(current, next string) error {
	if !CanTransition(current, next) {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidStateTransition, current, next)
	}
	return nil
}

// CanCancel checks if an order in the given status is eligible for cancellation.
func CanCancel(status string) bool {
	return status == StatusPlaced || status == StatusConfirmed
}
