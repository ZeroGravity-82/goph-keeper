package client

import (
	"context"
	"fmt"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"zerogravity-82/goph-keeper/internal/crypto"
	"zerogravity-82/goph-keeper/internal/pb"
)

type authClientFake struct {
	registerReq               *pb.RegisterRequest
	loginResp                 *pb.LoginResponse
	refreshReq                *pb.RefreshRequest
	refreshResp               *pb.RefreshResponse
	refreshError              error
	logoutReq                 *pb.LogoutRequest
	changeMasterKeyReq        *pb.ChangeMasterKeyRequest
	changeMasterKeyErr        error
	changeMasterKeyCalls      int
	changeMasterKeyUnauthOnce bool
	records                   *recordsClientFake
}

func (f *authClientFake) Register(
	_ context.Context,
	req *pb.RegisterRequest,
	_ ...grpc.CallOption,
) (*pb.RegisterResponse, error) {
	f.registerReq = req
	accessToken := "registered-access-token"
	refreshToken := "registered-refresh-token"
	return pb.RegisterResponse_builder{
		AccessToken:   &accessToken,
		RefreshToken:  &refreshToken,
		MasterKeySalt: req.GetMasterKeySalt(),
	}.Build(), nil
}

func (f *authClientFake) Login(
	context.Context,
	*pb.LoginRequest,
	...grpc.CallOption,
) (*pb.LoginResponse, error) {
	return f.loginResp, nil
}

func (f *authClientFake) Refresh(
	_ context.Context,
	req *pb.RefreshRequest,
	_ ...grpc.CallOption,
) (*pb.RefreshResponse, error) {
	f.refreshReq = req
	return f.refreshResp, f.refreshError
}

func (f *authClientFake) Logout(
	_ context.Context,
	req *pb.LogoutRequest,
	_ ...grpc.CallOption,
) (*pb.LogoutResponse, error) {
	f.logoutReq = req
	return pb.LogoutResponse_builder{}.Build(), nil
}

func (f *authClientFake) ChangeMasterKey(
	_ context.Context,
	req *pb.ChangeMasterKeyRequest,
	_ ...grpc.CallOption,
) (*pb.ChangeMasterKeyResponse, error) {
	f.changeMasterKeyCalls++
	f.changeMasterKeyReq = req
	if f.changeMasterKeyUnauthOnce && f.changeMasterKeyCalls == 1 {
		return nil, status.Error(codes.Unauthenticated, "access token is invalid")
	}
	if f.changeMasterKeyErr != nil {
		return nil, f.changeMasterKeyErr
	}
	if f.records != nil {
		for _, item := range req.GetRecords() {
			record := f.records.records[item.GetRecordId()]
			if record == nil {
				continue
			}
			version := record.GetVersion() + 1
			recordID := record.GetRecordId()
			recordType := record.GetType()
			title := record.GetTitle()
			description := record.GetDescription()
			f.records.records[item.GetRecordId()] = pb.Record_builder{
				RecordId:         &recordID,
				Type:             &recordType,
				Title:            &title,
				Description:      &description,
				EncryptedDek:     item.GetEncryptedDek(),
				EncryptedPayload: record.GetEncryptedPayload(),
				Version:          &version,
				CreatedAt:        record.GetCreatedAt(),
				UpdatedAt:        fixedUpdatedRecordTimestamp(),
				File:             record.GetFile(),
			}.Build()
		}
	}
	accessToken := "changed-master-key-access-token"
	refreshToken := "changed-master-key-refresh-token"
	return pb.ChangeMasterKeyResponse_builder{
		AccessToken:  &accessToken,
		RefreshToken: &refreshToken,
	}.Build(), nil
}

type recordsClientFake struct {
	nextID         int
	records        map[string]*pb.Record
	encryptedFiles map[string][]byte
	listCalls      int
	listUnauthOnce bool
}

func newRecordsClientFake() *recordsClientFake {
	return &recordsClientFake{
		records:        make(map[string]*pb.Record),
		encryptedFiles: make(map[string][]byte),
	}
}

func (f *recordsClientFake) nextRecordID() string {
	f.nextID++
	return fmt.Sprintf("record-%d", f.nextID)
}

func fixedRecordTimestamp() *timestamppb.Timestamp {
	return timestamppb.New(time.Date(2026, time.July, 2, 10, 0, 0, 0, time.UTC))
}

func fixedUpdatedRecordTimestamp() *timestamppb.Timestamp {
	return timestamppb.New(time.Date(2026, time.July, 2, 10, 1, 0, 0, time.UTC))
}

