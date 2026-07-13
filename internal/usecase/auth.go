package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"zerogravity-82/goph-keeper/internal/auth"
	"zerogravity-82/goph-keeper/internal/domain/model"
)

const initialSecurityVersion int64 = 1

// AuthTokens содержит пару access/refresh-токенов пользовательской сессии.
type AuthTokens struct {
	AccessToken  string
	RefreshToken string
}

// RegisterInput описывает входные данные сценария регистрации пользователя.
type RegisterInput struct {
	Login             string
	Password          string
	MasterKeySalt     []byte
	MasterKeyVerifier []byte
}

// RegisterOutput описывает результат сценария регистрации пользователя.
type RegisterOutput struct {
	AuthTokens    AuthTokens
	MasterKeySalt []byte
}

// LoginInput описывает входные данные сценария входа в аккаунт.
type LoginInput struct {
	Login    string
	Password string
}

// LoginOutput описывает результат сценария входа в аккаунт.
type LoginOutput struct {
	AuthTokens        AuthTokens
	MasterKeySalt     []byte
	MasterKeyVerifier []byte
}

// RefreshInput описывает входные данные сценария обновления пары токенов.
type RefreshInput struct {
	RefreshToken string
}

// RefreshOutput описывает результат сценария обновления пары токенов.
type RefreshOutput struct {
	AuthTokens AuthTokens
}

// LogoutInput описывает входные данные сценария выхода из аккаунта.
type LogoutInput struct {
	RefreshToken string
}

// ReencryptedRecordDEK содержит заново зашифрованный DEK приватной записи и версию, которую видел клиент.
type ReencryptedRecordDEK struct {
	RecordID        uuid.UUID
	ExpectedVersion int64
	EncryptedDEK    []byte
}

// ChangeMasterKeyInput описывает входные данные сценария смены мастер-ключа.
type ChangeMasterKeyInput struct {
	UserID            uuid.UUID
	SecurityVersion   int64
	MasterKeySalt     []byte
	MasterKeyVerifier []byte
	Records           []ReencryptedRecordDEK
}

// ChangeMasterKeyOutput описывает результат сценария смены мастер-ключа.
type ChangeMasterKeyOutput struct {
	AuthTokens AuthTokens
}

// userRepository описывает операции с пользователем, которые нужны сценариям аутентификации.
type userRepository interface {
	Create(ctx context.Context, u model.User) error
	GetByLogin(ctx context.Context, login string) (model.User, error)
	GetSecurityVersion(ctx context.Context, userID uuid.UUID) (int64, error)
	UpdateMasterKey(
		ctx context.Context,
		userID uuid.UUID,
		salt []byte,
		verifier []byte,
		updatedAt time.Time,
		expectedSecurityVersion int64,
	) (int64, error)
}

// masterKeyRecordRepository описывает операции с приватными записями, которые нужны при смене мастер-ключа.
type masterKeyRecordRepository interface {
	ReencryptDEKs(ctx context.Context, userID uuid.UUID, records []ReencryptedRecordDEK, updatedAt time.Time) error
}

