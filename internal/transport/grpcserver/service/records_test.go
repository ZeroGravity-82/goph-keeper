package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
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

// TestRecordsService_CreateRecord_OK проверяет успешное создание приватной записи через gRPC-обработчик.
func TestRecordsService_CreateRecord_OK(t *testing.T) {
	// Arrange
	userID := uuid.Must(uuid.NewV7())
	recordID := uuid.Must(uuid.NewV7())
	uc := &recordsUseCaseStub{createRecordOutput: usecase.CreateRecordOutput{RecordID: recordID, Version: 1}}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	req := pb.CreateRecordRequest_builder{
		Type:             new(pb.RecordType_RECORD_TYPE_TEXT),
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

// TestRecordsService_CreateRecord_OKWithMaxSizeData проверяет, что граничные размеры данных проходят валидацию
// gRPC-запроса.
func TestRecordsService_CreateRecord_OKWithMaxSizeData(t *testing.T) {
	// Arrange
	userID := uuid.Must(uuid.NewV7())
	recordID := uuid.Must(uuid.NewV7())
	uc := &recordsUseCaseStub{createRecordOutput: usecase.CreateRecordOutput{RecordID: recordID, Version: 1}}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	title := strings.Repeat("я", recordTitleMaxSizeChars)
	description := strings.Repeat("ю", recordDescriptionMaxSizeChars)
	req := pb.CreateRecordRequest_builder{
		Type:             new(pb.RecordType_RECORD_TYPE_TEXT),
		Title:            &title,
		Description:      &description,
		EncryptedDek:     make([]byte, recordEncryptedDEKMaxSizeBytes),
		EncryptedPayload: make([]byte, recordEncryptedPayloadMaxSizeBytes),
	}.Build()
	ctx := authcontext.WithUserID(context.Background(), userID)

	// Act
	_, err = recordsService.CreateRecord(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, title, uc.createRecordInput.Title)
	assert.Equal(t, description, uc.createRecordInput.Description)
	assert.Len(t, uc.createRecordInput.EncryptedDEK, recordEncryptedDEKMaxSizeBytes)
	assert.Len(t, uc.createRecordInput.EncryptedPayload, recordEncryptedPayloadMaxSizeBytes)
}

// TestRecordsService_CreateRecord_FailWithUnauthenticated проверяет ошибку при отсутствии идентификатора пользователя
// в контексте.
func TestRecordsService_CreateRecord_FailWithUnauthenticated(t *testing.T) {
	// Arrange
	recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
	require.NoError(t, err)
	req := pb.CreateRecordRequest_builder{
		Type:             new(pb.RecordType_RECORD_TYPE_TEXT),
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
		name        string
		req         *pb.CreateRecordRequest
		wantMessage string
	}{
		{name: "nil request", req: nil, wantMessage: "request is required"},
		{name: "unspecified type", req: newCreateRecordRequest(
			pb.RecordType_RECORD_TYPE_UNSPECIFIED,
			"title",
			[]byte("dek"),
			[]byte("payload")),
			wantMessage: "record type is invalid",
		},
		{
			name:        "empty title",
			req:         newCreateRecordRequest(textType, "", []byte("dek"), []byte("payload")),
			wantMessage: "title is required",
		},
		{
			name:        "blank title",
			req:         newCreateRecordRequest(textType, "   ", []byte("dek"), []byte("payload")),
			wantMessage: "title is required",
		},
		{
			name:        "empty encrypted dek",
			req:         newCreateRecordRequest(textType, "title", nil, []byte("payload")),
			wantMessage: "encrypted dek is required",
		},
		{
			name:        "empty encrypted payload",
			req:         newCreateRecordRequest(textType, "title", []byte("dek"), nil),
			wantMessage: "encrypted payload is required",
		},
		{
			name: "long title",
			req: newCreateRecordRequest(
				textType,
				strings.Repeat("a", recordTitleMaxSizeChars+1),
				[]byte("dek"),
				[]byte("payload"),
			),
			wantMessage: "title exceeds size limit",
		},
		{
			name: "long description",
			req: newCreateRecordRequestWithDescription(
				textType,
				"title",
				strings.Repeat("a", recordDescriptionMaxSizeChars+1),
				[]byte("dek"),
				[]byte("payload"),
			),
			wantMessage: "description exceeds size limit",
		},
		{
			name: "large encrypted dek",
			req: newCreateRecordRequest(
				textType,
				"title",
				make([]byte, recordEncryptedDEKMaxSizeBytes+1),
				[]byte("payload"),
			),
			wantMessage: "encrypted dek exceeds size limit",
		},
		{
			name: "large encrypted payload",
			req: newCreateRecordRequest(
				textType,
				"title",
				[]byte("dek"),
				make([]byte, recordEncryptedPayloadMaxSizeBytes+1),
			),
			wantMessage: "encrypted payload exceeds size limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			uc := &recordsUseCaseStub{}
			recordsService, err := NewRecordsService(uc, logging.NopLogger())
			require.NoError(t, err)
			ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))

			// Act
			_, err = recordsService.CreateRecord(ctx, tt.req)

			// Assert
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
			assert.Equal(t, tt.wantMessage, status.Convert(err).Message())
			assert.Empty(t, uc.createRecordInput)
		})
	}
}

