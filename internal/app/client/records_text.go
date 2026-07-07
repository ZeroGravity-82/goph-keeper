package client

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"

	"zerogravity-82/goph-keeper/internal/crypto"
	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/pb"
)

// CreateTextInput содержит данные для создания текстовой приватной записи.
type CreateTextInput struct {
	Title       string
	Description string
	Text        string
}

// TextRecord содержит расшифрованную текстовую приватную запись.
type TextRecord struct {
	RecordID    string
	Version     int64
	Title       string
	Description string
	Text        string
}

// UpdateTextInput содержит данные для обновления текстовой приватной записи.
type UpdateTextInput struct {
	RecordID        string
	ExpectedVersion int64
	Title           string
	Description     string
	Text            string
}

// CreateText шифрует payload на клиенте и создает текстовую приватную запись.
func (a *App) CreateText(ctx context.Context, in CreateTextInput) (CreateRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return CreateRecordOutput{}, err
	}
	if err := validateRecordMetadataSize(in.Title, in.Description); err != nil {
		return CreateRecordOutput{}, err
	}
	if err := validateTextRecordPayloadSize(in.Text); err != nil {
		return CreateRecordOutput{}, err
	}

	encrypted, err := crypto.EncryptRecordData(a.masterKey, a.session.MasterKeySalt, model.TextPayload{
		Text: in.Text,
	})
	if err != nil {
		return CreateRecordOutput{}, fmt.Errorf("не удалось зашифровать текстовую приватную запись: %w", err)
	}

	recordType := pb.RecordType_RECORD_TYPE_TEXT
	var resp *pb.CreateRecordResponse
	err = a.withAccessTokenRefresh(ctx, func(ctx context.Context) error {
		var err error
		resp, err = a.records.CreateRecord(ctx, pb.CreateRecordRequest_builder{
			Type:             &recordType,
			Title:            &in.Title,
			Description:      &in.Description,
			EncryptedDek:     encrypted.EncryptedDEK.Data,
			EncryptedPayload: encrypted.EncryptedPayload.Data,
		}.Build())
		return err
	})
	if err != nil {
		return CreateRecordOutput{}, rpcError(
			err,
			"не удалось создать текстовую приватную запись",
			recordMutationErrorMessages,
		)
	}

	return CreateRecordOutput{RecordID: resp.GetRecordId(), Version: resp.GetVersion()}, nil
}

// UpdateText шифрует обновленный payload на клиенте и обновляет текстовую приватную запись.
func (a *App) UpdateText(ctx context.Context, in UpdateTextInput) (UpdateRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return UpdateRecordOutput{}, err
	}
	if err := validateRecordMetadataSize(in.Title, in.Description); err != nil {
		return UpdateRecordOutput{}, err
	}
	if err := validateTextRecordPayloadSize(in.Text); err != nil {
		return UpdateRecordOutput{}, err
	}

	encrypted, err := crypto.EncryptRecordData(a.masterKey, a.session.MasterKeySalt, model.TextPayload{
		Text: in.Text,
	})
	if err != nil {
		return UpdateRecordOutput{}, fmt.Errorf("не удалось зашифровать текстовую приватную запись: %w", err)
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

// GetText получает текстовую приватную запись и расшифровывает payload на клиенте.
func (a *App) GetText(ctx context.Context, recordID string) (TextRecord, error) {
	var resp *pb.GetRecordResponse
	err := a.withAccessTokenRefreshRetry(ctx, func(ctx context.Context) error {
		var err error
		resp, err = a.records.GetRecord(ctx, pb.GetRecordRequest_builder{RecordId: &recordID}.Build())
		return err
	})
	if err != nil {
		return TextRecord{}, rpcError(
			err,
			"не удалось получить текстовую приватную запись",
			map[codes.Code]string{
				codes.Unauthenticated: "сессия недействительна, войдите снова",
				codes.InvalidArgument: "некорректный идентификатор приватной записи",
				codes.NotFound:        "приватная запись не найдена",
			},
		)
	}
	record := resp.GetRecord()
	if record == nil {
		return TextRecord{}, errors.New("сервер вернул пустую приватную запись")
	}
	if record.GetType() != pb.RecordType_RECORD_TYPE_TEXT {
		return TextRecord{}, fmt.Errorf("приватная запись %s не содержит текстовые данные", recordID)
	}

	payload, err := crypto.DecryptRecordData[model.TextPayload](
		a.masterKey,
		a.session.MasterKeySalt,
		crypto.EncryptedRecordData{
			EncryptedDEK:     model.EncryptedBlob{Data: record.GetEncryptedDek()},
			EncryptedPayload: model.EncryptedBlob{Data: record.GetEncryptedPayload()},
		},
	)
	if err != nil {
		return TextRecord{}, fmt.Errorf("не удалось расшифровать текстовую приватную запись: %w", err)
	}

	return TextRecord{
		RecordID:    record.GetRecordId(),
		Version:     record.GetVersion(),
		Title:       record.GetTitle(),
		Description: record.GetDescription(),
		Text:        payload.Text,
	}, nil
}
