package grpcserver

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/auth"
	"zerogravity-82/goph-keeper/internal/transport/grpcserver/authcontext"
)

const (
	authorizationMetadataKey       = "authorization"
	recordsServiceFullMethodPrefix = "/gophkeeper.v1.Records/"
)

type accessTokenParser interface {
	ParseAccessToken(tokenString string) (*auth.AccessClaims, error)
}

func (s *GRPCServer) authenticateInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	if !requiresAccessToken(info.FullMethod) {
		return handler(ctx, req)
	}

	accessToken, err := accessTokenFromContext(ctx)
	if err != nil {
		return nil, err
	}
	claims, err := s.tokenParser.ParseAccessToken(accessToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "access token is invalid")
	}
	if claims == nil {
		return nil, status.Error(codes.Unauthenticated, "access token is invalid")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil || userID == uuid.Nil {
		return nil, status.Error(codes.Unauthenticated, "access token subject is invalid")
	}

	return handler(authcontext.WithUserID(ctx, userID), req)
}

func requiresAccessToken(fullMethod string) bool {
	return strings.HasPrefix(fullMethod, recordsServiceFullMethodPrefix)
}

func accessTokenFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "authorization metadata is required")
	}
	values := md.Get(authorizationMetadataKey)
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "authorization metadata is required")
	}

	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", status.Error(codes.Unauthenticated, "authorization metadata must use Bearer token")
	}
	return parts[1], nil
}
