package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/logging"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/transport/grpcserver/authcontext"
	"zerogravity-82/goph-keeper/internal/usecase"
)

type recordsUseCaseStub struct {
	createRecordInput  usecase.CreateRecordInput
	createRecordOutput usecase.CreateRecordOutput
	createRecordErr    error
	listRecordsInput   usecase.ListRecordsInput
	listRecordsOutput  usecase.ListRecordsOutput
	listRecordsErr     error
}

func (s *recordsUseCaseStub) CreateRecord(
	_ context.Context,
	in usecase.CreateRecordInput,
) (usecase.CreateRecordOutput, error) {
	s.createRecordInput = in
	return s.createRecordOutput, s.createRecordErr
}

func (s *recordsUseCaseStub) ListRecords(
	_ context.Context,
	in usecase.ListRecordsInput,
) (usecase.ListRecordsOutput, error) {
	s.listRecordsInput = in
	return s.listRecordsOutput, s.listRecordsErr
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
			File:        &model.RecordFileListItem{UploadStatus: model.UploadStatusPending},
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
	assert.Equal(t, pb.UploadStatus_UPLOAD_STATUS_PENDING, item.GetFile().GetUploadStatus())
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
