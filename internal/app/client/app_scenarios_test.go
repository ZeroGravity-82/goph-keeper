package client

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/crypto"
	"zerogravity-82/goph-keeper/internal/pb"
)

// TestApp_RegisterLoginAndStartSession проверяет клиентский путь аутентификации - регистрацию, вход и проверку
// мастер-ключа через фейковый AuthClient.
func TestApp_RegisterLoginAndStartSession(t *testing.T) {
	// Arrange
	ctx := context.Background()
	masterKey := "master-key"
	authClient := &authClientFake{}
	app := &App{auth: authClient}

	// Act
	registeredSession, err := app.Register(ctx, "user", "password", masterKey)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, authClient.registerReq)
	assert.Equal(t, "user", authClient.registerReq.GetLogin())
	assert.NotEmpty(t, authClient.registerReq.GetMasterKeySalt())
	assert.NotEmpty(t, authClient.registerReq.GetMasterKeyVerifier())
	require.NoError(t, app.StartSession(registeredSession, masterKey))
	assert.True(t, app.loggedIn)

	// Arrange
	salt, err := crypto.GenerateMasterKeySalt()
	require.NoError(t, err)
	verifier, err := crypto.EncryptMasterKeyVerifier(masterKey, salt)
	require.NoError(t, err)
	accessToken := "login-access-token"
	refreshToken := "login-refresh-token"
	authClient.loginResp = pb.LoginResponse_builder{
		AccessToken:       &accessToken,
		RefreshToken:      &refreshToken,
		MasterKeySalt:     salt,
		MasterKeyVerifier: verifier.Data,
	}.Build()

	// Act
	loginSession, err := app.Login(ctx, "user", "password")

	// Assert
	require.NoError(t, err)
	require.NoError(t, app.StartSession(loginSession, masterKey))
	assert.Equal(t, accessToken, app.session.AccessToken)
	assert.Equal(t, refreshToken, app.session.RefreshToken)
}

// TestApp_CredentialRecordLifecycle проверяет полный клиентский путь работы с обычной приватной записью - создание,
// чтение, обновление, получение списка и удаление через фейковый RecordsClient - на примере credential-записи.
func TestApp_CredentialRecordLifecycle(t *testing.T) {
	// Arrange
	ctx := context.Background()
	app := newStartedTestApp(t)

	// Act
	created, err := app.CreateCredential(ctx, CreateCredentialInput{
		Title:              "GitHub",
		Description:        "основной аккаунт",
		CredentialLogin:    "zerogravity",
		CredentialPassword: "secret",
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "record-1", created.RecordID)
	assert.Equal(t, int64(1), created.Version)

	// Act
	got, err := app.GetCredential(ctx, created.RecordID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "GitHub", got.Title)
	assert.Equal(t, "основной аккаунт", got.Description)
	assert.Equal(t, "zerogravity", got.Login)
	assert.Equal(t, "secret", got.Password)

	// Act
	updated, err := app.UpdateCredential(ctx, UpdateCredentialInput{
		RecordID:           created.RecordID,
		ExpectedVersion:    got.Version,
		Title:              "GitHub updated",
		Description:        "рабочий аккаунт",
		CredentialLogin:    "zerogravity-work",
		CredentialPassword: "new-secret",
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, created.RecordID, updated.RecordID)
	assert.Equal(t, int64(2), updated.Version)
	got, err = app.GetCredential(ctx, created.RecordID)
	require.NoError(t, err)
	assert.Equal(t, "GitHub updated", got.Title)
	assert.Equal(t, "рабочий аккаунт", got.Description)
	assert.Equal(t, "zerogravity-work", got.Login)
	assert.Equal(t, "new-secret", got.Password)

	// Act
	items, err := app.ListRecords(ctx)

	// Assert
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, created.RecordID, items[0].RecordID)
	assert.Equal(t, "credential", items[0].Type)
	assert.Equal(t, "GitHub updated", items[0].Title)

	// Act
	deleted, err := app.DeleteRecord(ctx, created.RecordID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, created.RecordID, deleted.RecordID)
	_, err = app.GetCredential(ctx, created.RecordID)
	require.Error(t, err)
	assert.Equal(t, "приватная запись не найдена", err.Error())
}

// TestApp_ListRecordsRefreshesAccessToken проверяет, что клиент обновляет пару токенов и повторяет исходный запрос,
// если access-токен протух во время клиентского сценария.
func TestApp_ListRecordsRefreshesAccessToken(t *testing.T) {
	// Arrange
	ctx := context.Background()
	recordsClient := newRecordsClientFake()
	recordsClient.listUnauthOnce = true
	newAccessToken := "new-access-token"
	newRefreshToken := "new-refresh-token"
	authClient := &authClientFake{
		refreshResp: pb.RefreshResponse_builder{
			AccessToken:  &newAccessToken,
			RefreshToken: &newRefreshToken,
		}.Build(),
	}
	app := newStartedTestApp(t)
	app.auth = authClient
	app.records = recordsClient

	_, err := app.CreateCredential(ctx, CreateCredentialInput{
		Title:              "GitHub",
		CredentialLogin:    "zerogravity",
		CredentialPassword: "secret",
	})
	require.NoError(t, err)
	oldRefreshToken := app.session.RefreshToken

	// Act
	items, err := app.ListRecords(ctx)

	// Assert
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "GitHub", items[0].Title)
	assert.Equal(t, 2, recordsClient.listCalls)
	require.NotNil(t, authClient.refreshReq)
	assert.Equal(t, oldRefreshToken, authClient.refreshReq.GetRefreshToken())
	assert.Equal(t, newAccessToken, app.session.AccessToken)
	assert.Equal(t, newRefreshToken, app.session.RefreshToken)
}

