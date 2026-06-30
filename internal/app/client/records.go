package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	"zerogravity-82/goph-keeper/internal/crypto"
	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/transport/grpcclient"
)

// CreateCredentialInput содержит данные для создания приватной записи с учетными данными.
type CreateCredentialInput struct {
	Title              string
	Description        string
	CredentialLogin    string
	CredentialPassword string
}

// CreateRecordOutput содержит результат создания приватной записи.
type CreateRecordOutput struct {
	RecordID string
	Version  int64
}

// CredentialRecord содержит расшифрованную приватную запись с учетными данными.
type CredentialRecord struct {
	RecordID    string
	Title       string
	Description string
	Login       string
	Password    string
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

// CreateCredential шифрует payload на клиенте и создает приватную запись с учетными данными.
func (a *App) CreateCredential(ctx context.Context, in CreateCredentialInput) (CreateRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return CreateRecordOutput{}, err
	}

	encrypted, err := crypto.EncryptRecordData(a.masterKey, a.session.MasterKeySalt, model.CredentialPayload{
		Login:    in.CredentialLogin,
		Password: in.CredentialPassword,
	})
	if err != nil {
		return CreateRecordOutput{}, fmt.Errorf("не удалось зашифровать приватную запись с учетными данными: %w", err)
	}

	recordType := pb.RecordType_RECORD_TYPE_CREDENTIAL
	ctx = grpcclient.WithAccessToken(ctx, a.session.AccessToken)
	resp, err := a.records.CreateRecord(ctx, pb.CreateRecordRequest_builder{
		Type:             &recordType,
		Title:            &in.Title,
		Description:      &in.Description,
		EncryptedDek:     encrypted.EncryptedDEK.Data,
		EncryptedPayload: encrypted.EncryptedPayload.Data,
	}.Build())
	if err != nil {
		return CreateRecordOutput{}, rpcError(
			err,
			"не удалось создать приватную запись с учетными данными",
			map[codes.Code]string{
				codes.Unauthenticated: "сессия недействительна, войдите снова",
				codes.InvalidArgument: "некорректные данные приватной записи",
			},
		)
	}

	return CreateRecordOutput{RecordID: resp.GetRecordId(), Version: resp.GetVersion()}, nil
}

// GetCredential получает приватную запись и расшифровывает payload на клиенте.
func (a *App) GetCredential(ctx context.Context, recordID string) (CredentialRecord, error) {
	if err := a.requireSession(); err != nil {
		return CredentialRecord{}, err
	}

	ctx = grpcclient.WithAccessToken(ctx, a.session.AccessToken)
	resp, err := a.records.GetRecord(ctx, pb.GetRecordRequest_builder{RecordId: &recordID}.Build())
	if err != nil {
		return CredentialRecord{}, rpcError(
			err,
			"не удалось получить приватную запись с учетными данными",
			map[codes.Code]string{
				codes.Unauthenticated: "сессия недействительна, войдите снова",
				codes.InvalidArgument: "некорректный идентификатор приватной записи",
				codes.NotFound:        "приватная запись не найдена",
			},
		)
	}
	record := resp.GetRecord()
	if record == nil {
		return CredentialRecord{}, errors.New("сервер вернул пустую приватную запись")
	}
	if record.GetType() != pb.RecordType_RECORD_TYPE_CREDENTIAL {
		return CredentialRecord{}, fmt.Errorf("приватная запись %s не содержит учетные данные", recordID)
	}

	payload, err := crypto.DecryptRecordData[model.CredentialPayload](
		a.masterKey,
		a.session.MasterKeySalt,
		crypto.EncryptedRecordData{
			EncryptedDEK:     model.EncryptedBlob{Data: record.GetEncryptedDek()},
			EncryptedPayload: model.EncryptedBlob{Data: record.GetEncryptedPayload()},
		},
	)
	if err != nil {
		return CredentialRecord{}, fmt.Errorf("не удалось расшифровать приватную запись с учетными данными: %w", err)
	}

	return CredentialRecord{
		RecordID:    record.GetRecordId(),
		Title:       record.GetTitle(),
		Description: record.GetDescription(),
		Login:       payload.Login,
		Password:    payload.Password,
	}, nil
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