func newCreateRecordRequest(
	recordType pb.RecordType,
	title string,
	encryptedDEK []byte,
	encryptedPayload []byte,
) *pb.CreateRecordRequest {
	return newCreateRecordRequestWithDescription(recordType, title, "", encryptedDEK, encryptedPayload)
}

func newCreateRecordRequestWithDescription(
	recordType pb.RecordType,
	title string,
	description string,
	encryptedDEK []byte,
	encryptedPayload []byte,
) *pb.CreateRecordRequest {
	return pb.CreateRecordRequest_builder{
		Type:             &recordType,
		Title:            &title,
		Description:      &description,
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

// TestRecordsService_UpdateRecord_OK проверяет успешное обновление приватной записи через gRPC-обработчик.
func TestRecordsService_UpdateRecord_OK(t *testing.T) {
	// Arrange
	userID := uuid.Must(uuid.NewV7())
	recordID := uuid.Must(uuid.NewV7())
	uc := &recordsUseCaseStub{updateRecordOutput: usecase.UpdateRecordOutput{RecordID: recordID, Version: 2}}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), userID)
	req := newUpdateRecordRequest(recordID.String(), "new title", []byte("new-dek"), []byte("new-payload"), 1)

	// Act
	resp, err := recordsService.UpdateRecord(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, recordID.String(), resp.GetRecordId())
	assert.Equal(t, int64(2), resp.GetVersion())
	assert.Equal(t, recordID, uc.updateRecordInput.RecordID)
	assert.Equal(t, userID, uc.updateRecordInput.UserID)
	assert.Equal(t, "new title", uc.updateRecordInput.Title)
	assert.Equal(t, "description", uc.updateRecordInput.Description)
	assert.Equal(t, []byte("new-dek"), uc.updateRecordInput.EncryptedDEK)
	assert.Equal(t, []byte("new-payload"), uc.updateRecordInput.EncryptedPayload)
	assert.Equal(t, int64(1), uc.updateRecordInput.ExpectedVersion)
}

