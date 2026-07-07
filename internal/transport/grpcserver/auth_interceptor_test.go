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
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
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

type userSessionCheckerStub struct {
	securityVersion int64
	err             error
	called          bool
}

func (s *userSessionCheckerStub) GetSecurityVersion(context.Context, uuid.UUID) (int64, error) {
	s.called = true
	if s.err != nil {
		return 0, s.err
	}
	if s.securityVersion > 0 {
		return s.securityVersion, nil
	}
	return 1, nil
}

// TestNewGRPCServer_RequiresDependencies проверяет валидацию обязательных зависимостей gRPC-сервера.
func TestNewGRPCServer_RequiresDependencies(t *testing.T) {
	tests := []struct {
		name           string
		addr           string
		creds          bool
		authService    pb.AuthServer
		recordsService pb.RecordsServer
		tokenParser    accessTokenParser
		sessionChecker UserSessionChecker
		wantErr        string
	}{
		{
			name:           "address",
			creds:          true,
			authService:    &pb.UnimplementedAuthServer{},
			recordsService: &pb.UnimplementedRecordsServer{},
			tokenParser:    &accessTokenParserStub{},
			sessionChecker: &userSessionCheckerStub{},
			wantErr:        "grpc server address is not provided",
		},
		{
			name:           "credentials",
			addr:           "127.0.0.1:0",
			authService:    &pb.UnimplementedAuthServer{},
			recordsService: &pb.UnimplementedRecordsServer{},
			tokenParser:    &accessTokenParserStub{},
			sessionChecker: &userSessionCheckerStub{},
			wantErr:        "transport credentials are not provided",
		},
		{
			name:           "auth service",
			addr:           "127.0.0.1:0",
			creds:          true,
			recordsService: &pb.UnimplementedRecordsServer{},
			tokenParser:    &accessTokenParserStub{},
			sessionChecker: &userSessionCheckerStub{},
			wantErr:        "auth service is not provided",
		},
		{
			name:           "records service",
			addr:           "127.0.0.1:0",
			creds:          true,
			authService:    &pb.UnimplementedAuthServer{},
			tokenParser:    &accessTokenParserStub{},
			sessionChecker: &userSessionCheckerStub{},
			wantErr:        "records service is not provided",
		},
		{
			name:           "token parser",
			addr:           "127.0.0.1:0",
			creds:          true,
			authService:    &pb.UnimplementedAuthServer{},
			recordsService: &pb.UnimplementedRecordsServer{},
			sessionChecker: &userSessionCheckerStub{},
			wantErr:        "access token parser is not provided",
		},
		{
			name:           "session checker",
			addr:           "127.0.0.1:0",
			creds:          true,
			authService:    &pb.UnimplementedAuthServer{},
			recordsService: &pb.UnimplementedRecordsServer{},
			tokenParser:    &accessTokenParserStub{},
			wantErr:        "user session checker is not provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var creds credentials.TransportCredentials
			if tt.creds {
				creds = insecure.NewCredentials()
			}

			// Act
			_, err := NewGRPCServer(
				tt.addr,
				creds,
				tt.authService,
				tt.recordsService,
				tt.tokenParser,
				tt.sessionChecker,
				nil,
			)

			// Assert
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}
}

// TestNewGRPCServer_CreatesServerWithDefaultLogger проверяет успешное создание сервера без явного логгера.
func TestNewGRPCServer_CreatesServerWithDefaultLogger(t *testing.T) {
	// Arrange
	parser := &accessTokenParserStub{}
	sessionChecker := &userSessionCheckerStub{}

	// Act
	srv, err := NewGRPCServer(
		"127.0.0.1:0",
		insecure.NewCredentials(),
		&pb.UnimplementedAuthServer{},
		&pb.UnimplementedRecordsServer{},
		parser,
		sessionChecker,
		nil,
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, srv)
	assert.Equal(t, "127.0.0.1:0", srv.addr)
	assert.NotNil(t, srv.logger)
	assert.Same(t, parser, srv.tokenParser)
	assert.Same(t, sessionChecker, srv.sessionChecker)
}

// TestAuthenticateUnary_SkipsAuthMethods проверяет, что методы аутентификации не требуют access-токен.
func TestAuthenticateUnary_SkipsAuthMethods(t *testing.T) {
	// Arrange
	parser := &accessTokenParserStub{err: errors.New("must not be called")}
	srv := &GRPCServer{tokenParser: parser}
	info := &grpc.UnaryServerInfo{FullMethod: pb.Auth_Register_FullMethodName}

	// Act
	resp, err := srv.authenticateUnaryInterceptor(
		context.Background(),
		nil,
		info,
		func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		},
	)

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
		SecurityVersion:  1,
	}}
	sessionChecker := &userSessionCheckerStub{securityVersion: 1}
	srv := &GRPCServer{tokenParser: parser, sessionChecker: sessionChecker}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer access-token"))
	info := &grpc.UnaryServerInfo{FullMethod: pb.Records_CreateRecord_FullMethodName}

	// Act
	resp, err := srv.authenticateUnaryInterceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		actualUserID, ok := authcontext.UserIDFromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, userID, actualUserID)
		return "ok", nil
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.True(t, parser.called)
	assert.True(t, sessionChecker.called)
}