// TestApp_TextRecordRoundTrip выполняет для текстовой приватной записи ограниченную проверку - маппинг payload через
// создание и чтение.
func TestApp_TextRecordRoundTrip(t *testing.T) {
	// Arrange
	ctx := context.Background()
	app := newStartedTestApp(t)

	// Act
	created, err := app.CreateText(ctx, CreateTextInput{
		Title:       "Коды восстановления",
		Description: "Коды восстановления GitLab",
		Text:        "code-1\ncode-2",
	})

	// Assert
	require.NoError(t, err)
	got, err := app.GetText(ctx, created.RecordID)
	require.NoError(t, err)
	assert.Equal(t, "Коды восстановления", got.Title)
	assert.Equal(t, "Коды восстановления GitLab", got.Description)
	assert.Equal(t, "code-1\ncode-2", got.Text)

	// Act
	updated, err := app.UpdateText(ctx, UpdateTextInput{
		RecordID:        created.RecordID,
		ExpectedVersion: got.Version,
		Title:           "Коды восстановления новые",
		Description:     "Коды восстановления GitLab новые",
		Text:            "code-3, code-4",
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, created.RecordID, updated.RecordID)
	assert.Equal(t, int64(2), updated.Version)
	got, err = app.GetText(ctx, created.RecordID)
	require.NoError(t, err)
	assert.Equal(t, "Коды восстановления новые", got.Title)
	assert.Equal(t, "Коды восстановления GitLab новые", got.Description)
	assert.Equal(t, "code-3, code-4", got.Text)
}

// TestApp_ChangeMasterKey_ReencryptsExistingRecords проверяет смену мастер-ключа для уже существующих записей.
func TestApp_ChangeMasterKey_ReencryptsExistingRecords(t *testing.T) {
	// Arrange
	ctx := context.Background()
	app := newStartedTestApp(t)
	authClient := app.auth.(*authClientFake)

	created, err := app.CreateText(ctx, CreateTextInput{
		Title: "Секретная заметка",
		Text:  "текст, зашифрованный старым мастер-ключом",
	})
	require.NoError(t, err)

	// Act
	err = app.ChangeMasterKey(ctx, "master-key", "new-master-key")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, authClient.changeMasterKeyReq)
	assert.NotEqual(t, []byte(nil), authClient.changeMasterKeyReq.GetMasterKeySalt())
	require.Len(t, authClient.changeMasterKeyReq.GetRecords(), 1)
	assert.Equal(t, created.RecordID, authClient.changeMasterKeyReq.GetRecords()[0].GetRecordId())
	assert.Equal(t, "changed-master-key-access-token", app.session.AccessToken)
	assert.Equal(t, "changed-master-key-refresh-token", app.session.RefreshToken)
	assert.Equal(t, "new-master-key", app.masterKey)

	got, err := app.GetText(ctx, created.RecordID)
	require.NoError(t, err)
	assert.Equal(t, "текст, зашифрованный старым мастер-ключом", got.Text)

	app.masterKey = "master-key"
	_, err = app.GetText(ctx, created.RecordID)
	require.Error(t, err)
}

