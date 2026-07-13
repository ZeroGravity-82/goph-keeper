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

type recordMutationGuardStub struct {
	calls         int
	err           error
	lockedUserIDs []uuid.UUID
	lockedInTx    []bool
}

func (g *recordMutationGuardStub) WithUserRecordsLock(
	ctx context.Context,
	userID uuid.UUID,
	fn func(ctx context.Context) error,
) error {
	g.calls++
	if g.err != nil {
		return g.err
	}
	g.lockedUserIDs = append(g.lockedUserIDs, userID)
	txCtx := context.WithValue(ctx, txContextKey{}, true)
	g.lockedInTx = append(g.lockedInTx, txCtx.Value(txContextKey{}) == true)
	return fn(txCtx)
}

type sessionTokenIssuerStub struct {
	accessToken         string
	refreshToken        string
	accessErr           error
	refreshErr          error
	lastSecurityVersion int64
}

func (i *sessionTokenIssuerStub) IssueAccessToken(_ uuid.UUID, securityVersion int64) (string, error) {
	if i.accessErr != nil {
		return "", i.accessErr
	}
	i.lastSecurityVersion = securityVersion
	return i.accessToken, nil
}

func (i *sessionTokenIssuerStub) GenerateRefreshToken() (string, error) {
	if i.refreshErr != nil {
		return "", i.refreshErr
	}
	return i.refreshToken, nil
}

