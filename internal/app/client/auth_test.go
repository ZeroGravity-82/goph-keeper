package client

import (
	"context"
	"strings"
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

// TestApp_Register_FailsWithLongCredentials проверяет клиентские лимиты логина и пароля при регистрации.
func TestApp_Register_FailsWithLongCredentials(t *testing.T) {
	tests := []struct {
		name     string
		login    string
		password string
		want     string
	}{
		{
			name:     "long login",
			login:    strings.Repeat("a", userLoginMaxSizeChars+1),
			password: "password",
			want:     "логин не должен превышать 128 символов",
		},
		{
			name:     "long password",
			login:    "user",
			password: strings.Repeat("a", userPasswordMaxSizeChars+1),
			want:     "пароль не должен превышать 256 символов",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			app := &App{auth: &authClientStub{}}

			// Act
			_, err := app.Register(context.Background(), tt.login, tt.password, "master-key")

			// Assert
			require.Error(t, err)
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

// TestApp_Login_FailsWithLongCredentials проверяет клиентские лимиты логина и пароля при входе в аккаунт.
func TestApp_Login_FailsWithLongCredentials(t *testing.T) {
	tests := []struct {
		name     string
		login    string
		password string
		want     string
	}{
		{
			name:     "long login",
			login:    strings.Repeat("a", userLoginMaxSizeChars+1),
			password: "password",
			want:     "логин не должен превышать 128 символов",
		},
		{
			name:     "long password",
			login:    "user",
			password: strings.Repeat("a", userPasswordMaxSizeChars+1),
			want:     "пароль не должен превышать 256 символов",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			app := &App{auth: &authClientStub{}}

			// Act
			_, err := app.Login(context.Background(), tt.login, tt.password)

			// Assert
			require.Error(t, err)
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

// TestApp_Register_FailsWithLongMasterKey проверяет клиентский лимит длины мастер-ключа при регистрации.
func TestApp_Register_FailsWithLongMasterKey(t *testing.T) {
	// Arrange
	authClient := &authClientStub{}
	app := &App{auth: authClient}

	// Act
	_, err := app.Register(context.Background(), "user", "password", strings.Repeat("a", masterKeyMaxSizeChars+1))

	// Assert
	require.Error(t, err)
	assert.Equal(t, "мастер-ключ не должен превышать 256 символов", err.Error())
}

// TestApp_StartSession_FailsWithLongMasterKey проверяет клиентский лимит длины мастер-ключа при открытии сессии.
func TestApp_StartSession_FailsWithLongMasterKey(t *testing.T) {
	// Arrange
	app := newStartedTestApp(t)

	// Act
	err := app.StartSession(app.session, strings.Repeat("a", masterKeyMaxSizeChars+1))

	// Assert
	require.Error(t, err)
	assert.Equal(t, "мастер-ключ не должен превышать 256 символов", err.Error())
}

// TestApp_ChangeMasterKey_FailsWithLongMasterKeys проверяет клиентский лимит длины текущего и нового мастер-ключа.
func TestApp_ChangeMasterKey_FailsWithLongMasterKeys(t *testing.T) {
	tests := []struct {
		name    string
		current string
		new     string
		want    string
	}{
		{
			name:    "long current master key",
			current: strings.Repeat("a", masterKeyMaxSizeChars+1),
			new:     "new-master-key",
			want:    "текущий мастер-ключ не должен превышать 256 символов",
		},
		{
			name:    "long new master key",
			current: "master-key",
			new:     strings.Repeat("a", masterKeyMaxSizeChars+1),
			want:    "новый мастер-ключ не должен превышать 256 символов",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			app := newStartedTestApp(t)

			// Act
			err := app.ChangeMasterKey(context.Background(), tt.current, tt.new)

			// Assert
			require.Error(t, err)
			assert.Equal(t, tt.want, err.Error())
		})
	}
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

func (s *authClientStub) ChangeMasterKey(
	context.Context,
	*pb.ChangeMasterKeyRequest,
	...grpc.CallOption,
) (*pb.ChangeMasterKeyResponse, error) {
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