func (f *recordsClientFake) CreateRecord(
	_ context.Context,
	req *pb.CreateRecordRequest,
	_ ...grpc.CallOption,
) (*pb.CreateRecordResponse, error) {
	recordID := f.nextRecordID()
	version := int64(1)
	now := fixedRecordTimestamp()
	recordType := req.GetType()
	title := req.GetTitle()
	description := req.GetDescription()
	f.records[recordID] = pb.Record_builder{
		RecordId:         &recordID,
		Type:             &recordType,
		Title:            &title,
		Description:      &description,
		EncryptedDek:     req.GetEncryptedDek(),
		EncryptedPayload: req.GetEncryptedPayload(),
		Version:          &version,
		CreatedAt:        now,
		UpdatedAt:        now,
	}.Build()
	return pb.CreateRecordResponse_builder{RecordId: &recordID, Version: &version}.Build(), nil
}

func (f *recordsClientFake) CreateBinaryRecord(
	ctx context.Context,
	_ ...grpc.CallOption,
) (grpc.ClientStreamingClient[pb.CreateBinaryRecordRequest, pb.CreateBinaryRecordResponse], error) {
	stream := &clientStreamFake[pb.CreateBinaryRecordRequest, pb.CreateBinaryRecordResponse]{ctx: ctx}
	var metadata *pb.CreateBinaryRecordMetadata
	var encryptedFile []byte
	stream.send = func(req *pb.CreateBinaryRecordRequest) error {
		switch req.WhichPayload() {
		case pb.CreateBinaryRecordRequest_Metadata_case:
			metadata = req.GetMetadata()
		case pb.CreateBinaryRecordRequest_Chunk_case:
			encryptedFile = append(encryptedFile, req.GetChunk()...)
		default:
			return status.Error(codes.InvalidArgument, "empty stream message")
		}
		return nil
	}
	stream.closeAndRecv = func() (*pb.CreateBinaryRecordResponse, error) {
		if metadata == nil {
			return nil, status.Error(codes.InvalidArgument, "metadata is required")
		}
		recordID := f.nextRecordID()
		version := int64(1)
		now := fixedRecordTimestamp()
		recordType := pb.RecordType_RECORD_TYPE_BINARY
		title := metadata.GetTitle()
		description := metadata.GetDescription()
		uploadStatus := pb.UploadStatus_UPLOAD_STATUS_UPLOADED
		f.records[recordID] = pb.Record_builder{
			RecordId:         &recordID,
			Type:             &recordType,
			Title:            &title,
			Description:      &description,
			EncryptedDek:     metadata.GetEncryptedDek(),
			EncryptedPayload: metadata.GetEncryptedPayload(),
			Version:          &version,
			CreatedAt:        now,
			UpdatedAt:        now,
			File: pb.RecordFile_builder{
				UploadStatus: &uploadStatus,
			}.Build(),
		}.Build()
		f.encryptedFiles[recordID] = append([]byte(nil), encryptedFile...)
		return pb.CreateBinaryRecordResponse_builder{RecordId: &recordID, Version: &version}.Build(), nil
	}
	return stream, nil
}

func (f *recordsClientFake) ListRecords(
	context.Context,
	*pb.ListRecordsRequest,
	...grpc.CallOption,
) (*pb.ListRecordsResponse, error) {
	f.listCalls++
	if f.listUnauthOnce && f.listCalls == 1 {
		return nil, status.Error(codes.Unauthenticated, "access token is invalid")
	}

	ids := make([]string, 0, len(f.records))
	for id := range f.records {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	items := make([]*pb.RecordListItem, 0, len(ids))
	for _, id := range ids {
		record := f.records[id]
		recordID := record.GetRecordId()
		recordType := record.GetType()
		title := record.GetTitle()
		description := record.GetDescription()
		items = append(items, pb.RecordListItem_builder{
			RecordId:    &recordID,
			Type:        &recordType,
			Title:       &title,
			Description: &description,
			CreatedAt:   record.GetCreatedAt(),
			UpdatedAt:   record.GetUpdatedAt(),
		}.Build())
	}
	return pb.ListRecordsResponse_builder{Items: items}.Build(), nil
}

func (f *recordsClientFake) GetRecord(
	_ context.Context,
	req *pb.GetRecordRequest,
	_ ...grpc.CallOption,
) (*pb.GetRecordResponse, error) {
	record, ok := f.records[req.GetRecordId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "record not found")
	}
	return pb.GetRecordResponse_builder{Record: record}.Build(), nil
}