// TestApp_ChangeMasterKey_ValidationErrors проверяет клиентские ошибки до обращения к серверу.
func TestApp_ChangeMasterKey_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		app     func(t *testing.T) *App
		current string
		new     string
		want    string
	}{
		{
			name:    "not logged in",
			app:     func(t *testing.T) *App { return &App{} },
			current: "master-key",
			new:     "new-master-key",
			want:    "пользователь не вошел в аккаунт",
		},
		{
			name:    "empty current master key",
			app:     newStartedTestApp,
			current: "",
			new:     "new-master-key",
			want:    "текущий мастер-ключ обязателен",
		},
		{
			name:    "empty new master key",
			app:     newStartedTestApp,
			current: "master-key",
			new:     "",
			want:    "новый мастер-ключ обязателен",
		},
		{
			name:    "same master key",
			app:     newStartedTestApp,
			current: "master-key",
			new:     "master-key",
			want:    "новый мастер-ключ должен отличаться от текущего",
		},
		{
			name:    "wrong current master key",
			app:     newStartedTestApp,
			current: "wrong-master-key",
			new:     "new-master-key",
			want:    "неверный текущий мастер-ключ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			app := tt.app(t)

			// Act
			err := app.ChangeMasterKey(context.Background(), tt.current, tt.new)

			// Assert
			require.Error(t, err)
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

// TestApp_ChangeMasterKey_ReturnsConflict проверяет сообщение при серверном конфликте версий приватных записей.
func TestApp_ChangeMasterKey_ReturnsConflict(t *testing.T) {
	// Arrange
	ctx := context.Background()
	app := newStartedTestApp(t)
	authClient := app.auth.(*authClientFake)
	authClient.changeMasterKeyErr = status.Error(codes.Aborted, "records changed")

	// Act
	err := app.ChangeMasterKey(ctx, "master-key", "new-master-key")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "приватные записи изменились во время смены мастер-ключа, повторите действие", err.Error())
	assert.Equal(t, "master-key", app.masterKey)
}

// TestApp_ChangeMasterKey_RefreshesAccessToken проверяет, что при истекшем access-токене клиент обновляет сессию и
// повторяет смену мастер-ключа.
func TestApp_ChangeMasterKey_RefreshesAccessToken(t *testing.T) {
	// Arrange
	ctx := context.Background()
	app := newStartedTestApp(t)
	authClient := app.auth.(*authClientFake)
	authClient.changeMasterKeyUnauthOnce = true
	authClient.refreshResp = pb.RefreshResponse_builder{
		AccessToken:  new("new-access-token"),
		RefreshToken: new("new-refresh-token"),
	}.Build()

	// Act
	err := app.ChangeMasterKey(ctx, "master-key", "new-master-key")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, authClient.changeMasterKeyCalls)
	require.NotNil(t, authClient.refreshReq)
	assert.Equal(t, "refresh-token", authClient.refreshReq.GetRefreshToken())
	assert.Equal(t, "changed-master-key-access-token", app.session.AccessToken)
	assert.Equal(t, "changed-master-key-refresh-token", app.session.RefreshToken)
	assert.Equal(t, "new-master-key", app.masterKey)
}

