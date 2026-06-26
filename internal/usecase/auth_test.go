package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/auth"
	"zerogravity-82/goph-keeper/internal/domain/model"
)

var errTest = errors.New("test error")

type txContextKey struct{}

type transactorStub struct {
	calls int
	err   error
}

func (t *transactorStub) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	t.calls++
	if t.err != nil {
		return t.err
	}
	return fn(context.WithValue(ctx, txContextKey{}, true))
}

type sessionTokenIssuerStub struct {
	accessToken  string
	refreshToken string
	accessErr    error
	refreshErr   error
}

func (i *sessionTokenIssuerStub) IssueAccessToken(uuid.UUID) (string, error) {
	if i.accessErr != nil {
		return "", i.accessErr
	}
	return i.accessToken, nil
}

func (i *sessionTokenIssuerStub) GenerateRefreshToken() (string, error) {
	if i.refreshErr != nil {
		return "", i.refreshErr
	}
	return i.refreshToken, nil
}

type userRepositoryStub struct {
	usersByLogin map[string]model.User
	getErr       error
	createErr    error
	created      []model.User
	createdInTx  []bool
}

func (r *userRepositoryStub) Create(ctx context.Context, u model.User) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, u)
	r.createdInTx = append(r.createdInTx, ctx.Value(txContextKey{}) == true)
	return nil
}

func (r *userRepositoryStub) GetByLogin(context.Context, string) (model.User, error) {
	if r.getErr != nil {
		return model.User{}, r.getErr
	}
	if r.usersByLogin == nil {
		return model.User{}, ErrUserNotFound
	}
	for _, u := range r.usersByLogin {
		return u, nil
	}
	return model.User{}, ErrUserNotFound
}

type refreshTokenRepositoryStub struct {
	createErr     error
	findErr       error
	revokeErr     error
	activeToken   model.RefreshToken
	created       []model.RefreshToken
	createdInTx   []bool
	revokedIDs    []uuid.UUID
	revokedInTx   []bool
	lastFindHash  string
	lastFindNow   time.Time
	lastRevokedAt time.Time
}

func (r *refreshTokenRepositoryStub) Create(ctx context.Context, token model.RefreshToken) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, token)
	r.createdInTx = append(r.createdInTx, ctx.Value(txContextKey{}) == true)
	return nil
}

func (r *refreshTokenRepositoryStub) FindActiveByHash(
	_ context.Context,
	tokenHash string,
	now time.Time,
) (model.RefreshToken, error) {
	r.lastFindHash = tokenHash
	r.lastFindNow = now
	if r.findErr != nil {
		return model.RefreshToken{}, r.findErr
	}
	return r.activeToken, nil
}

func (r *refreshTokenRepositoryStub) Revoke(ctx context.Context, tokenID uuid.UUID, revokedAt time.Time) error {
	if r.revokeErr != nil {
		return r.revokeErr
	}
	r.revokedIDs = append(r.revokedIDs, tokenID)
	r.revokedInTx = append(r.revokedInTx, ctx.Value(txContextKey{}) == true)
	r.lastRevokedAt = revokedAt
	return nil
}

func newTestAuthUseCase(t *testing.T) (
	*AuthUseCase,
	*userRepositoryStub,
	*refreshTokenRepositoryStub,
	*transactorStub,
	*sessionTokenIssuerStub,
) {
	t.Helper()

	userRepo := &userRepositoryStub{}
	refreshRepo := &refreshTokenRepositoryStub{}
	tx := &transactorStub{}
	issuer := &sessionTokenIssuerStub{accessToken: "access-token", refreshToken: "refresh-token"}
	uc, err := NewAuthUseCase(userRepo, refreshRepo, tx, issuer, time.Hour)
	require.NoError(t, err)
	return uc, userRepo, refreshRepo, tx, issuer
}