type userRepositoryStub struct {
	usersByLogin        map[string]model.User
	securityVersion     int64
	getErr              error
	getSecurityErr      error
	createErr           error
	updateErr           error
	created             []model.User
	createdInTx         []bool
	updatedKeys         []model.User
	updatedInTx         []bool
	expectedVersions    []int64
	newSecurityVersions []int64
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

func (r *userRepositoryStub) GetSecurityVersion(context.Context, uuid.UUID) (int64, error) {
	if r.getSecurityErr != nil {
		return 0, r.getSecurityErr
	}
	if r.securityVersion > 0 {
		return r.securityVersion, nil
	}
	return initialSecurityVersion, nil
}

func (r *userRepositoryStub) UpdateMasterKey(
	ctx context.Context,
	userID uuid.UUID,
	salt []byte,
	verifier []byte,
	updatedAt time.Time,
	expectedSecurityVersion int64,
) (int64, error) {
	if r.updateErr != nil {
		return 0, r.updateErr
	}
	r.expectedVersions = append(r.expectedVersions, expectedSecurityVersion)
	newSecurityVersion := expectedSecurityVersion + 1
	r.newSecurityVersions = append(r.newSecurityVersions, newSecurityVersion)
	r.updatedKeys = append(r.updatedKeys, model.User{
		ID:                userID,
		MasterKeySalt:     salt,
		MasterKeyVerifier: verifier,
		SecurityVersion:   newSecurityVersion,
		UpdatedAt:         updatedAt,
	})
	r.updatedInTx = append(r.updatedInTx, ctx.Value(txContextKey{}) == true)
	return newSecurityVersion, nil
}

type masterKeyRecordRepositoryStub struct {
	reencryptErr  error
	reencrypted   []ReencryptedRecordDEK
	reencryptedTx []bool
}

func (r *masterKeyRecordRepositoryStub) ReencryptDEKs(
	ctx context.Context,
	_ uuid.UUID,
	records []ReencryptedRecordDEK,
	_ time.Time,
) error {
	if r.reencryptErr != nil {
		return r.reencryptErr
	}
	r.reencrypted = append(r.reencrypted, records...)
	r.reencryptedTx = append(r.reencryptedTx, ctx.Value(txContextKey{}) == true)
	return nil
}

type refreshTokenRepositoryStub struct {
	createErr          error
	findErr            error
	revokeErr          error
	revokeByUserErr    error
	activeToken        model.RefreshToken
	created            []model.RefreshToken
	createdInTx        []bool
	revokedIDs         []uuid.UUID
	revokedInTx        []bool
	revokedUserIDs     []uuid.UUID
	revokedUserIDsInTx []bool
	lastFindHash       string
	lastFindNow        time.Time
	lastRevokedAt      time.Time
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

func (r *refreshTokenRepositoryStub) RevokeActiveByUserID(
	ctx context.Context,
	userID uuid.UUID,
	revokedAt time.Time,
) error {
	if r.revokeByUserErr != nil {
		return r.revokeByUserErr
	}
	r.revokedUserIDs = append(r.revokedUserIDs, userID)
	r.revokedUserIDsInTx = append(r.revokedUserIDsInTx, ctx.Value(txContextKey{}) == true)
	r.lastRevokedAt = revokedAt
	return nil
}

func newTestAuthUseCase(t *testing.T) (
	*AuthUseCase,
	*userRepositoryStub,
	*masterKeyRecordRepositoryStub,
	*refreshTokenRepositoryStub,
	*transactorStub,
	*sessionTokenIssuerStub,
) {
	t.Helper()

	userRepo := &userRepositoryStub{}
	recordRepo := &masterKeyRecordRepositoryStub{}
	refreshRepo := &refreshTokenRepositoryStub{}
	tx := &transactorStub{}
	guard := &recordMutationGuardStub{}
	issuer := &sessionTokenIssuerStub{accessToken: "access-token", refreshToken: "refresh-token"}
	validateMasterKeySalt := func(salt []byte) error {
		if len(salt) != 16 {
			return errTest
		}
		return nil
	}
	uc, err := NewAuthUseCase(userRepo, recordRepo, refreshRepo, tx, guard, issuer, validateMasterKeySalt, time.Hour)
	require.NoError(t, err)
	return uc, userRepo, recordRepo, refreshRepo, tx, issuer
}

func testRegisterInput() RegisterInput {
	return RegisterInput{
		Login:             "user",
		Password:          "password",
		MasterKeySalt:     []byte("1234567890abcdef"),
		MasterKeyVerifier: []byte("verifier"),
	}
}

// TestAuthUseCase_Register проверяет успешную регистрацию пользователя.
func TestAuthUseCase_Register(t *testing.T) {
	// Arrange
	uc, userRepo, _, refreshRepo, tx, issuer := newTestAuthUseCase(t)

	// Act
	out, err := uc.Register(context.Background(), testRegisterInput())

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
	assert.Equal(t, []byte("verifier"), userRepo.created[0].MasterKeyVerifier)
	assert.Equal(t, int64(1), userRepo.created[0].SecurityVersion)
	assert.Equal(t, userRepo.created[0].ID, refreshRepo.created[0].UserID)
	assert.Equal(t, auth.HashRefreshToken("refresh-token"), refreshRepo.created[0].TokenHash)
	assert.Equal(t, int64(1), refreshRepo.created[0].SecurityVersion)
	assert.Equal(t, int64(1), issuer.lastSecurityVersion)
	assert.True(t, userRepo.createdInTx[0])
	assert.True(t, refreshRepo.createdInTx[0])
}

// TestAuthUseCase_Register_FailWithTakenLogin проверяет ошибку при занятом логине.
func TestAuthUseCase_Register_FailWithTakenLogin(t *testing.T) {
	// Arrange
	uc, userRepo, _, refreshRepo, tx, _ := newTestAuthUseCase(t)
	userRepo.createErr = ErrLoginAlreadyTaken

	// Act
	out, err := uc.Register(context.Background(), testRegisterInput())

	// Assert
	require.ErrorIs(t, err, ErrLoginAlreadyTaken)
	assert.Empty(t, out)
	assert.Equal(t, 1, tx.calls)
	assert.Empty(t, userRepo.created)
	assert.Empty(t, refreshRepo.created)
}

// TestAuthUseCase_Register_FailWithEmptyPassword проверяет ошибку хеширования пустого пароля.
func TestAuthUseCase_Register_FailWithEmptyPassword(t *testing.T) {
	// Arrange
	uc, _, _, _, tx, _ := newTestAuthUseCase(t)

	in := testRegisterInput()
	in.Password = ""

	// Act
	_, err := uc.Register(context.Background(), in)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to hash password")
	assert.Zero(t, tx.calls)
}

// TestAuthUseCase_Register_FailWithInvalidMasterKeyData проверяет ошибки крипто-данных мастер-ключа при регистрации.
func TestAuthUseCase_Register_FailWithInvalidMasterKeyData(t *testing.T) {
	tests := []struct {
		name string
		edit func(in *RegisterInput)
	}{
		{name: "invalid salt", edit: func(in *RegisterInput) { in.MasterKeySalt = []byte("short") }},
		{name: "empty verifier", edit: func(in *RegisterInput) { in.MasterKeyVerifier = nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			uc, _, _, _, tx, _ := newTestAuthUseCase(t)
			in := testRegisterInput()
			tt.edit(&in)

			// Act
			_, err := uc.Register(context.Background(), in)

			// Assert
			require.Error(t, err)
			if tt.name == "invalid salt" {
				require.ErrorIs(t, err, ErrInvalidMasterKeySalt)
			}
			assert.Zero(t, tx.calls)
		})
	}
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
			uc, _, _, _, tx, issuer := newTestAuthUseCase(t)
			issuer.accessErr = tt.accessErr
			issuer.refreshErr = tt.refreshErr

			// Act
			_, err := uc.Register(context.Background(), testRegisterInput())

			// Assert
			require.ErrorIs(t, err, errTest)
			assert.Zero(t, tx.calls)
		})
	}
}

