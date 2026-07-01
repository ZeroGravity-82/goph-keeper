package client

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/transport/grpcclient"
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

// ListRecords возвращает список приватных записей пользователя.
func (a *App) ListRecords(ctx context.Context) ([]RecordListItem, error) {
	if err := a.requireSession(); err != nil {
		return nil, err
	}

	ctx = grpcclient.WithAccessToken(ctx, a.session.AccessToken)
	resp, err := a.records.ListRecords(ctx, pb.ListRecordsRequest_builder{}.Build())
	if err != nil {
		return nil, rpcError(err, "не удалось получить список приватных записей", map[codes.Code]string{
			codes.Unauthenticated: "сессия недействительна, в аккаунт войдите снова",
		})
	}

	items := make([]RecordListItem, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, RecordListItem{
			RecordID:    item.GetRecordId(),
			Type:        item.GetType().String(),
			Title:       item.GetTitle(),
			Description: item.GetDescription(),
			CreatedAt:   timestampAsTime(item.GetCreatedAt()),
			UpdatedAt:   timestampAsTime(item.GetUpdatedAt()),
		})
	}
	return items, nil
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
	if err := a.requireSession(); err != nil {
		return UpdateRecordOutput{}, err
	}

	ctx = grpcclient.WithAccessToken(ctx, a.session.AccessToken)
	resp, err := a.records.UpdateRecord(ctx, pb.UpdateRecordRequest_builder{
		RecordId:         &recordID,
		Title:            &title,
		Description:      &description,
		EncryptedDek:     encryptedDEK,
		EncryptedPayload: encryptedPayload,
		ExpectedVersion:  &expectedVersion,
	}.Build())
	if err != nil {
		return UpdateRecordOutput{}, rpcError(
			err,
			"не удалось обновить приватную запись",
			map[codes.Code]string{
				codes.Unauthenticated: "сессия недействительна, войдите снова",
				codes.InvalidArgument: "некорректные данные приватной записи",
				codes.NotFound:        "приватная запись не найдена",
				codes.Aborted:         "приватная запись была изменена с другого клиента, получите актуальную версию",
			},
		)
	}

	return UpdateRecordOutput{RecordID: resp.GetRecordId(), Version: resp.GetVersion()}, nil
}

// DeleteRecord удаляет приватную запись пользователя.
func (a *App) DeleteRecord(ctx context.Context, recordID string) (DeleteRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return DeleteRecordOutput{}, err
	}

	ctx = grpcclient.WithAccessToken(ctx, a.session.AccessToken)
	resp, err := a.records.DeleteRecord(ctx, pb.DeleteRecordRequest_builder{RecordId: &recordID}.Build())
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