// TestApp_CardRecordRoundTrip выполняет для приватной записи банковской карты ограниченную проверку - маппинг payload
// через создание и чтение.
func TestApp_CardRecordRoundTrip(t *testing.T) {
	// Arrange
	ctx := context.Background()
	app := newStartedTestApp(t)

	// Act
	created, err := app.CreateCard(ctx, CreateCardInput{
		Title:       "Основная карта",
		Description: "личная",
		Number:      "4111111111111111",
		HolderName:  "IVAN IVANOV",
		ExpiresAt:   "12/30",
		CVC:         "123",
	})

	// Assert
	require.NoError(t, err)
	got, err := app.GetCard(ctx, created.RecordID)
	require.NoError(t, err)
	assert.Equal(t, "Основная карта", got.Title)
	assert.Equal(t, "личная", got.Description)
	assert.Equal(t, "4111111111111111", got.Number)
	assert.Equal(t, "IVAN IVANOV", got.HolderName)
	assert.Equal(t, "12/30", got.ExpiresAt)
	assert.Equal(t, "123", got.CVC)

	// Act
	updated, err := app.UpdateCard(ctx, UpdateCardInput{
		RecordID:        created.RecordID,
		ExpectedVersion: got.Version,
		Title:           "Запасная карта",
		Description:     "семейная",
		Number:          "5555555555554444",
		HolderName:      "petr petrov",
		ExpiresAt:       "11/31",
		CVC:             "456",
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, created.RecordID, updated.RecordID)
	assert.Equal(t, int64(2), updated.Version)
	got, err = app.GetCard(ctx, created.RecordID)
	require.NoError(t, err)
	assert.Equal(t, "Запасная карта", got.Title)
	assert.Equal(t, "семейная", got.Description)
	assert.Equal(t, "5555555555554444", got.Number)
	assert.Equal(t, "PETR PETROV", got.HolderName)
	assert.Equal(t, "11/31", got.ExpiresAt)
	assert.Equal(t, "456", got.CVC)
}

// TestApp_CreateCard_ValidationErrors проверяет клиентские ошибки полей банковской карты до обращения к серверу.
func TestApp_CreateCard_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		in   CreateCardInput
		want string
	}{
		{
			name: "invalid number",
			in: CreateCardInput{
				Title:      "Карта",
				Number:     "4111111111111112",
				HolderName: "IVAN IVANOV",
				ExpiresAt:  "12/30",
				CVC:        "123",
			},
			want: "неверный номер карты",
		},
		{
			name: "invalid expiration",
			in: CreateCardInput{
				Title:      "Карта",
				Number:     "4111111111111111",
				HolderName: "IVAN IVANOV",
				ExpiresAt:  "13/30",
				CVC:        "123",
			},
			want: "срок действия карты должен быть в формате ММ/ГГ",
		},
		{
			name: "invalid cvc",
			in: CreateCardInput{
				Title:      "Карта",
				Number:     "4111111111111111",
				HolderName: "IVAN IVANOV",
				ExpiresAt:  "12/30",
				CVC:        "1234",
			},
			want: "CVC должен содержать ровно 3 цифры",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			app := newStartedTestApp(t)
			recordsClient := app.records.(*recordsClientFake)

			// Act
			_, err := app.CreateCard(context.Background(), tt.in)

			// Assert
			require.Error(t, err)
			assert.Equal(t, tt.want, err.Error())
			assert.Empty(t, recordsClient.records)
		})
	}
}

