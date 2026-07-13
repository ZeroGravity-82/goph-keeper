package grpcclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

// TestWithAccessToken проверяет добавление bearer-токена в исходящий gRPC-контекст.
func TestWithAccessToken(t *testing.T) {
	// Arrange
	ctx := context.Background()

	// Act
	ctx = WithAccessToken(ctx, "access-token")

	// Assert
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	assert.Equal(t, []string{"Bearer access-token"}, md.Get(authorizationMetadataKey))
}
