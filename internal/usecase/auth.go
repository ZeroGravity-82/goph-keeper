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

type tokenIssuer interface {
	IssueAccessToken(userID uuid.UUID) (string, error)
	GenerateRefreshToken() (string, error)
}

type AuthUseCase struct {
	userRepo         userRepository
	refreshTokenRepo refreshTokenRepository
	transactor       transactor
	tokens           tokenIssuer
	refreshTokenTTL  time.Duration
}

// NewAuthUseCase создает AuthUseCase.
func NewAuthUseCase(
	userRepo userRepository,
	refreshTokenRepo refreshTokenRepository,
	transactor transactor,
	tokens tokenIssuer,
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
	if tokens == nil {
		return nil, errors.New("token issuer is not provided")
	}
	if refreshTokenTTL <= 0 {
		return nil, errors.New("refresh token TTL must be positive")
	}

	return &AuthUseCase{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		transactor:       transactor,
		tokens:           tokens,
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

func (uc *AuthUseCase) issueTokens(userID uuid.UUID, now time.Time) (AuthTokens, model.RefreshToken, error) {
	access, err := uc.tokens.IssueAccessToken(userID)
	if err != nil {
		return AuthTokens{}, model.RefreshToken{}, err
	}
	refresh, err := uc.tokens.GenerateRefreshToken()
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