// TestApp_BinaryRecordLifecycle проверяет полный клиентский путь для бинарной приватной записи - создание, скачивание,
// обновление открытых метаданных, замену файла и повторное скачивание - через фейковый RecordsClient.
func TestApp_BinaryRecordLifecycle(t *testing.T) {
	// Arrange
	ctx := context.Background()
	app := newStartedTestApp(t)

	// Act
	originalFile := []byte("original file")
	created, err := app.CreateBinary(ctx, CreateBinaryInput{
		Title:       "Паспорт",
		Description: "Скан",
		Filename:    "passport.pdf",
		ContentType: "application/pdf",
		File:        bytes.NewReader(originalFile),
		FileSize:    int64(len(originalFile)),
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "record-1", created.RecordID)
	assert.Equal(t, int64(1), created.Version)

	// Act
	downloaded, err := app.DownloadBinaryFile(ctx, created.RecordID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, created.RecordID, downloaded.RecordID)
	assert.Equal(t, "passport.pdf", downloaded.Filename)
	assert.Equal(t, "application/pdf", downloaded.ContentType)
	assert.Equal(t, int64(len("original file")), downloaded.DeclaredSize)
	assert.Equal(t, []byte("original file"), downloaded.Data)

	// Act
	updatedMetadata, err := app.UpdateBinaryMetadata(ctx, UpdateBinaryMetadataInput{
		RecordID:        created.RecordID,
		ExpectedVersion: created.Version,
		Title:           "Паспорт исправленный",
		Description:     "новые паспортный данные",
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, created.RecordID, updatedMetadata.RecordID)
	assert.Equal(t, int64(2), updatedMetadata.Version)
	downloaded, err = app.DownloadBinaryFile(ctx, created.RecordID)
	require.NoError(t, err)
	assert.Equal(t, "passport.pdf", downloaded.Filename)
	assert.Equal(t, "application/pdf", downloaded.ContentType)
	assert.Equal(t, []byte("original file"), downloaded.Data)

	// Act
	newFile := []byte("new file")
	updatedFile, err := app.UpdateBinary(ctx, UpdateBinaryInput{
		RecordID:        created.RecordID,
		ExpectedVersion: updatedMetadata.Version,
		Title:           "Паспорт исправленный",
		Description:     "новый файл",
		Filename:        "passport.png",
		ContentType:     "image/png",
		File:            bytes.NewReader(newFile),
		FileSize:        int64(len(newFile)),
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, created.RecordID, updatedFile.RecordID)
	assert.Equal(t, int64(3), updatedFile.Version)
	downloaded, err = app.DownloadBinaryFile(ctx, created.RecordID)
	require.NoError(t, err)
	assert.Equal(t, "passport.png", downloaded.Filename)
	assert.Equal(t, "image/png", downloaded.ContentType)
	assert.Equal(t, int64(len("new file")), downloaded.DeclaredSize)
	assert.Equal(t, []byte("new file"), downloaded.Data)
}

// TestApp_BinaryRecordLifecycle_StreamsLargeFile проверяет загрузку и скачивание файла, который шифруется несколькими
// независимыми частями.
func TestApp_BinaryRecordLifecycle_StreamsLargeFile(t *testing.T) {
	// Arrange
	ctx := context.Background()
	app := newStartedTestApp(t)
	file := bytes.Repeat([]byte("x"), int(binaryPlainFilePartSizeBytes)+1)

	// Act
	created, err := app.CreateBinary(ctx, CreateBinaryInput{
		Title:       "Архив",
		Description: "Большой файл",
		Filename:    "archive.bin",
		ContentType: "application/octet-stream",
		File:        bytes.NewReader(file),
		FileSize:    int64(len(file)),
	})

	// Assert
	require.NoError(t, err)

	// Act
	downloaded, err := app.DownloadBinaryFile(ctx, created.RecordID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int64(len(file)), downloaded.DeclaredSize)
	assert.Equal(t, file, downloaded.Data)
}
