package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"zerogravity-82/goph-keeper/internal/auth"
	"zerogravity-82/goph-keeper/internal/crypto"
	"zerogravity-82/goph-keeper/internal/domain/model"
)

type AuthTokens struct {
	AccessToken  string
	RefreshToken string
}

type RegisterInput struct {
	Login    string
	Password string
}

type RegisterOutput struct {
	AuthTokens    AuthTokens
	MasterKeySalt []byte
}

type LoginInput struct {
	Login    string
	Password string
}

type LoginOutput struct {
	AuthTokens    AuthTokens
	MasterKeySalt []byte
}

type RefreshInput struct {
	RefreshToken string
}

type RefreshOutput struct {
	AuthTokens AuthTokens
}

type LogoutInput struct {
	RefreshToken string
}

type userRepository interface {
	Create(ctx context.Context, u model.User) error
	GetByLogin(ctx context.Context, login string) (model.User, error)
}

type refreshTokenRepository interface {
	Create(ctx context.Context, t model.RefreshToken) error
	FindActiveByHash(ctx context.Context, tokenHash string, now time.Time) (model.RefreshToken, error)
	Revoke(ctx context.Context, tokenID uuid.UUID, revokedAt time.Time) error
}

type transactor interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type sessionTokenIssuer interface {
	IssueAccessToken(userID uuid.UUID) (string, error)
	GenerateRefreshToken() (string, error)
}

type AuthUseCase struct {
	userRepo         userRepository
	refreshTokenRepo refreshTokenRepository
	transactor       transactor
	tokenIssuer      sessionTokenIssuer
	refreshTokenTTL  time.Duration
}

// NewAuthUseCase создает AuthUseCase.
func NewAuthUseCase(
	userRepo userRepository,
	refreshTokenRepo refreshTokenRepository,
	transactor transactor,
	tokenIssuer sessionTokenIssuer,
	refreshTokenTTL time.Duration,
) (*AuthUseCase, error) {
	if userRepo == nil {
		return nil, errors.New("user repository is not provided")
	}
	if refreshTokenRepo == nil {
		return nil, errors.New("refresh token repository is not provided")
	}
	if transactor == nil {
		return nil, errors.New("transactor is not provided")
	}
	if tokenIssuer == nil {
		return nil, errors.New("session token issuer is not provided")
	}
	if refreshTokenTTL <= 0 {
		return nil, errors.New("refresh token TTL must be positive")
	}

	return &AuthUseCase{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		transactor:       transactor,
		tokenIssuer:      tokenIssuer,
		refreshTokenTTL:  refreshTokenTTL,
	}, nil
}

// Register регистрирует пользователя и возвращает пару токенов (access/refresh) и соль для мастер-ключа.
func (uc *AuthUseCase) Register(ctx context.Context, in RegisterInput) (RegisterOutput, error) {
	// Убеждаемся, что логин уникален
	_, err := uc.userRepo.GetByLogin(ctx, in.Login)
	if err == nil {
		return RegisterOutput{}, model.ErrLoginAlreadyTaken
	}
	if !errors.Is(err, model.ErrUserNotFound) {
		err = fmt.Errorf("failed to verify login uniqueness: %w", err)
		return RegisterOutput{}, err
	}

	h, err := auth.HashPassword(in.Password)
	if err != nil {
		return RegisterOutput{}, fmt.Errorf("failed to hash password for new user: %w", err)
	}

	salt, err := crypto.GenerateMasterKeySalt()
	if err != nil {
		return RegisterOutput{}, fmt.Errorf("failed to generate master key salt for new user: %w", err)
	}

	uuidV7, err := uuid.NewV7()
	if err != nil {
		return RegisterOutput{}, fmt.Errorf("failed to generate ID for new user: %w", err)
	}
	now := time.Now().UTC()
	u := model.User{
		ID:            uuidV7,
		Login:         in.Login,
		PasswordHash:  h,
		MasterKeySalt: salt,
		RegisteredAt:  now,
		UpdatedAt:     now,
	}

	tokens, rt, err := uc.issueTokens(u.ID, now)
	if err != nil {
		return RegisterOutput{}, fmt.Errorf("failed to issue tokens: %w", err)
	}
	if err = uc.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		if err = uc.userRepo.Create(ctx, u); err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}
		if err = uc.refreshTokenRepo.Create(ctx, rt); err != nil {
			return fmt.Errorf("failed to persist refresh token: %w", err)
		}
		return nil
	}); err != nil {
		return RegisterOutput{}, fmt.Errorf("failed to register user: %w", err)
	}

	return RegisterOutput{AuthTokens: tokens, MasterKeySalt: salt}, nil
}

