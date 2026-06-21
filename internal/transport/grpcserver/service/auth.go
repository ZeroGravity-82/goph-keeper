package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/logging"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/usecase"
)

type authUseCase interface {
	Register(ctx context.Context, in usecase.RegisterInput) (usecase.RegisterOutput, error)
	Login(ctx context.Context, in usecase.LoginInput) (usecase.LoginOutput, error)
	Refresh(ctx context.Context, in usecase.RefreshInput) (usecase.RefreshOutput, error)
	Logout(ctx context.Context, in usecase.LogoutInput) error
}

// AuthService реализует gRPC-сервис аутентификации.
type AuthService struct {
	pb.UnimplementedAuthServer

	uc     authUseCase
	logger *slog.Logger
}

// NewAuthService создает AuthService.
func NewAuthService(uc authUseCase, logger *slog.Logger) (*AuthService, error) {
	if uc == nil {
		return nil, errors.New("auth use case is not provided")
	}
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &AuthService{uc: uc, logger: logger}, nil
}

// Register регистрирует пользователя и возвращает пару токенов и соль мастер-ключа.
func (s *AuthService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if err := validateRegisterRequest(req); err != nil {
		return nil, err
	}

	out, err := s.uc.Register(ctx, usecase.RegisterInput{
		Login:    req.GetLogin(),
		Password: req.GetPassword(),
	})
	if err != nil {
		if !isExpectedAuthError(err) {
			s.logger.Error("failed to register user", slog.Any("err", err))
		}
		return nil, authErrorToStatus(err)
	}

	return pb.RegisterResponse_builder{
		AccessToken:   &out.AuthTokens.AccessToken,
		RefreshToken:  &out.AuthTokens.RefreshToken,
		MasterKeySalt: out.MasterKeySalt,
	}.Build(), nil
}

// Login аутентифицирует пользователя и возвращает пару токенов и соль мастер-ключа.
func (s *AuthService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if err := validateLoginRequest(req); err != nil {
		return nil, err
	}

	out, err := s.uc.Login(ctx, usecase.LoginInput{
		Login:    req.GetLogin(),
		Password: req.GetPassword(),
	})
	if err != nil {
		if !isExpectedAuthError(err) {
			s.logger.Error("failed to login user", slog.Any("err", err))
		}
		return nil, authErrorToStatus(err)
	}

	return pb.LoginResponse_builder{
		AccessToken:   &out.AuthTokens.AccessToken,
		RefreshToken:  &out.AuthTokens.RefreshToken,
		MasterKeySalt: out.MasterKeySalt,
	}.Build(), nil
}

// Refresh обновляет пару токенов по refresh-токену.
func (s *AuthService) Refresh(ctx context.Context, req *pb.RefreshRequest) (*pb.RefreshResponse, error) {
	if err := validateRefreshRequest(req); err != nil {
		return nil, err
	}

	out, err := s.uc.Refresh(ctx, usecase.RefreshInput{RefreshToken: req.GetRefreshToken()})
	if err != nil {
		if !isExpectedAuthError(err) {
			s.logger.Error("failed to refresh tokens", slog.Any("err", err))
		}
		return nil, authErrorToStatus(err)
	}

	return pb.RefreshResponse_builder{
		AccessToken:  &out.AuthTokens.AccessToken,
		RefreshToken: &out.AuthTokens.RefreshToken,
	}.Build(), nil
}

// Logout завершает пользовательскую сессию по refresh-токену.
func (s *AuthService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	if err := validateLogoutRequest(req); err != nil {
		return nil, err
	}

	if err := s.uc.Logout(ctx, usecase.LogoutInput{RefreshToken: req.GetRefreshToken()}); err != nil {
		if !isExpectedAuthError(err) {
			s.logger.Error("failed to logout user", slog.Any("err", err))
		}
		return nil, authErrorToStatus(err)
	}

	return pb.LogoutResponse_builder{}.Build(), nil
}

func validateRegisterRequest(req *pb.RegisterRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	return validateCredentials(req.GetLogin(), req.GetPassword())
}

func validateLoginRequest(req *pb.LoginRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	return validateCredentials(req.GetLogin(), req.GetPassword())
}

func validateRefreshRequest(req *pb.RefreshRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	return validateRefreshToken(req.GetRefreshToken())
}

func validateLogoutRequest(req *pb.LogoutRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	return validateRefreshToken(req.GetRefreshToken())
}

func validateRefreshToken(refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return status.Error(codes.InvalidArgument, "refresh token is required")
	}
	return nil
}

func validateCredentials(login, password string) error {
	if strings.TrimSpace(login) == "" {
		return status.Error(codes.InvalidArgument, "login is required")
	}
	if password == "" {
		return status.Error(codes.InvalidArgument, "password is required")
	}
	return nil
}

func authErrorToStatus(err error) error {
	if errors.Is(err, model.ErrLoginAlreadyTaken) {
		return status.Error(codes.AlreadyExists, "login already taken")
	}
	if errors.Is(err, model.ErrAuthenticationFailed) {
		return status.Error(codes.Unauthenticated, "authentication failed")
	}
	return status.Error(codes.Internal, "internal error")
}

func isExpectedAuthError(err error) bool {
	return errors.Is(err, model.ErrLoginAlreadyTaken) || errors.Is(err, model.ErrAuthenticationFailed)
}
