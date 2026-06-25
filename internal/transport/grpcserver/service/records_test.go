package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/logging"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/transport/grpcserver/authcontext"
	"zerogravity-82/goph-keeper/internal/usecase"
)

type recordsUseCaseStub struct {
	createRecordInput        usecase.CreateRecordInput
	createRecordOutput       usecase.CreateRecordOutput
	createRecordErr          error
	createBinaryRecordInput  usecase.CreateBinaryRecordInput
	createBinaryRecordFile   []byte
	createBinaryRecordOutput usecase.CreateBinaryRecordOutput
	createBinaryRecordErr    error
	listRecordsInput         usecase.ListRecordsInput
	listRecordsOutput        usecase.ListRecordsOutput
	listRecordsErr           error
	getRecordInput           usecase.GetRecordInput
	getRecordOutput          usecase.GetRecordOutput
	getRecordErr             error
	downloadFileInput        usecase.DownloadFileInput
	downloadFileOutput       usecase.DownloadFileOutput
	downloadFileErr          error
}

func (s *recordsUseCaseStub) CreateRecord(
	_ context.Context,
	in usecase.CreateRecordInput,
) (usecase.CreateRecordOutput, error) {
	s.createRecordInput = in
	return s.createRecordOutput, s.createRecordErr
}

func (s *recordsUseCaseStub) CreateBinaryRecord(
	_ context.Context,
	in usecase.CreateBinaryRecordInput,
) (usecase.CreateBinaryRecordOutput, error) {
	s.createBinaryRecordInput = in
	if in.EncryptedFile != nil {
		data, err := io.ReadAll(in.EncryptedFile)
		if err != nil {
			return usecase.CreateBinaryRecordOutput{}, err
		}
		s.createBinaryRecordFile = data
	}
	return s.createBinaryRecordOutput, s.createBinaryRecordErr
}

func (s *recordsUseCaseStub) ListRecords(
	_ context.Context,
	in usecase.ListRecordsInput,
) (usecase.ListRecordsOutput, error) {
	s.listRecordsInput = in
	return s.listRecordsOutput, s.listRecordsErr
}

func (s *recordsUseCaseStub) GetRecord(
	_ context.Context,
	in usecase.GetRecordInput,
) (usecase.GetRecordOutput, error) {
	s.getRecordInput = in
	return s.getRecordOutput, s.getRecordErr
}

func (s *recordsUseCaseStub) DownloadFile(
	_ context.Context,
	in usecase.DownloadFileInput,
) (usecase.DownloadFileOutput, error) {
	s.downloadFileInput = in
	return s.downloadFileOutput, s.downloadFileErr
}

// TestRecordsService_CreateRecord_OK проверяет успешное создание приватной записи через gRPC-обработчик.
func TestRecordsService_CreateRecord_OK(t *testing.T) {
	// Arrange
	userID := uuid.Must(uuid.NewV7())
	recordID := uuid.Must(uuid.NewV7())
	uc := &recordsUseCaseStub{createRecordOutput: usecase.CreateRecordOutput{RecordID: recordID, Version: 1}}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	recordType := pb.RecordType_RECORD_TYPE_TEXT
	req := pb.CreateRecordRequest_builder{
		Type:             &recordType,
		Title:            new("title"),
		Description:      new("description"),
		EncryptedDek:     []byte("encrypted-dek"),
		EncryptedPayload: []byte("encrypted-payload"),
	}.Build()
	ctx := authcontext.WithUserID(context.Background(), userID)

	// Act
	resp, err := recordsService.CreateRecord(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, recordID.String(), resp.GetRecordId())
	assert.Equal(t, int64(1), resp.GetVersion())
	assert.Equal(t, userID, uc.createRecordInput.UserID)
	assert.Equal(t, model.RecordTypeText, uc.createRecordInput.Type)
	assert.Equal(t, "title", uc.createRecordInput.Title)
	assert.Equal(t, "description", uc.createRecordInput.Description)
	assert.Equal(t, []byte("encrypted-dek"), uc.createRecordInput.EncryptedDEK)
	assert.Equal(t, []byte("encrypted-payload"), uc.createRecordInput.EncryptedPayload)
}

