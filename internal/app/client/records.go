package client

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/pb"
)

// CreateRecordOutput содержит результат создания приватной записи.
type CreateRecordOutput struct {
	RecordID string
	Version  int64
}

// UpdateRecordOutput содержит результат обновления приватной записи.
type UpdateRecordOutput struct {
	RecordID string
	Version  int64
}

// DeleteRecordOutput содержит результат удаления приватной записи.
type DeleteRecordOutput struct {
	RecordID string
}

// RecordListItem содержит краткую информацию о приватной записи.
type RecordListItem struct {
	RecordID    string
	Type        string
	Title       string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// recordMutationErrorMessages содержит пользовательские сообщения для создания и обновления обычных приватных записей.
var recordMutationErrorMessages = map[codes.Code]string{
	codes.Unauthenticated:   "сессия недействительна, войдите снова",
	codes.InvalidArgument:   "некорректные данные приватной записи",
	codes.NotFound:          "приватная запись не найдена",
	codes.Aborted:           "приватная запись была изменена с другого клиента, получите актуальную версию",
	codes.ResourceExhausted: "размер данных приватной записи превышает допустимый лимит",
}

// binaryRecordMutationErrorMessages содержит пользовательские сообщения для создания и замены бинарных приватных
// записей.
var binaryRecordMutationErrorMessages = map[codes.Code]string{
	codes.Unauthenticated:    "сессия недействительна, войдите снова",
	codes.InvalidArgument:    "некорректные данные бинарной приватной записи",
	codes.NotFound:           "приватная запись не найдена",
	codes.Aborted:            "приватная запись была изменена с другого клиента, получите актуальную версию",
	codes.FailedPrecondition: "файл не может быть загружен в текущем состоянии",
	codes.ResourceExhausted:  "размер данных бинарной приватной записи превышает допустимый лимит",
	codes.DeadlineExceeded:   "истекло время ожидания загрузки файла",
	codes.Canceled:           "загрузка файла отменена",
	codes.Unavailable:        "сервер временно недоступен",
	codes.PermissionDenied:   "доступ запрещен",
}

// rawRecord содержит минимальный набор полей приватной записи, нужный для переупаковки DEK при смене мастер-ключа.
type rawRecord struct {
	RecordID     string
	Version      int64
	EncryptedDEK []byte
}

// ListRecords возвращает список приватных записей пользователя.
func (a *App) ListRecords(ctx context.Context) ([]RecordListItem, error) {
	var resp *pb.ListRecordsResponse
	err := a.withAccessTokenRefreshRetry(ctx, func(ctx context.Context) error {
		var err error
		resp, err = a.records.ListRecords(ctx, pb.ListRecordsRequest_builder{}.Build())
		return err
	})
	if err != nil {
		return nil, rpcError(err, "не удалось получить список приватных записей", map[codes.Code]string{
			codes.Unauthenticated: "сессия недействительна, войдите снова",
		})
	}

	items := make([]RecordListItem, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, RecordListItem{
			RecordID:    item.GetRecordId(),
			Type:        recordTypeString(item.GetType()),
			Title:       item.GetTitle(),
			Description: item.GetDescription(),
			CreatedAt:   timestampAsTime(item.GetCreatedAt()),
			UpdatedAt:   timestampAsTime(item.GetUpdatedAt()),
		})
	}
	return items, nil
}

func (a *App) getRawRecord(ctx context.Context, recordID string) (rawRecord, error) {
	var resp *pb.GetRecordResponse
	err := a.withAccessTokenRefreshRetry(ctx, func(ctx context.Context) error {
		var err error
		resp, err = a.records.GetRecord(ctx, pb.GetRecordRequest_builder{RecordId: &recordID}.Build())
		return err
	})
	if err != nil {
		return rawRecord{}, rpcError(
			err,
			"не удалось получить приватную запись",
			map[codes.Code]string{
				codes.Unauthenticated: "сессия недействительна, войдите снова",
				codes.InvalidArgument: "некорректный идентификатор приватной записи",
				codes.NotFound:        "приватная запись не найдена",
			},
		)
	}
	record := resp.GetRecord()
	if record == nil {
		return rawRecord{}, errors.New("сервер вернул пустую приватную запись")
	}
	return rawRecord{
		RecordID:     record.GetRecordId(),
		Version:      record.GetVersion(),
		EncryptedDEK: record.GetEncryptedDek(),
	}, nil
}

func recordTypeString(recordType pb.RecordType) string {
	switch recordType {
	case pb.RecordType_RECORD_TYPE_CREDENTIAL:
		return string(model.RecordTypeCredential)
	case pb.RecordType_RECORD_TYPE_TEXT:
		return string(model.RecordTypeText)
	case pb.RecordType_RECORD_TYPE_CARD:
		return string(model.RecordTypeCard)
	case pb.RecordType_RECORD_TYPE_BINARY:
		return string(model.RecordTypeBinary)
	default:
		return "unknown"
	}
}

func (a *App) updateRecord(
	ctx context.Context,
	recordID string,
	title string,
	description string,
	encryptedDEK []byte,
	encryptedPayload []byte,
	expectedVersion int64,
) (UpdateRecordOutput, error) {
	if err := validateRecordMetadataSize(title, description); err != nil {
		return UpdateRecordOutput{}, err
	}

	var resp *pb.UpdateRecordResponse
	err := a.withAccessTokenRefresh(ctx, func(ctx context.Context) error {
		var err error
		resp, err = a.records.UpdateRecord(ctx, pb.UpdateRecordRequest_builder{
			RecordId:         &recordID,
			Title:            &title,
			Description:      &description,
			EncryptedDek:     encryptedDEK,
			EncryptedPayload: encryptedPayload,
			ExpectedVersion:  &expectedVersion,
		}.Build())
		return err
	})
	if err != nil {
		return UpdateRecordOutput{}, rpcError(
			err,
			"не удалось обновить приватную запись",
			recordMutationErrorMessages,
		)
	}

	return UpdateRecordOutput{RecordID: resp.GetRecordId(), Version: resp.GetVersion()}, nil
}

// DeleteRecord удаляет приватную запись пользователя.
func (a *App) DeleteRecord(ctx context.Context, recordID string) (DeleteRecordOutput, error) {
	var resp *pb.DeleteRecordResponse
	err := a.withAccessTokenRefresh(ctx, func(ctx context.Context) error {
		var err error
		resp, err = a.records.DeleteRecord(ctx, pb.DeleteRecordRequest_builder{RecordId: &recordID}.Build())
		return err
	})
	if err != nil {
		return DeleteRecordOutput{}, rpcError(
			err,
			"не удалось удалить приватную запись",
			map[codes.Code]string{
				codes.Unauthenticated: "сессия недействительна, войдите снова",
				codes.InvalidArgument: "некорректный идентификатор приватной записи",
				codes.NotFound:        "приватная запись не найдена",
			},
		)
	}

	return DeleteRecordOutput{RecordID: resp.GetRecordId()}, nil
}

func timestampAsTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}
