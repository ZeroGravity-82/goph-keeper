package grpcclient

import (
	"context"

	"google.golang.org/grpc/metadata"
)

const authorizationMetadataKey = "authorization"

// WithAccessToken добавляет access-токен в исходящий gRPC-контекст.
func WithAccessToken(ctx context.Context, accessToken string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, authorizationMetadataKey, "Bearer "+accessToken)
}
