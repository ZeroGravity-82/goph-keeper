package service

import (
	"context"
	"errors"
	"testing"

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
}

func (s *recordsUseCaseStub) CreateRecord(
	_ context.Context,
	in usecase.CreateRecordInput,
) (usecase.CreateRecordOutput, error) {
	s.createRecordInput = in
	return s.createRecordOutput, s.createRecordErr
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
