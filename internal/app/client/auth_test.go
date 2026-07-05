package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/pb"
)

type authClientStub struct {
	refreshReq  *pb.RefreshRequest
	refreshResp *pb.RefreshResponse
	refreshErr  error
}

func (s *authClientStub) Register(
	context.Context,
	*pb.RegisterRequest,
	...grpc.CallOption,
) (*pb.RegisterResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (s *authClientStub) Login(
	context.Context,
	*pb.LoginRequest,
	...grpc.CallOption,
) (*pb.LoginResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (s *authClientStub) Refresh(
	_ context.Context,
	req *pb.RefreshRequest,
	_ ...grpc.CallOption,
) (*pb.RefreshResponse, error) {
	s.refreshReq = req
	return s.refreshResp, s.refreshErr
}

func (s *authClientStub) Logout(
	context.Context,
	*pb.LogoutRequest,
	...grpc.CallOption,
) (*pb.LogoutResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// TestApp_withAccessTokenRefresh_RefreshesAndRetries проверяет, что клиент обновляет пару токенов и повторяет запрос
// при истекшем access-токене.
func TestApp_withAccessTokenRefresh_RefreshesAndRetries(t *testing.T) {
	// Arrange
	oldRefreshToken := "old-refresh-token"
	newAccessToken := "new-access-token"
	newRefreshToken := "new-refresh-token"
	authClient := &authClientStub{
		refreshResp: pb.RefreshResponse_builder{
			AccessToken:  &newAccessToken,
			RefreshToken: &newRefreshToken,
		}.Build(),
	}
	app := &App{
		auth: authClient,
		session: AuthSession{
			AccessToken:  "old-access-token",
			RefreshToken: oldRefreshToken,
		},
		masterKey: "master-key",
		loggedIn:  true,
	}
	var tokens []string

	// Act
	err := app.withAccessTokenRefresh(context.Background(), func(ctx context.Context) error {
		tokens = append(tokens, outgoingAuthorization(t, ctx))
		if len(tokens) == 1 {
			return status.Error(codes.Unauthenticated, "access token is invalid")
		}
		return nil
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"Bearer old-access-token", "Bearer new-access-token"}, tokens)
	require.NotNil(t, authClient.refreshReq)
	assert.Equal(t, oldRefreshToken, authClient.refreshReq.GetRefreshToken())
	assert.Equal(t, newAccessToken, app.session.AccessToken)
	assert.Equal(t, newRefreshToken, app.session.RefreshToken)
}

// TestApp_withAccessTokenRefreshRetry_RetriesTransientError проверяет повтор читающего унарного gRPC-запроса с тем же
// access-токеном после временной сетевой ошибки.
func TestApp_withAccessTokenRefreshRetry_RetriesTransientError(t *testing.T) {
	// Arrange
	app := &App{
		session: AuthSession{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
		},
		masterKey: "master-key",
		loggedIn:  true,
	}
	var tokens []string

	// Act
	err := app.withAccessTokenRefreshRetry(context.Background(), func(ctx context.Context) error {
		tokens = append(tokens, outgoingAuthorization(t, ctx))
		if len(tokens) == 1 {
			return status.Error(codes.Unavailable, "server is temporarily unavailable")
		}
		return nil
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"Bearer access-token", "Bearer access-token"}, tokens)
}

func outgoingAuthorization(t *testing.T, ctx context.Context) string {
	t.Helper()

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	values := md.Get("authorization")
	require.Len(t, values, 1)
	return values[0]
}

// TestApp_LogoutClearsSession проверяет, что выход из аккаунта отправляет refresh-токен и очищает сессию в памяти.
func TestApp_LogoutClearsSession(t *testing.T) {
	// Arrange
	authClient := &authClientFake{}
	app := &App{
		auth: authClient,
		session: AuthSession{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
		},
		masterKey: "master-key",
		loggedIn:  true,
	}

	// Act
	err := app.Logout(context.Background())

	// Assert
	require.NoError(t, err)
	require.NotNil(t, authClient.logoutReq)
	assert.Equal(t, "refresh-token", authClient.logoutReq.GetRefreshToken())
	assert.False(t, app.loggedIn)
	assert.Empty(t, app.session.AccessToken)
	assert.Empty(t, app.session.RefreshToken)
	assert.Empty(t, app.masterKey)
}

// TestApp_withAccessTokenRefresh_ClearsSessionWhenRefreshTokenIsInvalid проверяет, что при недействительном
// refresh-токене клиент очищает сессию и не повторяет исходный запрос.
func TestApp_withAccessTokenRefresh_ClearsSessionWhenRefreshTokenIsInvalid(t *testing.T) {
	// Arrange
	authClient := &authClientStub{
		refreshErr: status.Error(codes.Unauthenticated, "refresh token is invalid"),
	}
	app := &App{
		auth: authClient,
		session: AuthSession{
			AccessToken:  "old-access-token",
			RefreshToken: "old-refresh-token",
		},
		masterKey: "master-key",
		loggedIn:  true,
	}
	calls := 0

	// Act
	err := app.withAccessTokenRefresh(context.Background(), func(context.Context) error {
		calls++
		return status.Error(codes.Unauthenticated, "access token is invalid")
	})

	// Assert
	require.Error(t, err)
	assert.Equal(t, "сессия истекла, войдите в аккаунт снова", err.Error())
	assert.Equal(t, 1, calls)
	assert.Empty(t, app.session.AccessToken)
	assert.Empty(t, app.session.RefreshToken)
	assert.Empty(t, app.masterKey)
	assert.False(t, app.loggedIn)
}
