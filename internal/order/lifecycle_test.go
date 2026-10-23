package order

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrderLifecycle_StateTransitions(t *testing.T) {
	allStatuses := []string{
		StatusPlaced,
		StatusConfirmed,
		StatusPacked,
		StatusOutForDelivery,
		StatusDelivered,
		StatusCancelled,
		StatusFailed,
	}

	tests := []struct {
		from    string
		to      string
		allowed bool
	}{
		// Allowed transitions
		{StatusPlaced, StatusConfirmed, true},
		{StatusPlaced, StatusCancelled, true},
		{StatusPlaced, StatusFailed, true},
		{StatusConfirmed, StatusPacked, true},
		{StatusConfirmed, StatusCancelled, true},
		{StatusPacked, StatusOutForDelivery, true},
		{StatusOutForDelivery, StatusDelivered, true},

		// Explicit forbidden transitions
		{StatusPlaced, StatusPacked, false},
		{StatusPlaced, StatusOutForDelivery, false},
		{StatusPlaced, StatusDelivered, false},
		{StatusConfirmed, StatusOutForDelivery, false},
		{StatusConfirmed, StatusDelivered, false},
		{StatusConfirmed, StatusFailed, false},
		{StatusPacked, StatusCancelled, false},
		{StatusPacked, StatusDelivered, false},
		{StatusOutForDelivery, StatusCancelled, false},
		{StatusOutForDelivery, StatusConfirmed, false},
		{StatusDelivered, StatusCancelled, false},
		{StatusDelivered, StatusPlaced, false},
		{StatusCancelled, StatusPlaced, false},
		{StatusCancelled, StatusConfirmed, false},
		{StatusFailed, StatusPlaced, false},
		{StatusFailed, StatusConfirmed, false},
	}

	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			can := CanTransition(tt.from, tt.to)
			assert.Equal(t, tt.allowed, can)

			err := ValidateTransition(tt.from, tt.to)
			if tt.allowed {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, ErrInvalidStateTransition)
			}
		})
	}

	// Verify terminal states have zero outward transitions
	terminalStates := []string{StatusDelivered, StatusCancelled, StatusFailed}
	for _, term := range terminalStates {
		for _, target := range allStatuses {
			t.Run("terminal_"+term+"->"+target, func(t *testing.T) {
				assert.False(t, CanTransition(term, target))
				assert.Error(t, ValidateTransition(term, target))
			})
		}
	}
}

func TestOrderLifecycle_CanCancel(t *testing.T) {
	tests := []struct {
		status    string
		canCancel bool
	}{
		{StatusPlaced, true},
		{StatusConfirmed, true},
		{StatusPacked, false},
		{StatusOutForDelivery, false},
		{StatusDelivered, false},
		{StatusCancelled, false},
		{StatusFailed, false},
		{"UNKNOWN_STATUS", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			assert.Equal(t, tt.canCancel, CanCancel(tt.status))
		})
	}
}
