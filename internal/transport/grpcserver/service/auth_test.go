package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/logging"
	"zerogravity-82/goph-keeper/internal/pb"
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

// TestAuthService_Register_OK проверяет успешную регистрацию через gRPC-обработчик.
func TestAuthService_Register_OK(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{registerOutput: usecase.RegisterOutput{
		AuthTokens: usecase.AuthTokens{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
		},
		MasterKeySalt: []byte("salt"),
	}}

	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := pb.RegisterRequest_builder{
		Login:    new("user"),
		Password: new("password"),
	}.Build()

	// Act
	resp, err := authService.Register(context.Background(), req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "access-token", resp.GetAccessToken())
	assert.Equal(t, "refresh-token", resp.GetRefreshToken())
	assert.Equal(t, []byte("salt"), resp.GetMasterKeySalt())
}

// TestAuthService_Register_FailWithInvalidArgument проверяет валидацию обязательных полей запроса.
func TestAuthService_Register_FailWithInvalidArgument(t *testing.T) {
	// Arrange
	tests := []struct {
		name string
		req  *pb.RegisterRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty login", req: pb.RegisterRequest_builder{Login: new(""), Password: new("password")}.Build()},
		{name: "blank login", req: pb.RegisterRequest_builder{Login: new("   "), Password: new("password")}.Build()},
		{name: "empty password", req: pb.RegisterRequest_builder{Login: new("user"), Password: new("")}.Build()},
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

// TestAuthService_Register_FailWithAlreadyExists проверяет маппинг занятого логина в код ошибки AlreadyExists.
func TestAuthService_Register_FailWithAlreadyExists(t *testing.T) {
	// Arrange
	uc := &authUseCaseStub{registerErr: model.ErrLoginAlreadyTaken}
	authService, err := NewAuthService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := pb.RegisterRequest_builder{Login: new("user"), Password: new("password")}.Build()

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
	req := pb.RegisterRequest_builder{Login: new("user"), Password: new("password")}.Build()

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
		MasterKeySalt: []byte("salt"),
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
	uc := &authUseCaseStub{loginErr: model.ErrAuthenticationFailed}
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
	uc := &authUseCaseStub{refreshErr: model.ErrAuthenticationFailed}
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
