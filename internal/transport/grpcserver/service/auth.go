package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/logging"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/transport/grpcserver/authcontext"
	"zerogravity-82/goph-keeper/internal/usecase"
)

const (
	userLoginMaxChars         = 128
	userPasswordMaxChars      = 256
	masterKeySaltBytes        = 16
	masterKeyVerifierMaxBytes = 128
)

// authUseCase описывает сценарии сервиса аутентификации: регистрация, вход в аккаунт, обновление пары токенов, выход
// из аккаунта и смена мастер-ключа.
type authUseCase interface {
	Register(ctx context.Context, in usecase.RegisterInput) (usecase.RegisterOutput, error)
	Login(ctx context.Context, in usecase.LoginInput) (usecase.LoginOutput, error)
	Refresh(ctx context.Context, in usecase.RefreshInput) (usecase.RefreshOutput, error)
	Logout(ctx context.Context, in usecase.LogoutInput) error
	ChangeMasterKey(ctx context.Context, in usecase.ChangeMasterKeyInput) (usecase.ChangeMasterKeyOutput, error)
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
	if err := validateMasterKeySaltSize(req.GetMasterKeySalt()); err != nil {
		return usecase.RegisterInput{}, err
	}
	if len(req.GetMasterKeyVerifier()) == 0 {
		return usecase.RegisterInput{}, status.Error(codes.InvalidArgument, "master key verifier is required")
	}
	if err := validateMasterKeyVerifierSize(req.GetMasterKeyVerifier()); err != nil {
		return usecase.RegisterInput{}, err
	}
	return usecase.RegisterInput{
		Login:             req.GetLogin(),
		Password:          req.GetPassword(),
		MasterKeySalt:     req.GetMasterKeySalt(),
		MasterKeyVerifier: req.GetMasterKeyVerifier(),
	}, nil
}

// validateCredentials проверяет обязательность и длину учетных данных пользователя.
func validateCredentials(login, password string) error {
	if strings.TrimSpace(login) == "" {
		return status.Error(codes.InvalidArgument, "login is required")
	}
	if password == "" {
		return status.Error(codes.InvalidArgument, "password is required")
	}
	if utf8.RuneCountInString(login) > userLoginMaxChars {
		return status.Error(codes.InvalidArgument, "login exceeds size limit")
	}
	if utf8.RuneCountInString(password) > userPasswordMaxChars {
		return status.Error(codes.InvalidArgument, "password exceeds size limit")
	}
	return nil
}

// validateMasterKeySaltSize проверяет размер соли мастер-ключа из gRPC-запроса.
func validateMasterKeySaltSize(salt []byte) error {
	if len(salt) != masterKeySaltBytes {
		return status.Error(codes.InvalidArgument, "master key salt has invalid length")
	}
	return nil
}

// validateMasterKeyVerifierSize проверяет размер зашифрованного верификатора мастер-ключа из gRPC-запроса.
func validateMasterKeyVerifierSize(verifier []byte) error {
	if len(verifier) > masterKeyVerifierMaxBytes {
		return status.Error(codes.InvalidArgument, "master key verifier exceeds size limit")
	}
	return nil
}