// Login аутентифицирует пользователя и возвращает пару токенов (access/refresh) и соль для мастер-ключа.
func (uc *AuthUseCase) Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	u, err := uc.userRepo.GetByLogin(ctx, in.Login)
	if err != nil {
		if errors.Is(err, model.ErrUserNotFound) {
			return LoginOutput{}, model.ErrAuthenticationFailed
		}
		return LoginOutput{}, fmt.Errorf("failed to get user by login: %w", err)
	}

	if err = auth.CheckPasswordHash(in.Password, u.PasswordHash); err != nil {
		return LoginOutput{}, model.ErrAuthenticationFailed
	}

	now := time.Now().UTC()
	tokens, rt, err := uc.issueTokens(u.ID, now)
	if err != nil {
		return LoginOutput{}, fmt.Errorf("failed to issue tokens: %w", err)
	}

	if err = uc.refreshTokenRepo.Create(ctx, rt); err != nil {
		return LoginOutput{}, fmt.Errorf("failed to save login session: %w", err)
	}

	return LoginOutput{AuthTokens: tokens, MasterKeySalt: u.MasterKeySalt}, nil
}

// Refresh обновляет пару токенов по активному refresh-токену.
func (uc *AuthUseCase) Refresh(ctx context.Context, in RefreshInput) (RefreshOutput, error) {
	now := time.Now().UTC()
	activeRefreshHash := auth.HashRefreshToken(in.RefreshToken)

	var tokens AuthTokens
	if err := uc.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		activeRefreshToken, err := uc.refreshTokenRepo.FindActiveByHash(ctx, activeRefreshHash, now)
		if err != nil {
			if errors.Is(err, model.ErrRefreshTokenNotFound) {
				return model.ErrAuthenticationFailed
			}
			return fmt.Errorf("failed to find active refresh token: %w", err)
		}

		newTokens, newRefreshToken, err := uc.issueTokens(activeRefreshToken.UserID, now)
		if err != nil {
			return fmt.Errorf("failed to issue tokens: %w", err)
		}

		if err = uc.refreshTokenRepo.Revoke(ctx, activeRefreshToken.ID, now); err != nil {
			return fmt.Errorf("failed to revoke refresh token: %w", err)
		}
		if err = uc.refreshTokenRepo.Create(ctx, newRefreshToken); err != nil {
			return fmt.Errorf("failed to persist refresh token: %w", err)
		}

		tokens = newTokens
		return nil
	}); err != nil {
		if errors.Is(err, model.ErrAuthenticationFailed) {
			return RefreshOutput{}, model.ErrAuthenticationFailed
		}
		return RefreshOutput{}, fmt.Errorf("failed to refresh tokens: %w", err)
	}

	return RefreshOutput{AuthTokens: tokens}, nil
}

// Logout завершает пользовательскую сессию, отзывая активный refresh-токен.
func (uc *AuthUseCase) Logout(ctx context.Context, in LogoutInput) error {
	now := time.Now().UTC()
	activeRefreshHash := auth.HashRefreshToken(in.RefreshToken)

	if err := uc.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		activeRefreshToken, err := uc.refreshTokenRepo.FindActiveByHash(ctx, activeRefreshHash, now)
		if err != nil {
			if errors.Is(err, model.ErrRefreshTokenNotFound) {
				return model.ErrAuthenticationFailed
			}
			return fmt.Errorf("failed to find active refresh token: %w", err)
		}

		if err = uc.refreshTokenRepo.Revoke(ctx, activeRefreshToken.ID, now); err != nil {
			return fmt.Errorf("failed to revoke refresh token: %w", err)
		}

		return nil
	}); err != nil {
		if errors.Is(err, model.ErrAuthenticationFailed) {
			return model.ErrAuthenticationFailed
		}
		return fmt.Errorf("failed to logout user: %w", err)
	}

	return nil
}

func (uc *AuthUseCase) issueTokens(userID uuid.UUID, now time.Time) (AuthTokens, model.RefreshToken, error) {
	access, err := uc.tokenIssuer.IssueAccessToken(userID)
	if err != nil {
		return AuthTokens{}, model.RefreshToken{}, err
	}
	refresh, err := uc.tokenIssuer.GenerateRefreshToken()
	if err != nil {
		return AuthTokens{}, model.RefreshToken{}, err
	}

	uuidV7, err := uuid.NewV7()
	if err != nil {
		return AuthTokens{}, model.RefreshToken{}, fmt.Errorf("failed to generate ID for refresh token: %w", err)
	}
	rt := model.RefreshToken{
		ID:        uuidV7,
		UserID:    userID,
		TokenHash: auth.HashRefreshToken(refresh),
		ExpiresAt: now.Add(uc.refreshTokenTTL),
		IssuedAt:  now,
		RevokedAt: nil,
	}

	return AuthTokens{AccessToken: access, RefreshToken: refresh}, rt, nil
}
