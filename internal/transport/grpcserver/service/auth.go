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
		if !errors.Is(err, model.ErrLoginAlreadyTaken) {
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

func validateRegisterRequest(req *pb.RegisterRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if strings.TrimSpace(req.GetLogin()) == "" {
		return status.Error(codes.InvalidArgument, "login is required")
	}
	if req.GetPassword() == "" {
		return status.Error(codes.InvalidArgument, "password is required")
	}
	return nil
}

func authErrorToStatus(err error) error {
	if errors.Is(err, model.ErrLoginAlreadyTaken) {
		return status.Error(codes.AlreadyExists, "login already taken")
	}
	return status.Error(codes.Internal, "internal error")
}
