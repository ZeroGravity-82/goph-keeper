package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/logging"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/usecase"
)

// authUseCase описывает сценарии аутентификации, которые нужны gRPC-сервису.
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
	in, err := registerInputFromRequest(req)
	if err != nil {
		return nil, err
	}

	out, err := s.uc.Register(ctx, in)
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

// registerInputFromRequest валидирует gRPC-запрос и преобразует его во входной DTO сценария регистрации.
func registerInputFromRequest(req *pb.RegisterRequest) (usecase.RegisterInput, error) {
	if req == nil {
		return usecase.RegisterInput{}, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateCredentials(req.GetLogin(), req.GetPassword()); err != nil {
		return usecase.RegisterInput{}, err
	}
	if len(req.GetMasterKeySalt()) == 0 {
		return usecase.RegisterInput{}, status.Error(codes.InvalidArgument, "master key salt is required")
	}
	if len(req.GetMasterKeyVerifier()) == 0 {
		return usecase.RegisterInput{}, status.Error(codes.InvalidArgument, "master key verifier is required")
	}
	return usecase.RegisterInput{
		Login:             req.GetLogin(),
		Password:          req.GetPassword(),
		MasterKeySalt:     req.GetMasterKeySalt(),
		MasterKeyVerifier: req.GetMasterKeyVerifier(),
	}, nil
}

// validateCredentials проверяет обязательные учетные данные пользователя.
func validateCredentials(login, password string) error {
	if strings.TrimSpace(login) == "" {
		return status.Error(codes.InvalidArgument, "login is required")
	}
	if password == "" {
		return status.Error(codes.InvalidArgument, "password is required")
	}
	return nil
}

// isExpectedAuthError определяет ожидаемые ошибки аутентификации, которые не нужно логировать как внутренние ошибки.
func isExpectedAuthError(err error) bool {
	return errors.Is(err, usecase.ErrLoginAlreadyTaken) ||
		errors.Is(err, usecase.ErrAuthenticationFailed) ||
		errors.Is(err, usecase.ErrUserNotFound) ||
		errors.Is(err, usecase.ErrInvalidMasterKeySalt)
}

// authErrorToStatus преобразует ошибку сценария аутентификации в gRPC-статус.
func authErrorToStatus(err error) error {
	if errors.Is(err, usecase.ErrLoginAlreadyTaken) {
		return status.Error(codes.AlreadyExists, "login already taken")
	}
	if errors.Is(err, usecase.ErrAuthenticationFailed) {
		return status.Error(codes.Unauthenticated, "authentication failed")
	}
	if errors.Is(err, usecase.ErrUserNotFound) {
		return status.Error(codes.NotFound, "user not found")
	}
	if errors.Is(err, usecase.ErrInvalidMasterKeySalt) {
		return status.Error(codes.InvalidArgument, "master key salt has invalid length")
	}
	return status.Error(codes.Internal, "internal error")
}

// Login аутентифицирует пользователя и возвращает пару токенов и соль мастер-ключа.
func (s *AuthService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	in, err := loginInputFromRequest(req)
	if err != nil {
		return nil, err
	}

	out, err := s.uc.Login(ctx, in)
	if err != nil {
		if !isExpectedAuthError(err) {
			s.logger.Error("failed to login user", slog.Any("err", err))
		}
		return nil, authErrorToStatus(err)
	}

	return pb.LoginResponse_builder{
		AccessToken:       &out.AuthTokens.AccessToken,
		RefreshToken:      &out.AuthTokens.RefreshToken,
		MasterKeySalt:     out.MasterKeySalt,
		MasterKeyVerifier: out.MasterKeyVerifier,
	}.Build(), nil
}

// loginInputFromRequest валидирует gRPC-запрос и преобразует его во входной DTO сценария входа в аккаунт.
func loginInputFromRequest(req *pb.LoginRequest) (usecase.LoginInput, error) {
	if req == nil {
		return usecase.LoginInput{}, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateCredentials(req.GetLogin(), req.GetPassword()); err != nil {
		return usecase.LoginInput{}, err
	}
	return usecase.LoginInput{Login: req.GetLogin(), Password: req.GetPassword()}, nil
}

// Refresh обновляет пару токенов по refresh-токену.
func (s *AuthService) Refresh(ctx context.Context, req *pb.RefreshRequest) (*pb.RefreshResponse, error) {
	in, err := refreshInputFromRequest(req)
	if err != nil {
		return nil, err
	}

	out, err := s.uc.Refresh(ctx, in)
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

// refreshInputFromRequest валидирует gRPC-запрос и преобразует его во входной DTO сценария обновления токенов.
func refreshInputFromRequest(req *pb.RefreshRequest) (usecase.RefreshInput, error) {
	if req == nil {
		return usecase.RefreshInput{}, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateRefreshToken(req.GetRefreshToken()); err != nil {
		return usecase.RefreshInput{}, err
	}
	return usecase.RefreshInput{RefreshToken: req.GetRefreshToken()}, nil
}

// validateRefreshToken проверяет обязательный refresh-токен.
func validateRefreshToken(refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return status.Error(codes.InvalidArgument, "refresh token is required")
	}
	return nil
}

// Logout завершает пользовательскую сессию по refresh-токену.
func (s *AuthService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	in, err := logoutInputFromRequest(req)
	if err != nil {
		return nil, err
	}

	if err := s.uc.Logout(ctx, in); err != nil {
		if !isExpectedAuthError(err) {
			s.logger.Error("failed to logout user", slog.Any("err", err))
		}
		return nil, authErrorToStatus(err)
	}

	return pb.LogoutResponse_builder{}.Build(), nil
}

// logoutInputFromRequest валидирует gRPC-запрос и преобразует его во входной DTO сценария завершения сессии.
func logoutInputFromRequest(req *pb.LogoutRequest) (usecase.LogoutInput, error) {
	if req == nil {
		return usecase.LogoutInput{}, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateRefreshToken(req.GetRefreshToken()); err != nil {
		return usecase.LogoutInput{}, err
	}
	return usecase.LogoutInput{RefreshToken: req.GetRefreshToken()}, nil
}