// isExpectedAuthError определяет ожидаемые ошибки аутентификации, которые не нужно логировать как внутренние ошибки.
func isExpectedAuthError(err error) bool {
	return errors.Is(err, usecase.ErrLoginAlreadyTaken) ||
		errors.Is(err, usecase.ErrAuthenticationFailed) ||
		errors.Is(err, usecase.ErrUserNotFound) ||
		errors.Is(err, usecase.ErrInvalidMasterKeySalt) ||
		errors.Is(err, usecase.ErrMasterKeyChangeConflict)
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
	if errors.Is(err, usecase.ErrMasterKeyChangeConflict) {
		return status.Error(codes.Aborted, "private records changed during master key change")
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

// ChangeMasterKey обновляет данные мастер-ключа и зашифрованные DEK приватных записей пользователя.
func (s *AuthService) ChangeMasterKey(
	ctx context.Context,
	req *pb.ChangeMasterKeyRequest,
) (*pb.ChangeMasterKeyResponse, error) {
	in, err := changeMasterKeyInputFromRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	out, err := s.uc.ChangeMasterKey(ctx, in)
	if err != nil {
		if !isExpectedAuthError(err) {
			s.logger.Error("failed to change master key", slog.Any("err", err))
		}
		return nil, authErrorToStatus(err)
	}

	return pb.ChangeMasterKeyResponse_builder{
		AccessToken:  &out.AuthTokens.AccessToken,
		RefreshToken: &out.AuthTokens.RefreshToken,
	}.Build(), nil
}

// changeMasterKeyInputFromRequest валидирует gRPC-запрос и преобразует его во входной DTO сценария смены мастер-ключа.
func changeMasterKeyInputFromRequest(
	ctx context.Context,
	req *pb.ChangeMasterKeyRequest,
) (usecase.ChangeMasterKeyInput, error) {
	if req == nil {
		return usecase.ChangeMasterKeyInput{}, status.Error(codes.InvalidArgument, "request is required")
	}
	userID, ok := authcontext.UserIDFromContext(ctx)
	if !ok {
		return usecase.ChangeMasterKeyInput{}, status.Error(codes.Unauthenticated, "authentication is required")
	}
	securityVersion, ok := authcontext.SecurityVersionFromContext(ctx)
	if !ok {
		return usecase.ChangeMasterKeyInput{}, status.Error(codes.Unauthenticated, "security version is required")
	}
	if len(req.GetMasterKeySalt()) == 0 {
		return usecase.ChangeMasterKeyInput{}, status.Error(codes.InvalidArgument, "master key salt is required")
	}
	if err := validateMasterKeySaltSize(req.GetMasterKeySalt()); err != nil {
		return usecase.ChangeMasterKeyInput{}, err
	}
	if len(req.GetMasterKeyVerifier()) == 0 {
		return usecase.ChangeMasterKeyInput{}, status.Error(codes.InvalidArgument, "master key verifier is required")
	}
	if err := validateMasterKeyVerifierSize(req.GetMasterKeyVerifier()); err != nil {
		return usecase.ChangeMasterKeyInput{}, err
	}

	records := make([]usecase.ReencryptedRecordDEK, 0, len(req.GetRecords()))
	for _, item := range req.GetRecords() {
		if item == nil {
			return usecase.ChangeMasterKeyInput{}, status.Error(codes.InvalidArgument, "record item is required")
		}
		recordID, err := uuid.Parse(item.GetRecordId())
		if err != nil || recordID == uuid.Nil {
			return usecase.ChangeMasterKeyInput{}, status.Error(codes.InvalidArgument, "record ID is invalid")
		}
		if item.GetExpectedVersion() <= 0 {
			return usecase.ChangeMasterKeyInput{}, status.Error(codes.InvalidArgument, "record expected version is invalid")
		}
		if len(item.GetEncryptedDek()) == 0 {
			return usecase.ChangeMasterKeyInput{}, status.Error(codes.InvalidArgument, "record encrypted DEK is required")
		}
		if len(item.GetEncryptedDek()) > recordEncryptedDEKMaxBytes {
			return usecase.ChangeMasterKeyInput{}, status.Error(
				codes.InvalidArgument,
				"record encrypted DEK exceeds size limit",
			)
		}
		records = append(records, usecase.ReencryptedRecordDEK{
			RecordID:        recordID,
			ExpectedVersion: item.GetExpectedVersion(),
			EncryptedDEK:    item.GetEncryptedDek(),
		})
	}
	return usecase.ChangeMasterKeyInput{
		UserID:            userID,
		SecurityVersion:   securityVersion,
		MasterKeySalt:     req.GetMasterKeySalt(),
		MasterKeyVerifier: req.GetMasterKeyVerifier(),
		Records:           records,
	}, nil
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