// refreshTokenRepository описывает операции с refresh-токенами пользовательских сессий.
type refreshTokenRepository interface {
	Create(ctx context.Context, t model.RefreshToken) error
	FindActiveByHash(ctx context.Context, tokenHash string, now time.Time) (model.RefreshToken, error)
	Revoke(ctx context.Context, tokenID uuid.UUID, revokedAt time.Time) error
	RevokeActiveByUserID(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error
}

// transactor выполняет несколько операций хранилища в одной транзакции.
type transactor interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// sessionTokenIssuer выпускает access- и refresh-токены для клиентской сессии.
type sessionTokenIssuer interface {
	IssueAccessToken(userID uuid.UUID, securityVersion int64) (string, error)
	GenerateRefreshToken() (string, error)
}

type masterKeySaltValidator func(salt []byte) error

// AuthUseCase реализует сценарии аутентификации, управления токенами и смены мастер-ключа.
type AuthUseCase struct {
	userRepo              userRepository
	recordRepo            masterKeyRecordRepository
	refreshTokenRepo      refreshTokenRepository
	transactor            transactor
	recordMutationGuard   recordMutationGuard
	tokenIssuer           sessionTokenIssuer
	validateMasterKeySalt masterKeySaltValidator
	refreshTokenTTL       time.Duration
}

// NewAuthUseCase создает AuthUseCase.
func NewAuthUseCase(
	userRepo userRepository,
	recordRepo masterKeyRecordRepository,
	refreshTokenRepo refreshTokenRepository,
	transactor transactor,
	recordMutationGuard recordMutationGuard,
	tokenIssuer sessionTokenIssuer,
	validateMasterKeySalt masterKeySaltValidator,
	refreshTokenTTL time.Duration,
) (*AuthUseCase, error) {
	if userRepo == nil {
		return nil, errors.New("user repository is not provided")
	}
	if recordRepo == nil {
		return nil, errors.New("record repository is not provided")
	}
	if refreshTokenRepo == nil {
		return nil, errors.New("refresh token repository is not provided")
	}
	if transactor == nil {
		return nil, errors.New("transactor is not provided")
	}
	if recordMutationGuard == nil {
		return nil, errors.New("record mutation guard is not provided")
	}
	if tokenIssuer == nil {
		return nil, errors.New("session token issuer is not provided")
	}
	if validateMasterKeySalt == nil {
		return nil, errors.New("master key salt validator is not provided")
	}
	if refreshTokenTTL <= 0 {
		return nil, errors.New("refresh token TTL must be positive")
	}

	return &AuthUseCase{
		userRepo:              userRepo,
		recordRepo:            recordRepo,
		refreshTokenRepo:      refreshTokenRepo,
		transactor:            transactor,
		recordMutationGuard:   recordMutationGuard,
		tokenIssuer:           tokenIssuer,
		validateMasterKeySalt: validateMasterKeySalt,
		refreshTokenTTL:       refreshTokenTTL,
	}, nil
}

// Register регистрирует пользователя и возвращает пару токенов (access/refresh) и соль для мастер-ключа.
func (uc *AuthUseCase) Register(ctx context.Context, in RegisterInput) (RegisterOutput, error) {
	h, err := auth.HashPassword(in.Password)
	if err != nil {
		return RegisterOutput{}, fmt.Errorf("failed to hash password for new user: %w", err)
	}

	if err = uc.validateMasterKeySalt(in.MasterKeySalt); err != nil {
		return RegisterOutput{}, fmt.Errorf("%w: %v", ErrInvalidMasterKeySalt, err)
	}
	if len(in.MasterKeyVerifier) == 0 {
		return RegisterOutput{}, errors.New("master key verifier is required")
	}

	uuidV7, err := uuid.NewV7()
	if err != nil {
		return RegisterOutput{}, fmt.Errorf("failed to generate ID for new user: %w", err)
	}
	now := time.Now().UTC()
	u := model.User{
		ID:                uuidV7,
		Login:             in.Login,
		PasswordHash:      h,
		MasterKeySalt:     in.MasterKeySalt,
		MasterKeyVerifier: in.MasterKeyVerifier,
		SecurityVersion:   initialSecurityVersion,
		RegisteredAt:      now,
		UpdatedAt:         now,
	}

	tokens, rt, err := uc.issueTokens(u.ID, u.SecurityVersion, now)
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
		if errors.Is(err, ErrLoginAlreadyTaken) {
			return RegisterOutput{}, ErrLoginAlreadyTaken
		}
		return RegisterOutput{}, fmt.Errorf("failed to register user: %w", err)
	}

	return RegisterOutput{AuthTokens: tokens, MasterKeySalt: in.MasterKeySalt}, nil
}

// Login аутентифицирует пользователя и возвращает пару токенов (access/refresh) и соль для мастер-ключа.
func (uc *AuthUseCase) Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	u, err := uc.userRepo.GetByLogin(ctx, in.Login)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return LoginOutput{}, ErrAuthenticationFailed
		}
		return LoginOutput{}, fmt.Errorf("failed to get user by login: %w", err)
	}

	if err = auth.CheckPasswordHash(in.Password, u.PasswordHash); err != nil {
		return LoginOutput{}, ErrAuthenticationFailed
	}

	now := time.Now().UTC()
	tokens, rt, err := uc.issueTokens(u.ID, u.SecurityVersion, now)
	if err != nil {
		return LoginOutput{}, fmt.Errorf("failed to issue tokens: %w", err)
	}

	if err = uc.refreshTokenRepo.Create(ctx, rt); err != nil {
		return LoginOutput{}, fmt.Errorf("failed to save login session: %w", err)
	}

	return LoginOutput{
		AuthTokens:        tokens,
		MasterKeySalt:     u.MasterKeySalt,
		MasterKeyVerifier: u.MasterKeyVerifier,
	}, nil
}

