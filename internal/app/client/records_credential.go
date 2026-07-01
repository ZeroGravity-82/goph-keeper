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

// CreateCredentialInput содержит данные для создания приватной записи с учетными данными.
type CreateCredentialInput struct {
	Title              string
	Description        string
	CredentialLogin    string
	CredentialPassword string
}

// CredentialRecord содержит расшифрованную приватную запись с учетными данными.
type CredentialRecord struct {
	RecordID    string
	Version     int64
	Title       string
	Description string
	Login       string
	Password    string
}

// UpdateCredentialInput содержит данные для обновления приватной записи с учетными данными.
type UpdateCredentialInput struct {
	RecordID           string
	ExpectedVersion    int64
	Title              string
	Description        string
	CredentialLogin    string
	CredentialPassword string
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

// UpdateCredential шифрует обновленный payload на клиенте и обновляет приватную запись с учетными данными.
func (a *App) UpdateCredential(ctx context.Context, in UpdateCredentialInput) (UpdateRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return UpdateRecordOutput{}, err
	}

	encrypted, err := crypto.EncryptRecordData(a.masterKey, a.session.MasterKeySalt, model.CredentialPayload{
		Login:    in.CredentialLogin,
		Password: in.CredentialPassword,
	})
	if err != nil {
		return UpdateRecordOutput{}, fmt.Errorf("не удалось зашифровать приватную запись с учетными данными: %w", err)
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
		Version:     record.GetVersion(),
		Title:       record.GetTitle(),
		Description: record.GetDescription(),
		Login:       payload.Login,
		Password:    payload.Password,
	}, nil
}
