package client

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Test_retryUnaryWithBackoff_RetriesTransientErrors проверяет повтор читающего унарного gRPC-запроса после временных
// ошибок.
func Test_retryUnaryWithBackoff_RetriesTransientErrors(t *testing.T) {
	// Arrange
	delays := []time.Duration{0, 0}
	calls := 0

	// Act
	err := retryUnaryWithBackoff(
		context.Background(),
		delays,
		func(context.Context) error {
			calls++
			if calls < 3 {
				return status.Error(codes.Unavailable, "server is temporarily unavailable")
			}
			return nil
		},
	)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 3, calls)
}

// Test_retryUnaryWithBackoff_DoesNotRetryPermanentErrors проверяет, что постоянные ошибки не повторяются.
func Test_retryUnaryWithBackoff_DoesNotRetryPermanentErrors(t *testing.T) {
	// Arrange
	calls := 0

	// Act
	err := retryUnaryWithBackoff(
		context.Background(),
		[]time.Duration{0},
		func(context.Context) error {
			calls++
			return status.Error(codes.InvalidArgument, "invalid request")
		},
	)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Equal(t, 1, calls)
}

// Test_retryUnaryWithBackoff_StopsWhenContextCanceled проверяет, что backoff не игнорирует отмену context.
func Test_retryUnaryWithBackoff_StopsWhenContextCanceled(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0

	// Act
	err := retryUnaryWithBackoff(
		ctx,
		[]time.Duration{10 * time.Millisecond},
		func(context.Context) error {
			calls++
			cancel()
			return status.Error(codes.Unavailable, "server is temporarily unavailable")
		},
	)

	// Assert
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, calls)
}