// TestRecordsService_UpdateRecord_FailWithUnauthenticated проверяет ошибку при отсутствии идентификатора пользователя
// в контексте.
func TestRecordsService_UpdateRecord_FailWithUnauthenticated(t *testing.T) {
	// Arrange
	recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
	require.NoError(t, err)
	req := newUpdateRecordRequest(uuid.Must(uuid.NewV7()).String(), "title", []byte("dek"), []byte("payload"), 1)

	// Act
	_, err = recordsService.UpdateRecord(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestRecordsService_UpdateRecord_FailWithInvalidArgument проверяет валидацию запроса на обновление приватной записи.
func TestRecordsService_UpdateRecord_FailWithInvalidArgument(t *testing.T) {
	// Arrange
	recordID := uuid.Must(uuid.NewV7()).String()
	tests := []struct {
		name string
		req  *pb.UpdateRecordRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty record id", req: newUpdateRecordRequest("", "title", []byte("dek"), []byte("payload"), 1)},
		{
			name: "invalid record id",
			req:  newUpdateRecordRequest("not-a-uuid", "title", []byte("dek"), []byte("payload"), 1),
		},
		{name: "empty title", req: newUpdateRecordRequest(recordID, "", []byte("dek"), []byte("payload"), 1)},
		{name: "blank title", req: newUpdateRecordRequest(recordID, "   ", []byte("dek"), []byte("payload"), 1)},
		{name: "empty encrypted dek", req: newUpdateRecordRequest(recordID, "title", nil, []byte("payload"), 1)},
		{name: "empty encrypted payload", req: newUpdateRecordRequest(recordID, "title", []byte("dek"), nil, 1)},
		{
			name: "zero expected version",
			req:  newUpdateRecordRequest(recordID, "title", []byte("dek"), []byte("payload"), 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
			require.NoError(t, err)
			ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))

			// Act
			_, err = recordsService.UpdateRecord(ctx, tt.req)

			// Assert
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func newUpdateRecordRequest(
	recordID string,
	title string,
	encryptedDEK []byte,
	encryptedPayload []byte,
	expectedVersion int64,
) *pb.UpdateRecordRequest {
	return pb.UpdateRecordRequest_builder{
		RecordId:         &recordID,
		Title:            &title,
		Description:      new("description"),
		EncryptedDek:     encryptedDEK,
		EncryptedPayload: encryptedPayload,
		ExpectedVersion:  &expectedVersion,
	}.Build()
}

// TestRecordsService_UpdateRecord_FailWithNotFound проверяет маппинг отсутствующей приватной записи в код ошибки
// NotFound.
func TestRecordsService_UpdateRecord_FailWithNotFound(t *testing.T) {
	// Arrange
	uc := &recordsUseCaseStub{updateRecordErr: usecase.ErrRecordNotFound}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))
	req := newUpdateRecordRequest(uuid.Must(uuid.NewV7()).String(), "title", []byte("dek"), []byte("payload"), 1)

	// Act
	_, err = recordsService.UpdateRecord(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// TestRecordsService_UpdateRecord_FailWithVersionConflict проверяет маппинг конфликта версии в код ошибки Aborted.
func TestRecordsService_UpdateRecord_FailWithVersionConflict(t *testing.T) {
	// Arrange
	uc := &recordsUseCaseStub{updateRecordErr: usecase.ErrRecordVersionConflict}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))
	req := newUpdateRecordRequest(uuid.Must(uuid.NewV7()).String(), "title", []byte("dek"), []byte("payload"), 1)

	// Act
	_, err = recordsService.UpdateRecord(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Aborted, status.Code(err))
}

// TestRecordsService_UpdateRecord_FailWithInternalError проверяет маппинг неизвестной ошибки в код ошибки Internal.
func TestRecordsService_UpdateRecord_FailWithInternalError(t *testing.T) {
	// Arrange
	uc := &recordsUseCaseStub{updateRecordErr: errors.New("some internal error")}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))
	req := newUpdateRecordRequest(uuid.Must(uuid.NewV7()).String(), "title", []byte("dek"), []byte("payload"), 1)

	// Act
	_, err = recordsService.UpdateRecord(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// TestRecordsService_DeleteRecord_OK проверяет успешное удаление приватной записи через gRPC-обработчик.
func TestRecordsService_DeleteRecord_OK(t *testing.T) {
	// Arrange
	userID := uuid.Must(uuid.NewV7())
	recordID := uuid.Must(uuid.NewV7())
	uc := &recordsUseCaseStub{deleteRecordOutput: usecase.DeleteRecordOutput{RecordID: recordID}}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), userID)
	req := pb.DeleteRecordRequest_builder{RecordId: new(recordID.String())}.Build()

	// Act
	resp, err := recordsService.DeleteRecord(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, recordID.String(), resp.GetRecordId())
	assert.Equal(t, recordID, uc.deleteRecordInput.RecordID)
	assert.Equal(t, userID, uc.deleteRecordInput.UserID)
}