// TestAuthUseCase_Register проверяет успешную регистрацию пользователя.
func TestAuthUseCase_Register(t *testing.T) {
	// Arrange
	uc, userRepo, refreshRepo, tx, _ := newTestAuthUseCase(t)

	// Act
	out, err := uc.Register(context.Background(), RegisterInput{Login: "user", Password: "password"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "access-token", out.AuthTokens.AccessToken)
	assert.Equal(t, "refresh-token", out.AuthTokens.RefreshToken)
	assert.Len(t, out.MasterKeySalt, 16)
	assert.Equal(t, 1, tx.calls)
	require.Len(t, userRepo.created, 1)
	require.Len(t, refreshRepo.created, 1)
	assert.Equal(t, "user", userRepo.created[0].Login)
	assert.NotEmpty(t, userRepo.created[0].PasswordHash)
	assert.Equal(t, out.MasterKeySalt, userRepo.created[0].MasterKeySalt)
	assert.Equal(t, userRepo.created[0].ID, refreshRepo.created[0].UserID)
	assert.Equal(t, auth.HashRefreshToken("refresh-token"), refreshRepo.created[0].TokenHash)
	assert.True(t, userRepo.createdInTx[0])
	assert.True(t, refreshRepo.createdInTx[0])
}

// TestAuthUseCase_Register_FailWithTakenLogin проверяет ошибку при занятом логине.
func TestAuthUseCase_Register_FailWithTakenLogin(t *testing.T) {
	// Arrange
	uc, userRepo, refreshRepo, tx, _ := newTestAuthUseCase(t)
	userRepo.usersByLogin = map[string]model.User{"user": {ID: uuid.MustParse("018f6b7c-0000-7000-8000-000000000001")}}

	// Act
	out, err := uc.Register(context.Background(), RegisterInput{Login: "user", Password: "password"})

	// Assert
	require.ErrorIs(t, err, ErrLoginAlreadyTaken)
	assert.Empty(t, out)
	assert.Zero(t, tx.calls)
	assert.Empty(t, refreshRepo.created)
}

// TestAuthUseCase_Register_FailWithLoginLookupError проверяет ошибку проверки уникальности логина.
func TestAuthUseCase_Register_FailWithLoginLookupError(t *testing.T) {
	// Arrange
	uc, userRepo, _, tx, _ := newTestAuthUseCase(t)
	userRepo.getErr = errTest

	// Act
	_, err := uc.Register(context.Background(), RegisterInput{Login: "user", Password: "password"})

	// Assert
	require.ErrorIs(t, err, errTest)
	assert.Zero(t, tx.calls)
}

// TestAuthUseCase_Register_FailWithEmptyPassword проверяет ошибку хеширования пустого пароля.
func TestAuthUseCase_Register_FailWithEmptyPassword(t *testing.T) {
	// Arrange
	uc, _, _, tx, _ := newTestAuthUseCase(t)

	// Act
	_, err := uc.Register(context.Background(), RegisterInput{Login: "user", Password: ""})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to hash password")
	assert.Zero(t, tx.calls)
}

// TestAuthUseCase_Register_FailWithTokenIssueError проверяет ошибки выпуска токенов.
func TestAuthUseCase_Register_FailWithTokenIssueError(t *testing.T) {
	tests := []struct {
		name       string
		accessErr  error
		refreshErr error
	}{
		{name: "access token", accessErr: errTest},
		{name: "refresh token", refreshErr: errTest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			uc, _, _, tx, issuer := newTestAuthUseCase(t)
			issuer.accessErr = tt.accessErr
			issuer.refreshErr = tt.refreshErr

			// Act
			_, err := uc.Register(context.Background(), RegisterInput{Login: "user", Password: "password"})

			// Assert
			require.ErrorIs(t, err, errTest)
			assert.Zero(t, tx.calls)
		})
	}
}

// TestAuthUseCase_Register_FailWithCreateUserError проверяет ошибку создания пользователя в транзакции.
func TestAuthUseCase_Register_FailWithCreateUserError(t *testing.T) {
	// Arrange
	uc, userRepo, refreshRepo, tx, _ := newTestAuthUseCase(t)
	userRepo.createErr = errTest

	// Act
	_, err := uc.Register(context.Background(), RegisterInput{Login: "user", Password: "password"})

	// Assert
	require.ErrorIs(t, err, errTest)
	assert.Equal(t, 1, tx.calls)
	assert.Empty(t, refreshRepo.created)
}

// TestAuthUseCase_Register_FailWithCreateRefreshTokenError проверяет ошибку сохранения refresh-токена.
func TestAuthUseCase_Register_FailWithCreateRefreshTokenError(t *testing.T) {
	// Arrange
	uc, userRepo, refreshRepo, tx, _ := newTestAuthUseCase(t)
	refreshRepo.createErr = errTest

	// Act
	_, err := uc.Register(context.Background(), RegisterInput{Login: "user", Password: "password"})

	// Assert
	require.ErrorIs(t, err, errTest)
	assert.Equal(t, 1, tx.calls)
	require.Len(t, userRepo.created, 1)
}

