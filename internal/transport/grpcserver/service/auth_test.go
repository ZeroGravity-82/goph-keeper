package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/logging"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/transport/grpcserver/authcontext"
	"zerogravity-82/goph-keeper/internal/usecase"
)

type authUseCaseStub struct {
	registerInput  usecase.RegisterInput
	registerOutput usecase.RegisterOutput
	registerErr    error

	loginInput  usecase.LoginInput
	loginOutput usecase.LoginOutput
	loginErr    error

	refreshInput  usecase.RefreshInput
	refreshOutput usecase.RefreshOutput
	refreshErr    error

	logoutInput usecase.LogoutInput
	logoutErr   error

	changeMasterKeyInput  usecase.ChangeMasterKeyInput
	changeMasterKeyOutput usecase.ChangeMasterKeyOutput
	changeMasterKeyErr    error
}

func (s *authUseCaseStub) Register(_ context.Context, in usecase.RegisterInput) (usecase.RegisterOutput, error) {
	s.registerInput = in
	return s.registerOutput, s.registerErr
}

func (s *authUseCaseStub) Login(_ context.Context, in usecase.LoginInput) (usecase.LoginOutput, error) {
	s.loginInput = in
	return s.loginOutput, s.loginErr
}

func (s *authUseCaseStub) Refresh(_ context.Context, in usecase.RefreshInput) (usecase.RefreshOutput, error) {
	s.refreshInput = in
	return s.refreshOutput, s.refreshErr
}

func (s *authUseCaseStub) Logout(_ context.Context, in usecase.LogoutInput) error {
	s.logoutInput = in
	return s.logoutErr
}

func (s *authUseCaseStub) ChangeMasterKey(
	_ context.Context,
	in usecase.ChangeMasterKeyInput,
) (usecase.ChangeMasterKeyOutput, error) {
	s.changeMasterKeyInput = in
	return s.changeMasterKeyOutput, s.changeMasterKeyErr
}

// TestAuthService_Register_OK проверяет успешную регистрацию через gRPC-обработчик.
func TestAuthService_Register_OK(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{registerOutput: usecase.RegisterOutput{
		AuthTokens: usecase.AuthTokens{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
		},
		MasterKeySalt: []byte("1234567890abcdef"),
	}}

	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := registerRequest("user", "password")

	// Act
	resp, err := authService.Register(context.Background(), req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "access-token", resp.GetAccessToken())
	assert.Equal(t, "refresh-token", resp.GetRefreshToken())
	assert.Equal(t, []byte("1234567890abcdef"), resp.GetMasterKeySalt())
	assert.Equal(t, "user", uc.registerInput.Login)
	assert.Equal(t, "password", uc.registerInput.Password)
	assert.Equal(t, []byte("1234567890abcdef"), uc.registerInput.MasterKeySalt)
	assert.Equal(t, []byte("verifier"), uc.registerInput.MasterKeyVerifier)
}