// TestRecordsService_DeleteRecord_FailWithUnauthenticated проверяет ошибку при отсутствии идентификатора пользователя
// в контексте.
func TestRecordsService_DeleteRecord_FailWithUnauthenticated(t *testing.T) {
	// Arrange
	recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
	require.NoError(t, err)
	req := pb.DeleteRecordRequest_builder{RecordId: new(uuid.Must(uuid.NewV7()).String())}.Build()

	// Act
	_, err = recordsService.DeleteRecord(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestRecordsService_DeleteRecord_FailWithInvalidArgument проверяет валидацию запроса на удаление приватной записи.
func TestRecordsService_DeleteRecord_FailWithInvalidArgument(t *testing.T) {
	// Arrange
	tests := []struct {
		name string
		req  *pb.DeleteRecordRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty record id", req: pb.DeleteRecordRequest_builder{RecordId: new("")}.Build()},
		{name: "invalid record id", req: pb.DeleteRecordRequest_builder{RecordId: new("not-a-uuid")}.Build()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
			require.NoError(t, err)
			ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))

			// Act
			_, err = recordsService.DeleteRecord(ctx, tt.req)

			// Assert
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

// TestRecordsService_DeleteRecord_FailWithNotFound проверяет маппинг отсутствующей записи в код ошибки NotFound.
func TestRecordsService_DeleteRecord_FailWithNotFound(t *testing.T) {
	// Arrange
	uc := &recordsUseCaseStub{deleteRecordErr: usecase.ErrRecordNotFound}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))
	req := pb.DeleteRecordRequest_builder{RecordId: new(uuid.Must(uuid.NewV7()).String())}.Build()

	// Act
	_, err = recordsService.DeleteRecord(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// TestRecordsService_DeleteRecord_FailWithInternalError проверяет маппинг неизвестной ошибки в код ошибки Internal.
func TestRecordsService_DeleteRecord_FailWithInternalError(t *testing.T) {
	// Arrange
	uc := &recordsUseCaseStub{deleteRecordErr: errors.New("some internal error")}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))
	req := pb.DeleteRecordRequest_builder{RecordId: new(uuid.Must(uuid.NewV7()).String())}.Build()

	// Act
	_, err = recordsService.DeleteRecord(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// TestRecordsService_DownloadFile_OK проверяет успешное скачивание файла чанками через gRPC-обработчик.
func TestRecordsService_DownloadFile_OK(t *testing.T) {
	// Arrange
	userID := uuid.Must(uuid.NewV7())
	recordID := uuid.Must(uuid.NewV7())
	encryptedFile := bytes.Repeat([]byte("a"), downloadChunkSizeBytes+10)
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
	assert.Equal(t, downloadChunkSizeBytes, len(stream.chunks[0]))
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

// TestRecordsService_CompleteBinaryMultipartUpload_OK проверяет завершение multipart-загрузки с передачей контрольной
// суммы зашифрованного файла в сценарий.
func TestRecordsService_CompleteBinaryMultipartUpload_OK(t *testing.T) {
	// Arrange
	userID := uuid.Must(uuid.NewV7())
	uploadID := uuid.Must(uuid.NewV7())
	recordID := uuid.Must(uuid.NewV7())
	encryptedSHA256 := strings.Repeat("A", sha256HexSizeChars)
	normalizedSHA256 := strings.ToLower(encryptedSHA256)
	uc := &recordsUseCaseStub{
		completeMultipartOutput: usecase.CompleteBinaryMultipartUploadOutput{
			RecordID:        recordID,
			Version:         1,
			UploadStatus:    model.UploadStatusUploaded,
			EncryptedSHA256: normalizedSHA256,
		},
	}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), userID)
	req := pb.CompleteBinaryMultipartUploadRequest_builder{
		UploadId:        new(uploadID.String()),
		EncryptedSha256: &encryptedSHA256,
	}.Build()

	// Act
	resp, err := recordsService.CompleteBinaryMultipartUpload(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, userID, uc.completeMultipartInput.UserID)
	assert.Equal(t, uploadID, uc.completeMultipartInput.UploadID)
	assert.Equal(t, normalizedSHA256, uc.completeMultipartInput.EncryptedSHA256)
	assert.Equal(t, recordID.String(), resp.GetRecordId())
	assert.Equal(t, normalizedSHA256, resp.GetEncryptedSha256())
}

// TestRecordsService_CompleteBinaryMultipartUpload_FailWithInvalidArgument проверяет валидацию запроса завершения
// multipart-загрузки.
func TestRecordsService_CompleteBinaryMultipartUpload_FailWithInvalidArgument(t *testing.T) {
	// Arrange
	uploadID := uuid.Must(uuid.NewV7()).String()
	tests := []struct {
		name string
		req  *pb.CompleteBinaryMultipartUploadRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty upload id", req: pb.CompleteBinaryMultipartUploadRequest_builder{
			UploadId:        new(""),
			EncryptedSha256: new(strings.Repeat("a", sha256HexSizeChars)),
		}.Build()},
		{name: "invalid upload id", req: pb.CompleteBinaryMultipartUploadRequest_builder{
			UploadId:        new("not-a-uuid"),
			EncryptedSha256: new(strings.Repeat("a", sha256HexSizeChars)),
		}.Build()},
		{name: "empty sha256", req: pb.CompleteBinaryMultipartUploadRequest_builder{
			UploadId:        &uploadID,
			EncryptedSha256: new(""),
		}.Build()},
		{name: "short sha256", req: pb.CompleteBinaryMultipartUploadRequest_builder{
			UploadId:        &uploadID,
			EncryptedSha256: new(strings.Repeat("a", sha256HexSizeChars-1)),
		}.Build()},
		{name: "non hex sha256", req: pb.CompleteBinaryMultipartUploadRequest_builder{
			UploadId:        &uploadID,
			EncryptedSha256: new(strings.Repeat("z", sha256HexSizeChars)),
		}.Build()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			recordsService, err := NewRecordsService(&recordsUseCaseStub{}, logging.NopLogger())
			require.NoError(t, err)
			ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))

			// Act
			_, err = recordsService.CompleteBinaryMultipartUpload(ctx, tt.req)

			// Assert
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

// TestRecordsService_CompleteBinaryMultipartUpload_FailWithChecksumMismatch проверяет маппинг ошибки контрольной суммы
// в код ошибки DataLoss.
func TestRecordsService_CompleteBinaryMultipartUpload_FailWithChecksumMismatch(t *testing.T) {
	// Arrange
	uc := &recordsUseCaseStub{completeMultipartErr: usecase.ErrMultipartUploadChecksumMismatch}
	recordsService, err := NewRecordsService(uc, logging.NopLogger())
	require.NoError(t, err)
	ctx := authcontext.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))
	uploadID := uuid.Must(uuid.NewV7()).String()
	encryptedSHA256 := strings.Repeat("a", sha256HexSizeChars)
	req := pb.CompleteBinaryMultipartUploadRequest_builder{
		UploadId:        &uploadID,
		EncryptedSha256: &encryptedSHA256,
	}.Build()

	// Act
	_, err = recordsService.CompleteBinaryMultipartUpload(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.DataLoss, status.Code(err))
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