// TestRecordsService_CreateRecord_FailWithUnauthenticated проверяет ошибку при отсутствии идентификатора пользователя
// в контексте.
func TestRecordsService_CreateRecord_FailWithUnauthenticated(t *testing.T) {
	// Arrange
	recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
	require.NoError(t, err)
	recordType := pb.RecordType_RECORD_TYPE_TEXT
	req := pb.CreateRecordRequest_builder{
		Type:             &recordType,
		Title:            new("title"),
		EncryptedDek:     []byte("encrypted-dek"),
		EncryptedPayload: []byte("encrypted-payload"),
	}.Build()

	// Act
	_, err = recordsService.CreateRecord(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestRecordsService_CreateRecord_FailWithInvalidArgument проверяет валидацию обязательных полей запроса.
func TestRecordsService_CreateRecord_FailWithInvalidArgument(t *testing.T) {
	// Arrange
	textType := pb.RecordType_RECORD_TYPE_TEXT
	tests := []struct {
		name string
		req  *pb.CreateRecordRequest
	}{
		{name: "nil request", req: nil},
		{name: "unspecified type", req: newCreateRecordRequest(
			pb.RecordType_RECORD_TYPE_UNSPECIFIED,
			"title",
			[]byte("dek"),
			[]byte("payload")),
		},
		{name: "empty title", req: newCreateRecordRequest(textType, "", []byte("dek"), []byte("payload"))},
		{name: "blank title", req: newCreateRecordRequest(textType, "   ", []byte("dek"), []byte("payload"))},
		{name: "empty encrypted dek", req: newCreateRecordRequest(textType, "title", nil, []byte("payload"))},
		{name: "empty encrypted payload", req: newCreateRecordRequest(textType, "title", []byte("dek"), nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
			require.NoError(t, err)
			ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))

			// Act
			_, err = recordsService.CreateRecord(ctx, tt.req)

			// Assert
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func newCreateRecordRequest(
	recordType pb.RecordType,
	title string,
	encryptedDEK []byte,
	encryptedPayload []byte,
) *pb.CreateRecordRequest {
	return pb.CreateRecordRequest_builder{
		Type:             &recordType,
		Title:            &title,
		EncryptedDek:     encryptedDEK,
		EncryptedPayload: encryptedPayload,
	}.Build()
}

// TestRecordsService_CreateRecord_FailWithInternalError проверяет маппинг неизвестной ошибки в код ошибки Internal.
func TestRecordsService_CreateRecord_FailWithInternalError(t *testing.T) {
	// Arrange
	uc := &recordsUseCaseStub{createRecordErr: errors.New("some internal error")}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))
	req := newCreateRecordRequest(pb.RecordType_RECORD_TYPE_TEXT, "title", []byte("dek"), []byte("payload"))

	// Act
	_, err = recordsService.CreateRecord(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// TestRecordsService_CreateRecord_FailWithBinaryRecord проверяет, что бинарная приватная запись не создается обычным
// методом.
func TestRecordsService_CreateRecord_FailWithBinaryRecord(t *testing.T) {
	// Arrange
	uc := &recordsUseCaseStub{createRecordErr: usecase.ErrBinaryRecordNotSupported}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))
	req := newCreateRecordRequest(pb.RecordType_RECORD_TYPE_BINARY, "title", []byte("dek"), []byte("payload"))

	// Act
	_, err = recordsService.CreateRecord(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestRecordsService_CreateBinaryRecord_OK проверяет успешное создание бинарной приватной записи через client-stream
// обработчик.
func TestRecordsService_CreateBinaryRecord_OK(t *testing.T) {
	// Arrange
	userID := uuid.Must(uuid.NewV7())
	recordID := uuid.Must(uuid.NewV7())
	uc := &recordsUseCaseStub{createBinaryRecordOutput: usecase.CreateBinaryRecordOutput{
		RecordID:     recordID,
		Version:      1,
		UploadStatus: model.UploadStatusUploaded,
	}}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	stream := newCreateBinaryRecordTestStream(
		authcontext.WithUserID(context.Background(), userID),
		newCreateBinaryRecordMetadata("binary title", int64(len("encrypted-file"))),
		[]byte("encrypted-"),
		[]byte("file"),
	)

	// Act
	err = recordsService.CreateBinaryRecord(stream)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, stream.response)
	assert.Equal(t, recordID.String(), stream.response.GetRecordId())
	assert.Equal(t, int64(1), stream.response.GetVersion())
	assert.Equal(t, pb.UploadStatus_UPLOAD_STATUS_UPLOADED, stream.response.GetUploadStatus())
	assert.Equal(t, userID, uc.createBinaryRecordInput.UserID)
	assert.Equal(t, "binary title", uc.createBinaryRecordInput.Title)
	assert.Equal(t, "description", uc.createBinaryRecordInput.Description)
	assert.Equal(t, []byte("encrypted-dek"), uc.createBinaryRecordInput.EncryptedDEK)
	assert.Equal(t, []byte("encrypted-payload"), uc.createBinaryRecordInput.EncryptedPayload)
	assert.Equal(t, []byte("encrypted-file"), uc.createBinaryRecordFile)
	assert.Equal(t, int64(len("encrypted-file")), uc.createBinaryRecordInput.EncryptedSize)
	assert.Equal(t, model.UploadModeSinglePart, uc.createBinaryRecordInput.UploadMode)
}

// TestRecordsService_CreateBinaryRecord_FailWithInvalidArgument проверяет ошибки валидации stream-запроса.
func TestRecordsService_CreateBinaryRecord_FailWithInvalidArgument(t *testing.T) {
	// Arrange
	userID := uuid.Must(uuid.NewV7())
	tests := []struct {
		name   string
		stream *createBinaryRecordTestStream
	}{
		{
			name:   "empty stream",
			stream: newCreateBinaryRecordTestStream(authcontext.WithUserID(context.Background(), userID), nil),
		},
		{
			name: "first message is chunk",
			stream: newCreateBinaryRecordTestStreamWithRequests(
				authcontext.WithUserID(context.Background(), userID),
				pb.CreateBinaryRecordRequest_builder{Chunk: []byte("chunk")}.Build(),
			),
		},
		{
			name: "blank title",
			stream: newCreateBinaryRecordTestStream(
				authcontext.WithUserID(context.Background(), userID),
				newCreateBinaryRecordMetadata("   ", 1),
				[]byte("a"),
			),
		},
		{
			name: "unexpected metadata after first message",
			stream: newCreateBinaryRecordTestStreamWithRequests(
				authcontext.WithUserID(context.Background(), userID),
				pb.CreateBinaryRecordRequest_builder{
					Metadata: newCreateBinaryRecordMetadata("binary title", 1),
				}.Build(),
				pb.CreateBinaryRecordRequest_builder{
					Metadata: newCreateBinaryRecordMetadata("binary title", 1),
				}.Build(),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
			require.NoError(t, err)

			// Act
			err = recordsService.CreateBinaryRecord(tt.stream)

			// Assert
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

// TestRecordsService_CreateBinaryRecord_FailWithUnauthenticated проверяет ошибку при отсутствии user_id в контексте.
func TestRecordsService_CreateBinaryRecord_FailWithUnauthenticated(t *testing.T) {
	// Arrange
	recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
	require.NoError(t, err)
	stream := newCreateBinaryRecordTestStream(
		context.Background(),
		newCreateBinaryRecordMetadata("binary title", 1),
		[]byte("a"),
	)

	// Act
	err = recordsService.CreateBinaryRecord(stream)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestRecordsService_CreateBinaryRecord_FailWithUseCaseInvalidArgument проверяет маппинг прикладных ошибок валидации.
func TestRecordsService_CreateBinaryRecord_FailWithUseCaseInvalidArgument(t *testing.T) {
	// Arrange
	uc := &recordsUseCaseStub{createBinaryRecordErr: usecase.ErrBinaryEncryptedSizeMismatch}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	stream := newCreateBinaryRecordTestStream(
		authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7())),
		newCreateBinaryRecordMetadata("binary title", 1),
		[]byte("a"),
	)

	// Act
	err = recordsService.CreateBinaryRecord(stream)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestRecordsService_ListRecords_OK проверяет успешное получение списка приватных записей через gRPC-обработчик.
func TestRecordsService_ListRecords_OK(t *testing.T) {
	// Arrange
	userID := uuid.Must(uuid.NewV7())
	recordID := uuid.Must(uuid.NewV7())
	createdAt := time.Date(2026, time.June, 23, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	uc := &recordsUseCaseStub{listRecordsOutput: usecase.ListRecordsOutput{Items: []model.RecordListItem{
		{
			ID:          recordID,
			Type:        model.RecordTypeBinary,
			Title:       "binary title",
			Description: "binary description",
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
			File:        &model.RecordListItemFile{UploadStatus: model.UploadStatusUploading},
		},
	}}}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), userID)
	req := pb.ListRecordsRequest_builder{}.Build()

	// Act
	resp, err := recordsService.ListRecords(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, userID, uc.listRecordsInput.UserID)
	require.Len(t, resp.GetItems(), 1)
	item := resp.GetItems()[0]
	assert.Equal(t, recordID.String(), item.GetRecordId())
	assert.Equal(t, pb.RecordType_RECORD_TYPE_BINARY, item.GetType())
	assert.Equal(t, "binary title", item.GetTitle())
	assert.Equal(t, "binary description", item.GetDescription())
	assert.Equal(t, createdAt, item.GetCreatedAt().AsTime())
	assert.Equal(t, updatedAt, item.GetUpdatedAt().AsTime())
	require.NotNil(t, item.GetFile())
	assert.Equal(t, pb.UploadStatus_UPLOAD_STATUS_UPLOADING, item.GetFile().GetUploadStatus())
}

// TestRecordsService_ListRecords_FailWithUnauthenticated проверяет ошибку при отсутствии идентификатора пользователя
// в контексте.
func TestRecordsService_ListRecords_FailWithUnauthenticated(t *testing.T) {
	// Arrange
	recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
	require.NoError(t, err)
	req := pb.ListRecordsRequest_builder{}.Build()

	// Act
	_, err = recordsService.ListRecords(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestRecordsService_ListRecords_FailWithInvalidArgument проверяет валидацию запроса.
func TestRecordsService_ListRecords_FailWithInvalidArgument(t *testing.T) {
	// Arrange
	recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))

	// Act
	_, err = recordsService.ListRecords(ctx, nil)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestRecordsService_ListRecords_FailWithInternalError проверяет маппинг неизвестной ошибки в код ошибки Internal.
func TestRecordsService_ListRecords_FailWithInternalError(t *testing.T) {
	// Arrange
	uc := &recordsUseCaseStub{listRecordsErr: errors.New("some internal error")}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))
	req := pb.ListRecordsRequest_builder{}.Build()

	// Act
	_, err = recordsService.ListRecords(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// TestRecordsService_GetRecord_OK проверяет успешное получение приватной записи через gRPC-обработчик.
func TestRecordsService_GetRecord_OK(t *testing.T) {
	// Arrange
	userID := uuid.Must(uuid.NewV7())
	recordID := uuid.Must(uuid.NewV7())
	createdAt := time.Date(2026, time.June, 23, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	uc := &recordsUseCaseStub{getRecordOutput: usecase.GetRecordOutput{Record: model.Record{
		ID:               recordID,
		UserID:           userID,
		Type:             model.RecordTypeText,
		Title:            "text title",
		Description:      "text description",
		EncryptedDEK:     model.EncryptedBlob{Data: []byte("encrypted-dek")},
		EncryptedPayload: model.EncryptedBlob{Data: []byte("encrypted-payload")},
		Version:          2,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}}}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), userID)
	req := pb.GetRecordRequest_builder{RecordId: new(recordID.String())}.Build()

	// Act
	resp, err := recordsService.GetRecord(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, userID, uc.getRecordInput.UserID)
	assert.Equal(t, recordID, uc.getRecordInput.RecordID)
	record := resp.GetRecord()
	require.NotNil(t, record)
	assert.Equal(t, recordID.String(), record.GetRecordId())
	assert.Equal(t, pb.RecordType_RECORD_TYPE_TEXT, record.GetType())
	assert.Equal(t, "text title", record.GetTitle())
	assert.Equal(t, "text description", record.GetDescription())
	assert.Equal(t, []byte("encrypted-dek"), record.GetEncryptedDek())
	assert.Equal(t, []byte("encrypted-payload"), record.GetEncryptedPayload())
	assert.Equal(t, int64(2), record.GetVersion())
	assert.Equal(t, createdAt, record.GetCreatedAt().AsTime())
	assert.Equal(t, updatedAt, record.GetUpdatedAt().AsTime())
	assert.Nil(t, record.GetDeletedAt())
	assert.Nil(t, record.GetFile())
}

// TestRecordsService_GetRecord_FailWithUnauthenticated проверяет ошибку при отсутствии идентификатора пользователя
// в контексте.
func TestRecordsService_GetRecord_FailWithUnauthenticated(t *testing.T) {
	// Arrange
	recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
	require.NoError(t, err)
	req := pb.GetRecordRequest_builder{RecordId: new(uuid.Must(uuid.NewV7()).String())}.Build()

	// Act
	_, err = recordsService.GetRecord(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestRecordsService_GetRecord_FailWithInvalidArgument проверяет валидацию запроса.
func TestRecordsService_GetRecord_FailWithInvalidArgument(t *testing.T) {
	// Arrange
	tests := []struct {
		name string
		req  *pb.GetRecordRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty record id", req: pb.GetRecordRequest_builder{RecordId: new("")}.Build()},
		{name: "invalid record id", req: pb.GetRecordRequest_builder{RecordId: new("not-a-uuid")}.Build()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
			require.NoError(t, err)
			ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))

			// Act
			_, err = recordsService.GetRecord(ctx, tt.req)

			// Assert
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

// TestRecordsService_GetRecord_FailWithNotFound проверяет маппинг отсутствующей приватной записи в код ошибки NotFound.
func TestRecordsService_GetRecord_FailWithNotFound(t *testing.T) {
	// Arrange
	uc := &recordsUseCaseStub{getRecordErr: usecase.ErrRecordNotFound}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))
	req := pb.GetRecordRequest_builder{RecordId: new(uuid.Must(uuid.NewV7()).String())}.Build()

	// Act
	_, err = recordsService.GetRecord(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// TestRecordsService_GetRecord_FailWithInternalError проверяет маппинг неизвестной ошибки в код ошибки Internal.
func TestRecordsService_GetRecord_FailWithInternalError(t *testing.T) {
	// Arrange
	uc := &recordsUseCaseStub{getRecordErr: errors.New("some internal error")}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))
	req := pb.GetRecordRequest_builder{RecordId: new(uuid.Must(uuid.NewV7()).String())}.Build()

	// Act
	_, err = recordsService.GetRecord(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// TestRecordsService_DownloadFile_OK проверяет успешное скачивание файла чанками через gRPC-обработчик.
func TestRecordsService_DownloadFile_OK(t *testing.T) {
	// Arrange
	userID := uuid.Must(uuid.NewV7())
	recordID := uuid.Must(uuid.NewV7())
	encryptedFile := bytes.Repeat([]byte("a"), downloadChunkSize+10)
	uc := &recordsUseCaseStub{downloadFileOutput: usecase.DownloadFileOutput{
		EncryptedFile: io.NopCloser(bytes.NewReader(encryptedFile)),
	}}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	stream := newDownloadFileTestStream(authcontext.WithUserID(context.Background(), userID))
	req := pb.DownloadFileRequest_builder{RecordId: new(recordID.String())}.Build()

	// Act
	err = recordsService.DownloadFile(req, stream)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, userID, uc.downloadFileInput.UserID)
	assert.Equal(t, recordID, uc.downloadFileInput.RecordID)
	require.Len(t, stream.chunks, 2)
	assert.Equal(t, downloadChunkSize, len(stream.chunks[0]))
	assert.Equal(t, encryptedFile, bytes.Join(stream.chunks, nil))
}

// TestRecordsService_DownloadFile_FailWithUnauthenticated проверяет ошибку при отсутствии идентификатора пользователя
// в контексте.
func TestRecordsService_DownloadFile_FailWithUnauthenticated(t *testing.T) {
	// Arrange
	recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
	require.NoError(t, err)
	stream := newDownloadFileTestStream(context.Background())
	req := pb.DownloadFileRequest_builder{RecordId: new(uuid.Must(uuid.NewV7()).String())}.Build()

	// Act
	err = recordsService.DownloadFile(req, stream)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestRecordsService_DownloadFile_FailWithInvalidArgument проверяет валидацию запроса на скачивание файла.
func TestRecordsService_DownloadFile_FailWithInvalidArgument(t *testing.T) {
	// Arrange
	tests := []struct {
		name string
		req  *pb.DownloadFileRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty record id", req: pb.DownloadFileRequest_builder{RecordId: new("")}.Build()},
		{name: "invalid record id", req: pb.DownloadFileRequest_builder{RecordId: new("not-a-uuid")}.Build()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
			require.NoError(t, err)
			stream := newDownloadFileTestStream(authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7())))

			// Act
			err = recordsService.DownloadFile(tt.req, stream)

			// Assert
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

// TestRecordsService_DownloadFile_FailWithNotFound проверяет маппинг отсутствующей приватной записи в код ошибки
// NotFound.
func TestRecordsService_DownloadFile_FailWithNotFound(t *testing.T) {
	// Arrange
	uc := &recordsUseCaseStub{downloadFileErr: usecase.ErrRecordNotFound}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	stream := newDownloadFileTestStream(authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7())))
	req := pb.DownloadFileRequest_builder{RecordId: new(uuid.Must(uuid.NewV7()).String())}.Build()

	// Act
	err = recordsService.DownloadFile(req, stream)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// TestRecordsService_DownloadFile_FailWithUseCaseError проверяет маппинг прикладных ошибок.
func TestRecordsService_DownloadFile_FailWithUseCaseError(t *testing.T) {
	// Arrange
	tests := []struct {
		name       string
		usecaseErr error
		code       codes.Code
	}{
		{name: "record is not binary", usecaseErr: usecase.ErrRecordIsNotBinary, code: codes.InvalidArgument},
		{name: "file is not uploaded", usecaseErr: usecase.ErrRecordFileIsNotUploaded, code: codes.FailedPrecondition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			uc := &recordsUseCaseStub{downloadFileErr: tt.usecaseErr}
			recordsService, err := NewRecordsService(uc, logging.NopLogger())
			require.NoError(t, err)
			stream := newDownloadFileTestStream(authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7())))
			req := pb.DownloadFileRequest_builder{RecordId: new(uuid.Must(uuid.NewV7()).String())}.Build()

			// Act
			err = recordsService.DownloadFile(req, stream)

			// Assert
			require.Error(t, err)
			assert.Equal(t, tt.code, status.Code(err))
		})
	}
}

// TestRecordsService_DownloadFile_FailWithInternalError проверяет маппинг неизвестной ошибки в код ошибки Internal.
func TestRecordsService_DownloadFile_FailWithInternalError(t *testing.T) {
	// Arrange
	uc := &recordsUseCaseStub{downloadFileErr: errors.New("some internal error")}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	stream := newDownloadFileTestStream(authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7())))
	req := pb.DownloadFileRequest_builder{RecordId: new(uuid.Must(uuid.NewV7()).String())}.Build()

	// Act
	err = recordsService.DownloadFile(req, stream)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

type createBinaryRecordTestStream struct {
	ctx      context.Context
	requests []*pb.CreateBinaryRecordRequest
	response *pb.CreateBinaryRecordResponse
}

func newCreateBinaryRecordTestStream(
	ctx context.Context,
	metadata *pb.CreateBinaryRecordMetadata,
	chunks ...[]byte,
) *createBinaryRecordTestStream {
	requests := make([]*pb.CreateBinaryRecordRequest, 0, len(chunks)+1)
	if metadata != nil {
		requests = append(requests, pb.CreateBinaryRecordRequest_builder{Metadata: metadata}.Build())
	}
	for _, chunk := range chunks {
		requests = append(requests, pb.CreateBinaryRecordRequest_builder{Chunk: chunk}.Build())
	}
	return newCreateBinaryRecordTestStreamWithRequests(ctx, requests...)
}

func newCreateBinaryRecordTestStreamWithRequests(
	ctx context.Context,
	requests ...*pb.CreateBinaryRecordRequest,
) *createBinaryRecordTestStream {
	return &createBinaryRecordTestStream{ctx: ctx, requests: requests}
}

func newCreateBinaryRecordMetadata(title string, encryptedSize int64) *pb.CreateBinaryRecordMetadata {
	uploadMode := pb.UploadMode_UPLOAD_MODE_SINGLE_PART
	return pb.CreateBinaryRecordMetadata_builder{
		Title:            &title,
		Description:      new("description"),
		EncryptedDek:     []byte("encrypted-dek"),
		EncryptedPayload: []byte("encrypted-payload"),
		EncryptedSize:    &encryptedSize,
		UploadMode:       &uploadMode,
	}.Build()
}

func (s *createBinaryRecordTestStream) Recv() (*pb.CreateBinaryRecordRequest, error) {
	if len(s.requests) == 0 {
		return nil, io.EOF
	}
	req := s.requests[0]
	s.requests = s.requests[1:]
	return req, nil
}

func (s *createBinaryRecordTestStream) SendAndClose(resp *pb.CreateBinaryRecordResponse) error {
	s.response = resp
	return nil
}

func (s *createBinaryRecordTestStream) SetHeader(metadata.MD) error {
	return nil
}

func (s *createBinaryRecordTestStream) SendHeader(metadata.MD) error {
	return nil
}

func (s *createBinaryRecordTestStream) SetTrailer(metadata.MD) {}

func (s *createBinaryRecordTestStream) Context() context.Context {
	return s.ctx
}

func (s *createBinaryRecordTestStream) SendMsg(any) error {
	return nil
}

func (s *createBinaryRecordTestStream) RecvMsg(any) error {
	return nil
}

type downloadFileTestStream struct {
	ctx    context.Context
	chunks [][]byte
}

func newDownloadFileTestStream(ctx context.Context) *downloadFileTestStream {
	return &downloadFileTestStream{ctx: ctx}
}

func (s *downloadFileTestStream) Send(resp *pb.DownloadFileResponse) error {
	s.chunks = append(s.chunks, append([]byte(nil), resp.GetChunk()...))
	return nil
}

func (s *downloadFileTestStream) SetHeader(metadata.MD) error {
	return nil
}

func (s *downloadFileTestStream) SendHeader(metadata.MD) error {
	return nil
}

func (s *downloadFileTestStream) SetTrailer(metadata.MD) {}

func (s *downloadFileTestStream) Context() context.Context {
	return s.ctx
}

func (s *downloadFileTestStream) SendMsg(any) error {
	return nil
}

func (s *downloadFileTestStream) RecvMsg(any) error {
	return nil
}