// TestAuthUseCase_Login проверяет успешную аутентификацию пользователя.
func TestAuthUseCase_Login(t *testing.T) {
	// Arrange
	uc, userRepo, refreshRepo, _, _ := newTestAuthUseCase(t)
	passwordHash, err := auth.HashPassword("password")
	require.NoError(t, err)
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000002")
	userRepo.usersByLogin = map[string]model.User{
		"user": {ID: userID, Login: "user", PasswordHash: passwordHash, MasterKeySalt: []byte("1234567890abcdef")},
	}

	// Act
	out, err := uc.Login(context.Background(), LoginInput{Login: "user", Password: "password"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "access-token", out.AuthTokens.AccessToken)
	assert.Equal(t, "refresh-token", out.AuthTokens.RefreshToken)
	assert.Equal(t, []byte("1234567890abcdef"), out.MasterKeySalt)
	require.Len(t, refreshRepo.created, 1)
	assert.Equal(t, userID, refreshRepo.created[0].UserID)
}

// TestAuthUseCase_Login_FailWithAuthenticationFailed проверяет ошибки аутентификации.
func TestAuthUseCase_Login_FailWithAuthenticationFailed(t *testing.T) {
	t.Run("user not found", func(t *testing.T) {
		// Arrange
		uc, _, _, _, _ := newTestAuthUseCase(t)

		// Act
		_, err := uc.Login(context.Background(), LoginInput{Login: "user", Password: "password"})

		// Assert
		require.ErrorIs(t, err, ErrAuthenticationFailed)
	})

	t.Run("wrong password", func(t *testing.T) {
		// Arrange
		uc, userRepo, _, _, _ := newTestAuthUseCase(t)
		passwordHash, err := auth.HashPassword("password")
		require.NoError(t, err)
		userRepo.usersByLogin = map[string]model.User{"user": {PasswordHash: passwordHash}}

		// Act
		_, err = uc.Login(context.Background(), LoginInput{Login: "user", Password: "wrong"})

		// Assert
		require.ErrorIs(t, err, ErrAuthenticationFailed)
	})
}

// TestAuthUseCase_Login_FailWithRepositoryError проверяет ошибку чтения пользователя.
func TestAuthUseCase_Login_FailWithRepositoryError(t *testing.T) {
	// Arrange
	uc, userRepo, _, _, _ := newTestAuthUseCase(t)
	userRepo.getErr = errTest

	// Act
	_, err := uc.Login(context.Background(), LoginInput{Login: "user", Password: "password"})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestAuthUseCase_Login_FailWithCreateRefreshTokenError проверяет ошибку сохранения refresh-токена при логине.
func TestAuthUseCase_Login_FailWithCreateRefreshTokenError(t *testing.T) {
	// Arrange
	uc, userRepo, refreshRepo, _, _ := newTestAuthUseCase(t)
	passwordHash, err := auth.HashPassword("password")
	require.NoError(t, err)
	userRepo.usersByLogin = map[string]model.User{"user": {PasswordHash: passwordHash}}
	refreshRepo.createErr = errTest

	// Act
	_, err = uc.Login(context.Background(), LoginInput{Login: "user", Password: "password"})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestAuthUseCase_Refresh проверяет успешную ротацию refresh-токена.
func TestAuthUseCase_Refresh(t *testing.T) {
	// Arrange
	uc, _, refreshRepo, tx, _ := newTestAuthUseCase(t)
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000003")
	oldTokenID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000004")
	refreshRepo.activeToken = model.RefreshToken{ID: oldTokenID, UserID: userID}

	// Act
	out, err := uc.Refresh(context.Background(), RefreshInput{RefreshToken: "old-refresh"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "access-token", out.AuthTokens.AccessToken)
	assert.Equal(t, "refresh-token", out.AuthTokens.RefreshToken)
	assert.Equal(t, auth.HashRefreshToken("old-refresh"), refreshRepo.lastFindHash)
	assert.Equal(t, 1, tx.calls)
	assert.Equal(t, []uuid.UUID{oldTokenID}, refreshRepo.revokedIDs)
	require.Len(t, refreshRepo.created, 1)
	assert.Equal(t, userID, refreshRepo.created[0].UserID)
	assert.True(t, refreshRepo.revokedInTx[0])
	assert.True(t, refreshRepo.createdInTx[0])
}

// TestAuthUseCase_Refresh_FailWithAuthenticationFailed проверяет ошибку при отсутствующем refresh-токене.
func TestAuthUseCase_Refresh_FailWithAuthenticationFailed(t *testing.T) {
	// Arrange
	uc, _, refreshRepo, _, _ := newTestAuthUseCase(t)
	refreshRepo.findErr = ErrRefreshTokenNotFound

	// Act
	_, err := uc.Refresh(context.Background(), RefreshInput{RefreshToken: "old-refresh"})

	// Assert
	require.ErrorIs(t, err, ErrAuthenticationFailed)
}

// TestAuthUseCase_Refresh_FailWithRepositoryError проверяет ошибку поиска refresh-токена.
func TestAuthUseCase_Refresh_FailWithRepositoryError(t *testing.T) {
	// Arrange
	uc, _, refreshRepo, _, _ := newTestAuthUseCase(t)
	refreshRepo.findErr = errTest

	// Act
	_, err := uc.Refresh(context.Background(), RefreshInput{RefreshToken: "old-refresh"})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestAuthUseCase_Refresh_FailWithTokenIssueError проверяет ошибку выпуска новой пары токенов.
func TestAuthUseCase_Refresh_FailWithTokenIssueError(t *testing.T) {
	// Arrange
	uc, _, refreshRepo, _, issuer := newTestAuthUseCase(t)
	refreshRepo.activeToken = model.RefreshToken{ID: uuid.MustParse("018f6b7c-0000-7000-8000-000000000005")}
	issuer.accessErr = errTest

	// Act
	_, err := uc.Refresh(context.Background(), RefreshInput{RefreshToken: "old-refresh"})

	// Assert
	require.ErrorIs(t, err, errTest)
	assert.Empty(t, refreshRepo.revokedIDs)
	assert.Empty(t, refreshRepo.created)
}

// TestAuthUseCase_Refresh_FailWithRevokeError проверяет ошибку отзыва старого refresh-токена.
func TestAuthUseCase_Refresh_FailWithRevokeError(t *testing.T) {
	// Arrange
	uc, _, refreshRepo, _, _ := newTestAuthUseCase(t)
	refreshRepo.activeToken = model.RefreshToken{ID: uuid.MustParse("018f6b7c-0000-7000-8000-000000000006")}
	refreshRepo.revokeErr = errTest

	// Act
	_, err := uc.Refresh(context.Background(), RefreshInput{RefreshToken: "old-refresh"})

	// Assert
	require.ErrorIs(t, err, errTest)
	assert.Empty(t, refreshRepo.created)
}

// TestAuthUseCase_Refresh_FailWithCreateRefreshTokenError проверяет ошибку сохранения нового refresh-токена.
func TestAuthUseCase_Refresh_FailWithCreateRefreshTokenError(t *testing.T) {
	// Arrange
	uc, _, refreshRepo, _, _ := newTestAuthUseCase(t)
	refreshRepo.activeToken = model.RefreshToken{ID: uuid.MustParse("018f6b7c-0000-7000-8000-000000000007")}
	refreshRepo.createErr = errTest

	// Act
	_, err := uc.Refresh(context.Background(), RefreshInput{RefreshToken: "old-refresh"})

	// Assert
	require.ErrorIs(t, err, errTest)
	assert.Len(t, refreshRepo.revokedIDs, 1)
}

// TestAuthUseCase_Logout проверяет успешное завершение пользовательской сессии.
func TestAuthUseCase_Logout(t *testing.T) {
	// Arrange
	uc, _, refreshRepo, tx, _ := newTestAuthUseCase(t)
	tokenID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000008")
	refreshRepo.activeToken = model.RefreshToken{ID: tokenID}

	// Act
	err := uc.Logout(context.Background(), LogoutInput{RefreshToken: "refresh"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, tx.calls)
	assert.Equal(t, auth.HashRefreshToken("refresh"), refreshRepo.lastFindHash)
	assert.Equal(t, []uuid.UUID{tokenID}, refreshRepo.revokedIDs)
	assert.True(t, refreshRepo.revokedInTx[0])
}

// TestAuthUseCase_Logout_FailWithAuthenticationFailed проверяет ошибку при отсутствующем refresh-токене.
func TestAuthUseCase_Logout_FailWithAuthenticationFailed(t *testing.T) {
	// Arrange
	uc, _, refreshRepo, _, _ := newTestAuthUseCase(t)
	refreshRepo.findErr = ErrRefreshTokenNotFound

	// Act
	err := uc.Logout(context.Background(), LogoutInput{RefreshToken: "refresh"})

	// Assert
	require.ErrorIs(t, err, ErrAuthenticationFailed)
}

// TestAuthUseCase_Logout_FailWithRevokeError проверяет ошибку отзыва refresh-токена.
func TestAuthUseCase_Logout_FailWithRevokeError(t *testing.T) {
	// Arrange
	uc, _, refreshRepo, _, _ := newTestAuthUseCase(t)
	refreshRepo.activeToken = model.RefreshToken{ID: uuid.MustParse("018f6b7c-0000-7000-8000-000000000009")}
	refreshRepo.revokeErr = errTest

	// Act
	err := uc.Logout(context.Background(), LogoutInput{RefreshToken: "refresh"})

	// Assert
	require.ErrorIs(t, err, errTest)
}