// TestAuthUseCase_Register_FailWithCreateUserError проверяет ошибку создания пользователя в транзакции.
func TestAuthUseCase_Register_FailWithCreateUserError(t *testing.T) {
	// Arrange
	uc, userRepo, _, refreshRepo, tx, _ := newTestAuthUseCase(t)
	userRepo.createErr = errTest

	// Act
	_, err := uc.Register(context.Background(), testRegisterInput())

	// Assert
	require.ErrorIs(t, err, errTest)
	assert.Equal(t, 1, tx.calls)
	assert.Empty(t, refreshRepo.created)
}

// TestAuthUseCase_Register_FailWithCreateRefreshTokenError проверяет ошибку сохранения refresh-токена.
func TestAuthUseCase_Register_FailWithCreateRefreshTokenError(t *testing.T) {
	// Arrange
	uc, userRepo, _, refreshRepo, tx, _ := newTestAuthUseCase(t)
	refreshRepo.createErr = errTest

	// Act
	_, err := uc.Register(context.Background(), testRegisterInput())

	// Assert
	require.ErrorIs(t, err, errTest)
	assert.Equal(t, 1, tx.calls)
	require.Len(t, userRepo.created, 1)
}

// TestAuthUseCase_Login проверяет успешную аутентификацию пользователя.
func TestAuthUseCase_Login(t *testing.T) {
	// Arrange
	uc, userRepo, _, refreshRepo, _, issuer := newTestAuthUseCase(t)
	passwordHash, err := auth.HashPassword("password")
	require.NoError(t, err)
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000002")
	userRepo.usersByLogin = map[string]model.User{
		"user": {
			ID:                userID,
			Login:             "user",
			PasswordHash:      passwordHash,
			MasterKeySalt:     []byte("1234567890abcdef"),
			MasterKeyVerifier: []byte("verifier"),
			SecurityVersion:   3,
		},
	}

	// Act
	out, err := uc.Login(context.Background(), LoginInput{Login: "user", Password: "password"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "access-token", out.AuthTokens.AccessToken)
	assert.Equal(t, "refresh-token", out.AuthTokens.RefreshToken)
	assert.Equal(t, []byte("1234567890abcdef"), out.MasterKeySalt)
	assert.Equal(t, []byte("verifier"), out.MasterKeyVerifier)
	require.Len(t, refreshRepo.created, 1)
	assert.Equal(t, userID, refreshRepo.created[0].UserID)
	assert.Equal(t, int64(3), refreshRepo.created[0].SecurityVersion)
	assert.Equal(t, int64(3), issuer.lastSecurityVersion)
}

// TestAuthUseCase_Login_FailWithAuthenticationFailed проверяет ошибки аутентификации.
func TestAuthUseCase_Login_FailWithAuthenticationFailed(t *testing.T) {
	t.Run("user not found", func(t *testing.T) {
		// Arrange
		uc, _, _, _, _, _ := newTestAuthUseCase(t)

		// Act
		_, err := uc.Login(context.Background(), LoginInput{Login: "user", Password: "password"})

		// Assert
		require.ErrorIs(t, err, ErrAuthenticationFailed)
	})

	t.Run("wrong password", func(t *testing.T) {
		// Arrange
		uc, userRepo, _, _, _, _ := newTestAuthUseCase(t)
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
	uc, userRepo, _, _, _, _ := newTestAuthUseCase(t)
	userRepo.getErr = errTest

	// Act
	_, err := uc.Login(context.Background(), LoginInput{Login: "user", Password: "password"})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestAuthUseCase_Login_FailWithCreateRefreshTokenError проверяет ошибку сохранения refresh-токена при логине.
func TestAuthUseCase_Login_FailWithCreateRefreshTokenError(t *testing.T) {
	// Arrange
	uc, userRepo, _, refreshRepo, _, _ := newTestAuthUseCase(t)
	passwordHash, err := auth.HashPassword("password")
	require.NoError(t, err)
	userRepo.usersByLogin = map[string]model.User{"user": {PasswordHash: passwordHash, SecurityVersion: 1}}
	refreshRepo.createErr = errTest

	// Act
	_, err = uc.Login(context.Background(), LoginInput{Login: "user", Password: "password"})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestAuthUseCase_Refresh проверяет успешную ротацию refresh-токена.
func TestAuthUseCase_Refresh(t *testing.T) {
	// Arrange
	uc, userRepo, _, refreshRepo, tx, issuer := newTestAuthUseCase(t)
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000003")
	oldTokenID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000004")
	userRepo.securityVersion = 4
	refreshRepo.activeToken = model.RefreshToken{ID: oldTokenID, UserID: userID, SecurityVersion: 4}

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
	assert.Equal(t, int64(4), refreshRepo.created[0].SecurityVersion)
	assert.Equal(t, int64(4), issuer.lastSecurityVersion)
	assert.True(t, refreshRepo.revokedInTx[0])
	assert.True(t, refreshRepo.createdInTx[0])
}

// TestAuthUseCase_Refresh_FailWithOutdatedSecurityVersion проверяет ошибку при refresh-токене от старой версии
// security-состояния пользователя.
func TestAuthUseCase_Refresh_FailWithOutdatedSecurityVersion(t *testing.T) {
	// Arrange
	uc, userRepo, _, refreshRepo, _, _ := newTestAuthUseCase(t)
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000013")
	userRepo.securityVersion = 2
	refreshRepo.activeToken = model.RefreshToken{ID: uuid.New(), UserID: userID, SecurityVersion: 1}

	// Act
	_, err := uc.Refresh(context.Background(), RefreshInput{RefreshToken: "old-refresh"})

	// Assert
	require.ErrorIs(t, err, ErrAuthenticationFailed)
	assert.Empty(t, refreshRepo.revokedIDs)
	assert.Empty(t, refreshRepo.created)
}

// TestAuthUseCase_Refresh_FailWithAuthenticationFailed проверяет ошибку при отсутствующем refresh-токене.
func TestAuthUseCase_Refresh_FailWithAuthenticationFailed(t *testing.T) {
	// Arrange
	uc, _, _, refreshRepo, _, _ := newTestAuthUseCase(t)
	refreshRepo.findErr = ErrRefreshTokenNotFound

	// Act
	_, err := uc.Refresh(context.Background(), RefreshInput{RefreshToken: "old-refresh"})

	// Assert
	require.ErrorIs(t, err, ErrAuthenticationFailed)
}

// TestAuthUseCase_Refresh_FailWithRepositoryError проверяет ошибку поиска refresh-токена.
func TestAuthUseCase_Refresh_FailWithRepositoryError(t *testing.T) {
	// Arrange
	uc, _, _, refreshRepo, _, _ := newTestAuthUseCase(t)
	refreshRepo.findErr = errTest

	// Act
	_, err := uc.Refresh(context.Background(), RefreshInput{RefreshToken: "old-refresh"})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestAuthUseCase_Refresh_FailWithTokenIssueError проверяет ошибку выпуска новой пары токенов.
func TestAuthUseCase_Refresh_FailWithTokenIssueError(t *testing.T) {
	// Arrange
	uc, _, _, refreshRepo, _, issuer := newTestAuthUseCase(t)
	refreshRepo.activeToken = model.RefreshToken{
		ID:              uuid.MustParse("018f6b7c-0000-7000-8000-000000000005"),
		SecurityVersion: 1,
	}
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
	uc, _, _, refreshRepo, _, _ := newTestAuthUseCase(t)
	refreshRepo.activeToken = model.RefreshToken{
		ID:              uuid.MustParse("018f6b7c-0000-7000-8000-000000000006"),
		SecurityVersion: 1,
	}
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
	uc, _, _, refreshRepo, _, _ := newTestAuthUseCase(t)
	refreshRepo.activeToken = model.RefreshToken{
		ID:              uuid.MustParse("018f6b7c-0000-7000-8000-000000000007"),
		SecurityVersion: 1,
	}
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
	uc, _, _, refreshRepo, tx, _ := newTestAuthUseCase(t)
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
	uc, _, _, refreshRepo, _, _ := newTestAuthUseCase(t)
	refreshRepo.findErr = ErrRefreshTokenNotFound

	// Act
	err := uc.Logout(context.Background(), LogoutInput{RefreshToken: "refresh"})

	// Assert
	require.ErrorIs(t, err, ErrAuthenticationFailed)
}

// TestAuthUseCase_Logout_FailWithRevokeError проверяет ошибку отзыва refresh-токена.
func TestAuthUseCase_Logout_FailWithRevokeError(t *testing.T) {
	// Arrange
	uc, _, _, refreshRepo, _, _ := newTestAuthUseCase(t)
	refreshRepo.activeToken = model.RefreshToken{ID: uuid.MustParse("018f6b7c-0000-7000-8000-000000000009")}
	refreshRepo.revokeErr = errTest

	// Act
	err := uc.Logout(context.Background(), LogoutInput{RefreshToken: "refresh"})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestAuthUseCase_ChangeMasterKey проверяет смену мастер-ключа и переупаковку DEK под guard-блокировкой записей
// пользователя.
func TestAuthUseCase_ChangeMasterKey(t *testing.T) {
	// Arrange
	uc, userRepo, recordRepo, refreshRepo, tx, issuer := newTestAuthUseCase(t)
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000010")
	recordID := uuid.MustParse("018f6b7c-0000-7000-8000-200000000010")
	in := ChangeMasterKeyInput{
		UserID:            userID,
		SecurityVersion:   5,
		MasterKeySalt:     []byte("abcdef1234567890"),
		MasterKeyVerifier: []byte("new-verifier"),
		Records: []ReencryptedRecordDEK{
			{RecordID: recordID, ExpectedVersion: 2, EncryptedDEK: []byte("new-encrypted-dek")},
		},
	}

	// Act
	out, err := uc.ChangeMasterKey(context.Background(), in)

	// Assert
	require.NoError(t, err)
	guard := uc.recordMutationGuard.(*recordMutationGuardStub)
	assert.Equal(t, "access-token", out.AuthTokens.AccessToken)
	assert.Equal(t, "refresh-token", out.AuthTokens.RefreshToken)
	assert.Zero(t, tx.calls)
	assert.Equal(t, 1, guard.calls)
	require.Len(t, recordRepo.reencrypted, 1)
	assert.Equal(t, recordID, recordRepo.reencrypted[0].RecordID)
	assert.Equal(t, []uuid.UUID{userID}, guard.lockedUserIDs)
	require.Len(t, userRepo.updatedKeys, 1)
	assert.Equal(t, userID, userRepo.updatedKeys[0].ID)
	assert.Equal(t, []byte("abcdef1234567890"), userRepo.updatedKeys[0].MasterKeySalt)
	assert.Equal(t, []byte("new-verifier"), userRepo.updatedKeys[0].MasterKeyVerifier)
	assert.Equal(t, int64(5), userRepo.expectedVersions[0])
	assert.Equal(t, int64(6), userRepo.updatedKeys[0].SecurityVersion)
	assert.Equal(t, []uuid.UUID{userID}, refreshRepo.revokedUserIDs)
	require.Len(t, refreshRepo.created, 1)
	assert.Equal(t, userID, refreshRepo.created[0].UserID)
	assert.Equal(t, int64(6), refreshRepo.created[0].SecurityVersion)
	assert.Equal(t, int64(6), issuer.lastSecurityVersion)
	assert.True(t, guard.lockedInTx[0])
	assert.True(t, recordRepo.reencryptedTx[0])
	assert.True(t, userRepo.updatedInTx[0])
	assert.True(t, refreshRepo.revokedUserIDsInTx[0])
	assert.True(t, refreshRepo.createdInTx[0])
}

// TestAuthUseCase_ChangeMasterKey_FailWithRecordConflict проверяет проброс конфликта записей при смене мастер-ключа.
func TestAuthUseCase_ChangeMasterKey_FailWithRecordConflict(t *testing.T) {
	// Arrange
	uc, userRepo, recordRepo, _, _, _ := newTestAuthUseCase(t)
	recordRepo.reencryptErr = ErrMasterKeyChangeConflict
	in := ChangeMasterKeyInput{
		UserID:            uuid.MustParse("018f6b7c-0000-7000-8000-100000000011"),
		SecurityVersion:   1,
		MasterKeySalt:     []byte("abcdef1234567890"),
		MasterKeyVerifier: []byte("new-verifier"),
		Records: []ReencryptedRecordDEK{
			{
				RecordID:        uuid.MustParse("018f6b7c-0000-7000-8000-200000000011"),
				ExpectedVersion: 2,
				EncryptedDEK:    []byte("new-encrypted-dek"),
			},
		},
	}

	// Act
	_, err := uc.ChangeMasterKey(context.Background(), in)

	// Assert
	require.ErrorIs(t, err, ErrMasterKeyChangeConflict)
	assert.Empty(t, userRepo.updatedKeys)
}