func (f *recordsClientFake) UpdateRecord(
	_ context.Context,
	req *pb.UpdateRecordRequest,
	_ ...grpc.CallOption,
) (*pb.UpdateRecordResponse, error) {
	record, ok := f.records[req.GetRecordId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "record not found")
	}
	if record.GetVersion() != req.GetExpectedVersion() {
		return nil, status.Error(codes.Aborted, "record version conflict")
	}

	version := record.GetVersion() + 1
	updatedAt := fixedUpdatedRecordTimestamp()
	recordID := record.GetRecordId()
	recordType := record.GetType()
	title := req.GetTitle()
	description := req.GetDescription()
	f.records[req.GetRecordId()] = pb.Record_builder{
		RecordId:         &recordID,
		Type:             &recordType,
		Title:            &title,
		Description:      &description,
		EncryptedDek:     req.GetEncryptedDek(),
		EncryptedPayload: req.GetEncryptedPayload(),
		Version:          &version,
		CreatedAt:        record.GetCreatedAt(),
		UpdatedAt:        updatedAt,
		File:             record.GetFile(),
	}.Build()
	updatedRecordID := req.GetRecordId()
	return pb.UpdateRecordResponse_builder{
		RecordId: &updatedRecordID,
		Version:  &version,
	}.Build(), nil
}

func (f *recordsClientFake) UpdateBinaryRecord(
	ctx context.Context,
	_ ...grpc.CallOption,
) (grpc.ClientStreamingClient[pb.UpdateBinaryRecordRequest, pb.UpdateBinaryRecordResponse], error) {
	stream := &clientStreamFake[pb.UpdateBinaryRecordRequest, pb.UpdateBinaryRecordResponse]{ctx: ctx}
	var metadata *pb.UpdateBinaryRecordMetadata
	var encryptedFile []byte
	stream.send = func(req *pb.UpdateBinaryRecordRequest) error {
		switch req.WhichPayload() {
		case pb.UpdateBinaryRecordRequest_Metadata_case:
			metadata = req.GetMetadata()
		case pb.UpdateBinaryRecordRequest_Chunk_case:
			encryptedFile = append(encryptedFile, req.GetChunk()...)
		default:
			return status.Error(codes.InvalidArgument, "empty stream message")
		}
		return nil
	}
	stream.closeAndRecv = func() (*pb.UpdateBinaryRecordResponse, error) {
		if metadata == nil {
			return nil, status.Error(codes.InvalidArgument, "metadata is required")
		}
		record, ok := f.records[metadata.GetRecordId()]
		if !ok {
			return nil, status.Error(codes.NotFound, "record not found")
		}
		if record.GetVersion() != metadata.GetExpectedVersion() {
			return nil, status.Error(codes.Aborted, "record version conflict")
		}
		version := record.GetVersion() + 1
		updatedAt := fixedUpdatedRecordTimestamp()
		recordID := record.GetRecordId()
		recordType := record.GetType()
		title := metadata.GetTitle()
		description := metadata.GetDescription()
		uploadStatus := pb.UploadStatus_UPLOAD_STATUS_UPLOADED
		f.records[recordID] = pb.Record_builder{
			RecordId:         &recordID,
			Type:             &recordType,
			Title:            &title,
			Description:      &description,
			EncryptedDek:     metadata.GetEncryptedDek(),
			EncryptedPayload: metadata.GetEncryptedPayload(),
			Version:          &version,
			CreatedAt:        record.GetCreatedAt(),
			UpdatedAt:        updatedAt,
			File: pb.RecordFile_builder{
				UploadStatus: &uploadStatus,
			}.Build(),
		}.Build()
		f.encryptedFiles[recordID] = append([]byte(nil), encryptedFile...)
		return pb.UpdateBinaryRecordResponse_builder{
			RecordId: &recordID,
			Version:  &version,
		}.Build(), nil
	}
	return stream, nil
}

func (f *recordsClientFake) DeleteRecord(
	_ context.Context,
	req *pb.DeleteRecordRequest,
	_ ...grpc.CallOption,
) (*pb.DeleteRecordResponse, error) {
	if _, ok := f.records[req.GetRecordId()]; !ok {
		return nil, status.Error(codes.NotFound, "record not found")
	}
	delete(f.records, req.GetRecordId())
	deletedRecordID := req.GetRecordId()
	return pb.DeleteRecordResponse_builder{RecordId: &deletedRecordID}.Build(), nil
}

