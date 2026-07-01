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

func timestampAsTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}