// TestAuthenticateUnary_ProtectsChangeMasterKey проверяет, что смена мастер-ключа требует access-токен.
func TestAuthenticateUnary_ProtectsChangeMasterKey(t *testing.T) {
	// Arrange
	userID, err := uuid.NewV7()
	require.NoError(t, err)
	parser := &accessTokenParserStub{claims: &auth.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
		SecurityVersion:  1,
	}}
	sessionChecker := &userSessionCheckerStub{securityVersion: 1}
	srv := &GRPCServer{tokenParser: parser, sessionChecker: sessionChecker}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer access-token"))
	info := &grpc.UnaryServerInfo{FullMethod: pb.Auth_ChangeMasterKey_FullMethodName}

	// Act
	resp, err := srv.authenticateUnaryInterceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		actualUserID, ok := authcontext.UserIDFromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, userID, actualUserID)
		return "ok", nil
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.True(t, parser.called)
	assert.True(t, sessionChecker.called)
}

// TestAuthenticateStream_AddsUserIDToContext проверяет успешную аутентификацию streaming Records-метода.
func TestAuthenticateStream_AddsUserIDToContext(t *testing.T) {
	// Arrange
	userID, err := uuid.NewV7()
	require.NoError(t, err)
	parser := &accessTokenParserStub{claims: &auth.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
		SecurityVersion:  1,
	}}
	sessionChecker := &userSessionCheckerStub{securityVersion: 1}
	srv := &GRPCServer{tokenParser: parser, sessionChecker: sessionChecker}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer access-token"))
	stream := &authTestServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{FullMethod: pb.Records_UploadBinaryMultipartPart_FullMethodName}

	// Act
	err = srv.authenticateStreamInterceptor(nil, stream, info, func(_ any, stream grpc.ServerStream) error {
		actualUserID, ok := authcontext.UserIDFromContext(stream.Context())
		require.True(t, ok)
		assert.Equal(t, userID, actualUserID)
		return nil
	})

	// Assert
	require.NoError(t, err)
	assert.True(t, parser.called)
	assert.True(t, sessionChecker.called)
}

// TestAuthenticateUnary_FailsWithoutAuthorization проверяет ошибку при отсутствии метаданных authorization.
func TestAuthenticateUnary_FailsWithoutAuthorization(t *testing.T) {
	// Arrange
	srv := &GRPCServer{tokenParser: &accessTokenParserStub{}, sessionChecker: &userSessionCheckerStub{}}
	info := &grpc.UnaryServerInfo{FullMethod: pb.Records_CreateRecord_FullMethodName}

	// Act
	_, err := srv.authenticateUnaryInterceptor(
		context.Background(),
		nil,
		info,
		func(ctx context.Context, req any) (any, error) {
			return nil, errors.New("handler must not be called")
		},
	)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

type authTestServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authTestServerStream) Context() context.Context {
	return s.ctx
}

// TestAuthenticateUnary_FailsWithInvalidBearerScheme проверяет ошибку при неверном формате authorization metadata.
func TestAuthenticateUnary_FailsWithInvalidBearerScheme(t *testing.T) {
	// Arrange
	srv := &GRPCServer{tokenParser: &accessTokenParserStub{}, sessionChecker: &userSessionCheckerStub{}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Basic token"))
	info := &grpc.UnaryServerInfo{FullMethod: pb.Records_CreateRecord_FullMethodName}

	// Act
	_, err := srv.authenticateUnaryInterceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, errors.New("handler must not be called")
	})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAuthenticateUnary_FailsWithInvalidToken проверяет ошибку при невалидном access-токене.
func TestAuthenticateUnary_FailsWithInvalidToken(t *testing.T) {
	// Arrange
	srv := &GRPCServer{tokenParser: &accessTokenParserStub{err: auth.ErrInvalidToken}, sessionChecker: &userSessionCheckerStub{}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer invalid-token"))
	info := &grpc.UnaryServerInfo{FullMethod: pb.Records_CreateRecord_FullMethodName}

	// Act
	_, err := srv.authenticateUnaryInterceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
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
		SecurityVersion:  1,
	}}
	srv := &GRPCServer{tokenParser: parser, sessionChecker: &userSessionCheckerStub{}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer access-token"))
	info := &grpc.UnaryServerInfo{FullMethod: pb.Records_CreateRecord_FullMethodName}

	// Act
	_, err := srv.authenticateUnaryInterceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, errors.New("handler must not be called")
	})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAuthenticateUnary_FailsWithOutdatedSecurityVersion проверяет ошибку при устаревшей версии security-состояния
// пользователя в access-токене.
func TestAuthenticateUnary_FailsWithOutdatedSecurityVersion(t *testing.T) {
	// Arrange
	userID, err := uuid.NewV7()
	require.NoError(t, err)
	parser := &accessTokenParserStub{claims: &auth.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
		SecurityVersion:  1,
	}}
	sessionChecker := &userSessionCheckerStub{securityVersion: 2}
	srv := &GRPCServer{tokenParser: parser, sessionChecker: sessionChecker}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer access-token"))
	info := &grpc.UnaryServerInfo{FullMethod: pb.Records_CreateRecord_FullMethodName}

	// Act
	_, err = srv.authenticateUnaryInterceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, errors.New("handler must not be called")
	})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.True(t, sessionChecker.called)
}