func (f *recordsClientFake) DownloadFile(
	ctx context.Context,
	req *pb.DownloadFileRequest,
	_ ...grpc.CallOption,
) (grpc.ServerStreamingClient[pb.DownloadFileResponse], error) {
	encryptedFile, ok := f.encryptedFiles[req.GetRecordId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "file not found")
	}
	response := pb.DownloadFileResponse_builder{
		Chunk: append([]byte(nil), encryptedFile...),
	}.Build()
	return &serverStreamFake[pb.DownloadFileResponse]{
		ctx:       ctx,
		responses: []*pb.DownloadFileResponse{response},
	}, nil
}

type clientStreamFake[Req any, Res any] struct {
	ctx          context.Context
	send         func(*Req) error
	closeAndRecv func() (*Res, error)
}

func (s *clientStreamFake[Req, Res]) Send(req *Req) error {
	return s.send(req)
}

func (s *clientStreamFake[Req, Res]) CloseAndRecv() (*Res, error) {
	return s.closeAndRecv()
}

func (s *clientStreamFake[Req, Res]) Header() (metadata.MD, error) {
	return nil, nil
}

func (s *clientStreamFake[Req, Res]) Trailer() metadata.MD {
	return nil
}

func (s *clientStreamFake[Req, Res]) CloseSend() error {
	return nil
}

func (s *clientStreamFake[Req, Res]) Context() context.Context {
	return s.ctx
}

func (s *clientStreamFake[Req, Res]) SendMsg(any) error {
	return nil
}

func (s *clientStreamFake[Req, Res]) RecvMsg(any) error {
	return io.EOF
}

type serverStreamFake[Res any] struct {
	ctx       context.Context
	responses []*Res
	next      int
}

func (s *serverStreamFake[Res]) Recv() (*Res, error) {
	if s.next >= len(s.responses) {
		return nil, io.EOF
	}
	resp := s.responses[s.next]
	s.next++
	return resp, nil
}

func (s *serverStreamFake[Res]) Header() (metadata.MD, error) {
	return nil, nil
}

func (s *serverStreamFake[Res]) Trailer() metadata.MD {
	return nil
}

func (s *serverStreamFake[Res]) CloseSend() error {
	return nil
}

func (s *serverStreamFake[Res]) Context() context.Context {
	return s.ctx
}

func (s *serverStreamFake[Res]) SendMsg(any) error {
	return nil
}

func (s *serverStreamFake[Res]) RecvMsg(any) error {
	return nil
}

func newStartedTestApp(t *testing.T) *App {
	t.Helper()

	salt, err := crypto.GenerateMasterKeySalt()
	require.NoError(t, err)
	verifier, err := crypto.EncryptMasterKeyVerifier("master-key", salt)
	require.NoError(t, err)
	recordsClient := newRecordsClientFake()
	return &App{
		auth:    &authClientFake{records: recordsClient},
		records: recordsClient,
		session: AuthSession{
			AccessToken:       "access-token",
			RefreshToken:      "refresh-token",
			MasterKeySalt:     salt,
			MasterKeyVerifier: verifier.Data,
		},
		masterKey: "master-key",
		loggedIn:  true,
	}
}

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
	newAccessToken := "new-access-token"
	newRefreshToken := "new-refresh-token"
	authClient.changeMasterKeyUnauthOnce = true
	authClient.refreshResp = pb.RefreshResponse_builder{
		AccessToken:  &newAccessToken,
		RefreshToken: &newRefreshToken,
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

// TestApp_BinaryRecordLifecycle проверяет полный клиентский путь для бинарной приватной записи - создание, скачивание,
// обновление открытых метаданных, замену файла и повторное скачивание - через фейковый RecordsClient.
func TestApp_BinaryRecordLifecycle(t *testing.T) {
	// Arrange
	ctx := context.Background()
	app := newStartedTestApp(t)

	// Act
	created, err := app.CreateBinary(ctx, CreateBinaryInput{
		Title:       "Паспорт",
		Description: "Скан",
		Filename:    "passport.pdf",
		ContentType: "application/pdf",
		File:        []byte("original file"),
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
	updatedFile, err := app.UpdateBinary(ctx, UpdateBinaryInput{
		RecordID:        created.RecordID,
		ExpectedVersion: updatedMetadata.Version,
		Title:           "Паспорт исправленный",
		Description:     "новый файл",
		Filename:        "passport.png",
		ContentType:     "image/png",
		File:            []byte("new file"),
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