// TestAuthService_Register_FailWithInvalidArgument проверяет валидацию обязательных полей запроса.
func TestAuthService_Register_FailWithInvalidArgument(t *testing.T) {
	// Arrange
	tests := []struct {
		name string
		req  *pb.RegisterRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty login", req: registerRequest("", "password")},
		{name: "blank login", req: registerRequest("   ", "password")},
		{name: "empty password", req: registerRequest("user", "")},
		{name: "empty salt", req: registerRequestWithMasterKeyData("user", "password", nil, []byte("verifier"))},
		{name: "empty verifier", req: registerRequestWithMasterKeyData("user", "password", []byte("1234567890abcdef"), nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			authService, err := NewAuthService(&authUseCaseStub{}, logging.NopLogger())
			require.NoError(t, err)

			// Act
			_, err = authService.Register(context.Background(), tt.req)

			// Assert
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

// TestAuthService_Register_FailWithInvalidMasterKeySalt проверяет маппинг некорректной соли в InvalidArgument.
func TestAuthService_Register_FailWithInvalidMasterKeySalt(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{registerErr: usecase.ErrInvalidMasterKeySalt}
	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := registerRequest("user", "password")

	// Act
	_, err = authService.Register(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestAuthService_Register_FailWithAlreadyExists проверяет маппинг занятого логина в код ошибки AlreadyExists.
func TestAuthService_Register_FailWithAlreadyExists(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{registerErr: usecase.ErrLoginAlreadyTaken}
	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := registerRequest("user", "password")

	// Act
	_, err = authService.Register(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.AlreadyExists, status.Code(err))
}

// TestAuthService_Register_FailWithInternalError проверяет маппинг неизвестной ошибки в код ошибки Internal.
func TestAuthService_Register_FailWithInternalError(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{registerErr: errors.New("some internal error")}
	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := registerRequest("user", "password")

	// Act
	_, err = authService.Register(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// TestAuthService_Login_OK проверяет успешную аутентификацию через gRPC-обработчик.
func TestAuthService_Login_OK(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{loginOutput: usecase.LoginOutput{
		AuthTokens: usecase.AuthTokens{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
		},
		MasterKeySalt:     []byte("salt"),
		MasterKeyVerifier: []byte("verifier"),
	}}

	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := pb.LoginRequest_builder{
		Login:    new("user"),
		Password: new("password"),
	}.Build()

	// Act
	resp, err := authService.Login(context.Background(), req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "access-token", resp.GetAccessToken())
	assert.Equal(t, "refresh-token", resp.GetRefreshToken())
	assert.Equal(t, []byte("salt"), resp.GetMasterKeySalt())
	assert.Equal(t, []byte("verifier"), resp.GetMasterKeyVerifier())
}

// TestAuthService_Login_FailWithInvalidArgument проверяет валидацию обязательных полей запроса.
func TestAuthService_Login_FailWithInvalidArgument(t *testing.T) {
	// Arrange
	tests := []struct {
		name string
		req  *pb.LoginRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty login", req: pb.LoginRequest_builder{Login: new(""), Password: new("password")}.Build()},
		{name: "blank login", req: pb.LoginRequest_builder{Login: new("   "), Password: new("password")}.Build()},
		{name: "empty password", req: pb.LoginRequest_builder{Login: new("user"), Password: new("")}.Build()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			authService, err := NewAuthService(&authUseCaseStub{}, logging.NopLogger())
			require.NoError(t, err)

			// Act
			_, err = authService.Login(context.Background(), tt.req)

			// Assert
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

// TestAuthService_Login_FailWithUnauthenticated проверяет маппинг ошибки аутентификации.
func TestAuthService_Login_FailWithUnauthenticated(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{loginErr: usecase.ErrAuthenticationFailed}
	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := pb.LoginRequest_builder{Login: new("user"), Password: new("password")}.Build()

	// Act
	_, err = authService.Login(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAuthService_Login_FailWithInternalError проверяет маппинг неизвестной ошибки в код ошибки Internal.
func TestAuthService_Login_FailWithInternalError(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{loginErr: errors.New("some internal error")}
	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := pb.LoginRequest_builder{Login: new("user"), Password: new("password")}.Build()

	// Act
	_, err = authService.Login(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// TestAuthService_Refresh_OK проверяет успешное обновление пары токенов через gRPC-обработчик.
func TestAuthService_Refresh_OK(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{refreshOutput: usecase.RefreshOutput{
		AuthTokens: usecase.AuthTokens{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
		},
	}}

	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := pb.RefreshRequest_builder{RefreshToken: new("active-refresh-token")}.Build()

	// Act
	resp, err := authService.Refresh(context.Background(), req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "new-access-token", resp.GetAccessToken())
	assert.Equal(t, "new-refresh-token", resp.GetRefreshToken())
}

// TestAuthService_Refresh_FailWithInvalidArgument проверяет валидацию обязательного refresh-токена.
func TestAuthService_Refresh_FailWithInvalidArgument(t *testing.T) {
	// Arrange
	tests := []struct {
		name string
		req  *pb.RefreshRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty refresh token", req: pb.RefreshRequest_builder{RefreshToken: new("")}.Build()},
		{name: "blank refresh token", req: pb.RefreshRequest_builder{RefreshToken: new("   ")}.Build()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			authService, err := NewAuthService(&authUseCaseStub{}, logging.NopLogger())
			require.NoError(t, err)

			// Act
			_, err = authService.Refresh(context.Background(), tt.req)

			// Assert
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

// TestAuthService_Refresh_FailWithUnauthenticated проверяет маппинг ошибки аутентификации.
func TestAuthService_Refresh_FailWithUnauthenticated(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{refreshErr: usecase.ErrAuthenticationFailed}
	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := pb.RefreshRequest_builder{RefreshToken: new("refresh-token")}.Build()

	// Act
	_, err = authService.Refresh(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAuthService_Refresh_FailWithInternalError проверяет маппинг неизвестной ошибки в код ошибки Internal.
func TestAuthService_Refresh_FailWithInternalError(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{refreshErr: errors.New("some internal error")}
	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := pb.RefreshRequest_builder{RefreshToken: new("refresh-token")}.Build()

	// Act
	_, err = authService.Refresh(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// TestAuthService_Logout_OK проверяет успешное завершение сессии через gRPC-обработчик.
func TestAuthService_Logout_OK(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{changeMasterKeyOutput: usecase.ChangeMasterKeyOutput{
		AuthTokens: usecase.AuthTokens{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
		},
	}}
	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := pb.LogoutRequest_builder{RefreshToken: new("active-refresh-token")}.Build()

	// Act
	resp, err := authService.Logout(context.Background(), req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestAuthService_Logout_FailWithInvalidArgument проверяет валидацию обязательного refresh-токена.
func TestAuthService_Logout_FailWithInvalidArgument(t *testing.T) {
	// Arrange
	tests := []struct {
		name string
		req  *pb.LogoutRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty refresh token", req: pb.LogoutRequest_builder{RefreshToken: new("")}.Build()},
		{name: "blank refresh token", req: pb.LogoutRequest_builder{RefreshToken: new("   ")}.Build()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			authService, err := NewAuthService(&authUseCaseStub{}, logging.NopLogger())
			require.NoError(t, err)

			// Act
			_, err = authService.Logout(context.Background(), tt.req)

			// Assert
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

// TestAuthService_Logout_FailWithUnauthenticated проверяет маппинг ошибки аутентификации.
func TestAuthService_Logout_FailWithUnauthenticated(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{logoutErr: usecase.ErrAuthenticationFailed}
	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := pb.LogoutRequest_builder{RefreshToken: new("refresh-token")}.Build()

	// Act
	_, err = authService.Logout(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAuthService_Logout_FailWithInternalError проверяет маппинг неизвестной ошибки в код ошибки Internal.
func TestAuthService_Logout_FailWithInternalError(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{logoutErr: errors.New("some internal error")}
	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := pb.LogoutRequest_builder{RefreshToken: new("refresh-token")}.Build()

	// Act
	_, err = authService.Logout(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// TestAuthService_ChangeMasterKey_OK проверяет успешную смену мастер-ключа через gRPC-обработчик.
func TestAuthService_ChangeMasterKey_OK(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{changeMasterKeyOutput: usecase.ChangeMasterKeyOutput{
		AuthTokens: usecase.AuthTokens{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
		},
	}}
	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000012")
	recordID := "018f6b7c-0000-7000-8000-200000000012"
	expectedVersion := int64(3)
	req := pb.ChangeMasterKeyRequest_builder{
		MasterKeySalt:     []byte("abcdef1234567890"),
		MasterKeyVerifier: []byte("new-verifier"),
		Records: []*pb.ReencryptedRecordDEK{
			pb.ReencryptedRecordDEK_builder{
				RecordId:        &recordID,
				ExpectedVersion: &expectedVersion,
				EncryptedDek:    []byte("new-encrypted-dek"),
			}.Build(),
		},
	}.Build()

	// Act
	resp, err := authService.ChangeMasterKey(authcontext.WithUserSession(context.Background(), userID, 7), req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "new-access-token", resp.GetAccessToken())
	assert.Equal(t, "new-refresh-token", resp.GetRefreshToken())
	assert.Equal(t, userID, uc.changeMasterKeyInput.UserID)
	assert.Equal(t, int64(7), uc.changeMasterKeyInput.SecurityVersion)
	assert.Equal(t, []byte("abcdef1234567890"), uc.changeMasterKeyInput.MasterKeySalt)
	require.Len(t, uc.changeMasterKeyInput.Records, 1)
	assert.Equal(t, expectedVersion, uc.changeMasterKeyInput.Records[0].ExpectedVersion)
}

// TestAuthService_ChangeMasterKey_FailWithInvalidArgument проверяет валидацию запроса смены мастер-ключа.
func TestAuthService_ChangeMasterKey_FailWithInvalidArgument(t *testing.T) {
	tests := []struct {
		name string
		req  *pb.ChangeMasterKeyRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty salt", req: changeMasterKeyRequest(nil, []byte("verifier"))},
		{name: "empty verifier", req: changeMasterKeyRequest([]byte("abcdef1234567890"), nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			authService, err := NewAuthService(&authUseCaseStub{}, logging.NopLogger())
			require.NoError(t, err)
			userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000013")

			// Act
			_, err = authService.ChangeMasterKey(authcontext.WithUserSession(context.Background(), userID, 1), tt.req)

			// Assert
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

// TestAuthService_ChangeMasterKey_FailWithConflict проверяет маппинг конфликта смены мастер-ключа в Aborted.
func TestAuthService_ChangeMasterKey_FailWithConflict(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{changeMasterKeyErr: usecase.ErrMasterKeyChangeConflict}
	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000014")

	// Act
	_, err = authService.ChangeMasterKey(
		authcontext.WithUserSession(context.Background(), userID, 1),
		changeMasterKeyRequest([]byte("abcdef1234567890"), []byte("verifier")),
	)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Aborted, status.Code(err))
}

func changeMasterKeyRequest(salt []byte, verifier []byte) *pb.ChangeMasterKeyRequest {
	return pb.ChangeMasterKeyRequest_builder{
		MasterKeySalt:     salt,
		MasterKeyVerifier: verifier,
	}.Build()
}
