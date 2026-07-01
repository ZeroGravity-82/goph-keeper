package client

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"

	"zerogravity-82/goph-keeper/internal/crypto"
	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/transport/grpcclient"
)

// CreateCardInput содержит данные для создания приватной записи с данными банковской карты.
type CreateCardInput struct {
	Title       string
	Description string
	Number      string
	HolderName  string
	ExpiresAt   string
	CVC         string
}

// CardRecord содержит расшифрованную приватную запись с данными банковской карты.
type CardRecord struct {
	RecordID    string
	Version     int64
	Title       string
	Description string
	Number      string
	HolderName  string
	ExpiresAt   string
	CVC         string
}

// UpdateCardInput содержит данные для обновления приватной записи с данными банковской карты.
type UpdateCardInput struct {
	RecordID        string
	ExpectedVersion int64
	Title           string
	Description     string
	Number          string
	HolderName      string
	ExpiresAt       string
	CVC             string
}

// CreateCard шифрует payload на клиенте и создает приватную запись с данными банковской карты.
func (a *App) CreateCard(ctx context.Context, in CreateCardInput) (CreateRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return CreateRecordOutput{}, err
	}

	encrypted, err := crypto.EncryptRecordData(a.masterKey, a.session.MasterKeySalt, model.CardPayload{
		Number:     in.Number,
		HolderName: in.HolderName,
		ExpiresAt:  in.ExpiresAt,
		CVC:        in.CVC,
	})
	if err != nil {
		return CreateRecordOutput{}, fmt.Errorf("не удалось зашифровать приватную запись банковской карты: %w", err)
	}

	recordType := pb.RecordType_RECORD_TYPE_CARD
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
			"не удалось создать приватную запись банковской карты",
			map[codes.Code]string{
				codes.Unauthenticated: "сессия недействительна, войдите снова",
				codes.InvalidArgument: "некорректные данные приватной записи",
			},
		)
	}

	return CreateRecordOutput{RecordID: resp.GetRecordId(), Version: resp.GetVersion()}, nil
}

// UpdateCard шифрует обновленный payload на клиенте и обновляет приватную запись с данными банковской карты.
func (a *App) UpdateCard(ctx context.Context, in UpdateCardInput) (UpdateRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return UpdateRecordOutput{}, err
	}

	encrypted, err := crypto.EncryptRecordData(a.masterKey, a.session.MasterKeySalt, model.CardPayload{
		Number:     in.Number,
		HolderName: in.HolderName,
		ExpiresAt:  in.ExpiresAt,
		CVC:        in.CVC,
	})
	if err != nil {
		return UpdateRecordOutput{}, fmt.Errorf("не удалось зашифровать приватную запись банковской карты: %w", err)
	}

	return a.updateRecord(
		ctx,
		in.RecordID,
		in.Title,
		in.Description,
		encrypted.EncryptedDEK.Data,
		encrypted.EncryptedPayload.Data,
		in.ExpectedVersion,
	)
}

// GetCard получает приватную запись банковской карты и расшифровывает payload на клиенте.
func (a *App) GetCard(ctx context.Context, recordID string) (CardRecord, error) {
	if err := a.requireSession(); err != nil {
		return CardRecord{}, err
	}

	ctx = grpcclient.WithAccessToken(ctx, a.session.AccessToken)
	resp, err := a.records.GetRecord(ctx, pb.GetRecordRequest_builder{RecordId: &recordID}.Build())
	if err != nil {
		return CardRecord{}, rpcError(
			err,
			"не удалось получить приватную запись банковской карты",
			map[codes.Code]string{
				codes.Unauthenticated: "сессия недействительна, войдите снова",
				codes.InvalidArgument: "некорректный идентификатор приватной записи",
				codes.NotFound:        "приватная запись не найдена",
			},
		)
	}
	record := resp.GetRecord()
	if record == nil {
		return CardRecord{}, errors.New("сервер вернул пустую приватную запись")
	}
	if record.GetType() != pb.RecordType_RECORD_TYPE_CARD {
		return CardRecord{}, fmt.Errorf("приватная запись %s не содержит данные банковской карты", recordID)
	}

	payload, err := crypto.DecryptRecordData[model.CardPayload](
		a.masterKey,
		a.session.MasterKeySalt,
		crypto.EncryptedRecordData{
			EncryptedDEK:     model.EncryptedBlob{Data: record.GetEncryptedDek()},
			EncryptedPayload: model.EncryptedBlob{Data: record.GetEncryptedPayload()},
		},
	)
	if err != nil {
		return CardRecord{}, fmt.Errorf("не удалось расшифровать приватную запись банковской карты: %w", err)
	}

	return CardRecord{
		RecordID:    record.GetRecordId(),
		Version:     record.GetVersion(),
		Title:       record.GetTitle(),
		Description: record.GetDescription(),
		Number:      payload.Number,
		HolderName:  payload.HolderName,
		ExpiresAt:   payload.ExpiresAt,
		CVC:         payload.CVC,
	}, nil
}
