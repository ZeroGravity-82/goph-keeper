package grpcserver

import (
	"context"
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/auth"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/transport/grpcserver/authcontext"
)

type accessTokenParserStub struct {
	claims *auth.AccessClaims
	err    error
	called bool
}

func (s *accessTokenParserStub) ParseAccessToken(_ string) (*auth.AccessClaims, error) {
	s.called = true
	return s.claims, s.err
}

// TestAuthenticateUnary_SkipsAuthMethods проверяет, что методы аутентификации не требуют access-токен.
func TestAuthenticateUnary_SkipsAuthMethods(t *testing.T) {
	// Arrange
	parser := &accessTokenParserStub{err: errors.New("must not be called")}
	srv := &GRPCServer{tokenParser: parser}
	info := &grpc.UnaryServerInfo{FullMethod: pb.Auth_Register_FullMethodName}

	// Act
	resp, err := srv.authenticateInterceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.False(t, parser.called)
}

// TestAuthenticateUnary_AddsUserIDToContext проверяет успешную аутентификацию Records-метода.
func TestAuthenticateUnary_AddsUserIDToContext(t *testing.T) {
	// Arrange
	userID, err := uuid.NewV7()
	require.NoError(t, err)
	parser := &accessTokenParserStub{claims: &auth.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	}}
	srv := &GRPCServer{tokenParser: parser}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer access-token"))
	info := &grpc.UnaryServerInfo{FullMethod: pb.Records_CreateRecord_FullMethodName}

	// Act
	resp, err := srv.authenticateInterceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		actualUserID, ok := authcontext.UserIDFromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, userID, actualUserID)
		return "ok", nil
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.True(t, parser.called)
}

// TestAuthenticateUnary_FailsWithoutAuthorization проверяет ошибку при отсутствии метаданных authorization.
func TestAuthenticateUnary_FailsWithoutAuthorization(t *testing.T) {
	// Arrange
	srv := &GRPCServer{tokenParser: &accessTokenParserStub{}}
	info := &grpc.UnaryServerInfo{FullMethod: pb.Records_CreateRecord_FullMethodName}

	// Act
	_, err := srv.authenticateInterceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, errors.New("handler must not be called")
	})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAuthenticateUnary_FailsWithInvalidBearerScheme проверяет ошибку при неверном формате authorization metadata.
func TestAuthenticateUnary_FailsWithInvalidBearerScheme(t *testing.T) {
	// Arrange
	srv := &GRPCServer{tokenParser: &accessTokenParserStub{}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Basic token"))
	info := &grpc.UnaryServerInfo{FullMethod: pb.Records_CreateRecord_FullMethodName}

	// Act
	_, err := srv.authenticateInterceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, errors.New("handler must not be called")
	})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAuthenticateUnary_FailsWithInvalidToken проверяет ошибку при невалидном access-токене.
func TestAuthenticateUnary_FailsWithInvalidToken(t *testing.T) {
	// Arrange
	srv := &GRPCServer{tokenParser: &accessTokenParserStub{err: auth.ErrInvalidToken}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer invalid-token"))
	info := &grpc.UnaryServerInfo{FullMethod: pb.Records_CreateRecord_FullMethodName}

	// Act
	_, err := srv.authenticateInterceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, errors.New("handler must not be called")
	})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAuthenticateUnary_FailsWithInvalidSubject проверяет ошибку, если subject access-токена не является UUID.
func TestAuthenticateUnary_FailsWithInvalidSubject(t *testing.T) {
	// Arrange
	parser := &accessTokenParserStub{claims: &auth.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: "not-a-uuid"},
	}}
	srv := &GRPCServer{tokenParser: parser}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer access-token"))
	info := &grpc.UnaryServerInfo{FullMethod: pb.Records_CreateRecord_FullMethodName}

	// Act
	_, err := srv.authenticateInterceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, errors.New("handler must not be called")
	})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}
