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
}

func (s *authUseCaseStub) Register(_ context.Context, in usecase.RegisterInput) (usecase.RegisterOutput, error) {
	s.registerInput = in
	return s.registerOutput, s.registerErr
}

// TestAuthService_Register_OK проверяет успешную регистрацию через gRPC-хэндлер.
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
	assert.Equal(t, usecase.RegisterInput{Login: "user", Password: "password"}, uc.registerInput)
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