// Refresh обновляет пару токенов по активному refresh-токену.
func (uc *AuthUseCase) Refresh(ctx context.Context, in RefreshInput) (RefreshOutput, error) {
	now := time.Now().UTC()
	activeRefreshHash := auth.HashRefreshToken(in.RefreshToken)

	var tokens AuthTokens
	// Ротация refresh-токена атомарна: старый токен отзывается только вместе с сохранением нового.
	if err := uc.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		activeRefreshToken, err := uc.refreshTokenRepo.FindActiveByHash(ctx, activeRefreshHash, now)
		if err != nil {
			if errors.Is(err, ErrRefreshTokenNotFound) {
				return ErrAuthenticationFailed
			}
			return fmt.Errorf("failed to find active refresh token: %w", err)
		}

		securityVersion, err := uc.userRepo.GetSecurityVersion(ctx, activeRefreshToken.UserID)
		if err != nil {
			if errors.Is(err, ErrUserNotFound) {
				return ErrAuthenticationFailed
			}
			return err
		}
		// security_version в refresh-токене защищает старые сессии после смены мастер-ключа.
		if securityVersion != activeRefreshToken.SecurityVersion {
			return ErrAuthenticationFailed
		}

		newTokens, newRefreshToken, err := uc.issueTokens(activeRefreshToken.UserID, securityVersion, now)
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
		if errors.Is(err, ErrAuthenticationFailed) {
			return RefreshOutput{}, ErrAuthenticationFailed
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
			if errors.Is(err, ErrRefreshTokenNotFound) {
				return ErrAuthenticationFailed
			}
			return fmt.Errorf("failed to find active refresh token: %w", err)
		}

		if err = uc.refreshTokenRepo.Revoke(ctx, activeRefreshToken.ID, now); err != nil {
			return fmt.Errorf("failed to revoke refresh token: %w", err)
		}

		return nil
	}); err != nil {
		if errors.Is(err, ErrAuthenticationFailed) {
			return ErrAuthenticationFailed
		}
		return fmt.Errorf("failed to logout user: %w", err)
	}

	return nil
}

// ChangeMasterKey меняет мастер-ключ пользователя: обновляет соль и верификатор, переупаковывает DEK приватных записей,
// повышает security_version, отзывает активные refresh-токены и возвращает новую пару токенов.
func (uc *AuthUseCase) ChangeMasterKey(ctx context.Context, in ChangeMasterKeyInput) (ChangeMasterKeyOutput, error) {
	if in.UserID == uuid.Nil {
		return ChangeMasterKeyOutput{}, ErrAuthenticationFailed
	}
	if in.SecurityVersion <= 0 {
		return ChangeMasterKeyOutput{}, ErrAuthenticationFailed
	}
	if err := uc.validateMasterKeySalt(in.MasterKeySalt); err != nil {
		return ChangeMasterKeyOutput{}, fmt.Errorf("%w: %v", ErrInvalidMasterKeySalt, err)
	}
	if len(in.MasterKeyVerifier) == 0 {
		return ChangeMasterKeyOutput{}, errors.New("master key verifier is required")
	}
	for _, record := range in.Records {
		if record.RecordID == uuid.Nil {
			return ChangeMasterKeyOutput{}, errors.New("record ID is required")
		}
		if record.ExpectedVersion <= 0 {
			return ChangeMasterKeyOutput{}, errors.New("record expected version must be positive")
		}
		if len(record.EncryptedDEK) == 0 {
			return ChangeMasterKeyOutput{}, errors.New("record encrypted DEK is required")
		}
	}

	now := time.Now().UTC()
	var tokens AuthTokens
	if err := uc.recordMutationGuard.WithUserRecordsLock(ctx, in.UserID, func(ctx context.Context) error {
		// DEK переупаковываются в той же транзакции, что и смена security_version.
		// Иначе старые сессии могли бы остаться валидными при уже измененных данных ключей.
		if err := uc.recordRepo.ReencryptDEKs(ctx, in.UserID, in.Records, now); err != nil {
			return err
		}
		newSecurityVersion, err := uc.userRepo.UpdateMasterKey(
			ctx,
			in.UserID,
			in.MasterKeySalt,
			in.MasterKeyVerifier,
			now,
			in.SecurityVersion,
		)
		if err != nil {
			return err
		}
		if err = uc.refreshTokenRepo.RevokeActiveByUserID(ctx, in.UserID, now); err != nil {
			return err
		}
		newTokens, newRefreshToken, err := uc.issueTokens(in.UserID, newSecurityVersion, now)
		if err != nil {
			return fmt.Errorf("failed to issue tokens: %w", err)
		}
		if err = uc.refreshTokenRepo.Create(ctx, newRefreshToken); err != nil {
			return fmt.Errorf("failed to persist refresh token: %w", err)
		}
		tokens = newTokens
		return nil
	}); err != nil {
		if errors.Is(err, ErrMasterKeyChangeConflict) || errors.Is(err, ErrUserNotFound) {
			return ChangeMasterKeyOutput{}, err
		}
		return ChangeMasterKeyOutput{}, fmt.Errorf("failed to change master key: %w", err)
	}
	return ChangeMasterKeyOutput{AuthTokens: tokens}, nil
}

func (uc *AuthUseCase) issueTokens(
	userID uuid.UUID,
	securityVersion int64,
	now time.Time,
) (AuthTokens, model.RefreshToken, error) {
	access, err := uc.tokenIssuer.IssueAccessToken(userID, securityVersion)
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
		ID:              uuidV7,
		UserID:          userID,
		TokenHash:       auth.HashRefreshToken(refresh),
		SecurityVersion: securityVersion,
		ExpiresAt:       now.Add(uc.refreshTokenTTL),
		IssuedAt:        now,
		RevokedAt:       nil,
	}

	return AuthTokens{AccessToken: access, RefreshToken: refresh}, rt, nil
}
