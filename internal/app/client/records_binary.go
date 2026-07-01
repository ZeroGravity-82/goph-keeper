package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"google.golang.org/grpc/codes"

	"zerogravity-82/goph-keeper/internal/crypto"
	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/transport/grpcclient"
)

const maxPlainBinaryFileSize = 100 * 1024 * 1024

// CreateBinaryInput содержит данные для создания бинарной приватной записи.
type CreateBinaryInput struct {
	Title       string
	Description string
	Filename    string
	ContentType string
	File        []byte
}

// UpdateBinaryInput содержит данные для обновления бинарной приватной записи.
type UpdateBinaryInput struct {
	RecordID        string
	ExpectedVersion int64
	Title           string
	Description     string
	Filename        string
	ContentType     string
	File            []byte
}

// UpdateBinaryMetadataInput содержит данные для обновления открытых метаданных бинарной приватной записи.
type UpdateBinaryMetadataInput struct {
	RecordID        string
	ExpectedVersion int64
	Title           string
	Description     string
}

// BinaryFile содержит расшифрованный файл бинарной приватной записи.
type BinaryFile struct {
	RecordID     string
	Filename     string
	ContentType  string
	DeclaredSize int64
	Data         []byte
}

// CreateBinary шифрует payload и файл на клиенте, затем создает бинарную приватную запись через потоковый запрос.
func (a *App) CreateBinary(ctx context.Context, in CreateBinaryInput) (CreateRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return CreateRecordOutput{}, err
	}
	if len(in.File) == 0 {
		return CreateRecordOutput{}, errors.New("файл не должен быть пустым")
	}
	if len(in.File) > maxPlainBinaryFileSize {
		return CreateRecordOutput{}, fmt.Errorf("размер файла превышает лимит %d байт", maxPlainBinaryFileSize)
	}

	encrypted, err := crypto.EncryptBinaryRecordData(
		a.masterKey,
		a.session.MasterKeySalt,
		model.BinaryPayload{
			Filename:    in.Filename,
			ContentType: in.ContentType,
			Size:        int64(len(in.File)),
		},
		in.File,
	)
	if err != nil {
		return CreateRecordOutput{}, fmt.Errorf("не удалось зашифровать бинарную приватную запись: %w", err)
	}

	ctx = grpcclient.WithAccessToken(ctx, a.session.AccessToken)
	stream, err := a.records.CreateBinaryRecord(ctx)
	if err != nil {
		return CreateRecordOutput{}, rpcError(
			err,
			"не удалось начать создание бинарной приватной записи",
			map[codes.Code]string{codes.Unauthenticated: "сессия недействительна, войдите снова"},
		)
	}

	uploadMode := pb.UploadMode_UPLOAD_MODE_SINGLE_PART
	encryptedSize := int64(len(encrypted.EncryptedFile.Data))
	if err = stream.Send(pb.CreateBinaryRecordRequest_builder{
		Metadata: pb.CreateBinaryRecordMetadata_builder{
			Title:            &in.Title,
			Description:      &in.Description,
			EncryptedDek:     encrypted.EncryptedDEK.Data,
			EncryptedPayload: encrypted.EncryptedPayload.Data,
			EncryptedSize:    &encryptedSize,
			UploadMode:       &uploadMode,
		}.Build(),
	}.Build()); err != nil {
		return CreateRecordOutput{}, rpcError(err, "не удалось отправить метаданные файла", nil)
	}
	if err = stream.Send(pb.CreateBinaryRecordRequest_builder{
		Chunk: encrypted.EncryptedFile.Data,
	}.Build()); err != nil {
		return CreateRecordOutput{}, rpcError(err, "не удалось отправить файл", nil)
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return CreateRecordOutput{}, rpcError(
			err,
			"не удалось создать бинарную приватную запись",
			map[codes.Code]string{
				codes.Unauthenticated:    "сессия недействительна, войдите снова",
				codes.InvalidArgument:    "некорректные данные бинарной приватной записи",
				codes.FailedPrecondition: "файл не может быть загружен в текущем состоянии",
				codes.ResourceExhausted:  "размер файла превышает допустимый лимит",
				codes.DeadlineExceeded:   "истекло время ожидания загрузки файла",
				codes.Canceled:           "загрузка файла отменена",
				codes.Unavailable:        "сервер временно недоступен",
				codes.PermissionDenied:   "доступ запрещен",
			},
		)
	}

	return CreateRecordOutput{RecordID: resp.GetRecordId(), Version: resp.GetVersion()}, nil
}

// UpdateBinaryMetadata обновляет открытые метаданные бинарной приватной записи без замены файла.
func (a *App) UpdateBinaryMetadata(ctx context.Context, in UpdateBinaryMetadataInput) (UpdateRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return UpdateRecordOutput{}, err
	}

	ctx = grpcclient.WithAccessToken(ctx, a.session.AccessToken)
	resp, err := a.records.GetRecord(ctx, pb.GetRecordRequest_builder{RecordId: &in.RecordID}.Build())
	if err != nil {
		return UpdateRecordOutput{}, rpcError(
			err,
			"не удалось получить бинарную приватную запись",
			map[codes.Code]string{
				codes.Unauthenticated: "сессия недействительна, войдите снова",
				codes.InvalidArgument: "некорректный идентификатор приватной записи",
				codes.NotFound:        "приватная запись не найдена",
			},
		)
	}
	record := resp.GetRecord()
	if record == nil {
		return UpdateRecordOutput{}, errors.New("сервер вернул пустую приватную запись")
	}
	if record.GetType() != pb.RecordType_RECORD_TYPE_BINARY {
		return UpdateRecordOutput{}, fmt.Errorf("приватная запись %s не является бинарной", in.RecordID)
	}

	return a.updateRecord(
		ctx,
		in.RecordID,
		in.Title,
		in.Description,
		record.GetEncryptedDek(),
		record.GetEncryptedPayload(),
		in.ExpectedVersion,
	)
}

// UpdateBinary шифрует новый payload и файл на клиенте, затем заменяет файл бинарной приватной записи.
func (a *App) UpdateBinary(ctx context.Context, in UpdateBinaryInput) (UpdateRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return UpdateRecordOutput{}, err
	}
	if len(in.File) == 0 {
		return UpdateRecordOutput{}, errors.New("файл не должен быть пустым")
	}
	if len(in.File) > maxPlainBinaryFileSize {
		return UpdateRecordOutput{}, fmt.Errorf("размер файла превышает лимит %d байт", maxPlainBinaryFileSize)
	}

	encrypted, err := crypto.EncryptBinaryRecordData(
		a.masterKey,
		a.session.MasterKeySalt,
		model.BinaryPayload{
			Filename:    in.Filename,
			ContentType: in.ContentType,
			Size:        int64(len(in.File)),
		},
		in.File,
	)
	if err != nil {
		return UpdateRecordOutput{}, fmt.Errorf("не удалось зашифровать бинарную приватную запись: %w", err)
	}

	ctx = grpcclient.WithAccessToken(ctx, a.session.AccessToken)
	stream, err := a.records.UpdateBinaryRecord(ctx)
	if err != nil {
		return UpdateRecordOutput{}, rpcError(
			err,
			"не удалось начать обновление бинарной приватной записи",
			map[codes.Code]string{codes.Unauthenticated: "сессия недействительна, войдите снова"},
		)
	}

	uploadMode := pb.UploadMode_UPLOAD_MODE_SINGLE_PART
	encryptedSize := int64(len(encrypted.EncryptedFile.Data))
	if err = stream.Send(pb.UpdateBinaryRecordRequest_builder{
		Metadata: pb.UpdateBinaryRecordMetadata_builder{
			RecordId:         &in.RecordID,
			Title:            &in.Title,
			Description:      &in.Description,
			EncryptedDek:     encrypted.EncryptedDEK.Data,
			EncryptedPayload: encrypted.EncryptedPayload.Data,
			EncryptedSize:    &encryptedSize,
			UploadMode:       &uploadMode,
			ExpectedVersion:  &in.ExpectedVersion,
		}.Build(),
	}.Build()); err != nil {
		return UpdateRecordOutput{}, rpcError(err, "не удалось отправить метаданные файла", nil)
	}
	if err = stream.Send(pb.UpdateBinaryRecordRequest_builder{
		Chunk: encrypted.EncryptedFile.Data,
	}.Build()); err != nil {
		return UpdateRecordOutput{}, rpcError(err, "не удалось отправить файл", nil)
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return UpdateRecordOutput{}, rpcError(
			err,
			"не удалось обновить бинарную приватную запись",
			map[codes.Code]string{
				codes.Unauthenticated:    "сессия недействительна, войдите снова",
				codes.InvalidArgument:    "некорректные данные бинарной приватной записи",
				codes.NotFound:           "приватная запись не найдена",
				codes.Aborted:            "приватная запись была изменена с другого клиента, получите актуальную версию",
				codes.FailedPrecondition: "файл не может быть загружен в текущем состоянии",
				codes.ResourceExhausted:  "размер файла превышает допустимый лимит",
				codes.DeadlineExceeded:   "истекло время ожидания загрузки файла",
				codes.Canceled:           "загрузка файла отменена",
				codes.Unavailable:        "сервер временно недоступен",
				codes.PermissionDenied:   "доступ запрещен",
			},
		)
	}

	return UpdateRecordOutput{RecordID: resp.GetRecordId(), Version: resp.GetVersion()}, nil
}

// DownloadBinaryFile скачивает зашифрованный файл, расшифровывает его на клиенте и возвращает исходные данные.
func (a *App) DownloadBinaryFile(ctx context.Context, recordID string) (BinaryFile, error) {
	if err := a.requireSession(); err != nil {
		return BinaryFile{}, err
	}

	ctx = grpcclient.WithAccessToken(ctx, a.session.AccessToken)
	resp, err := a.records.GetRecord(ctx, pb.GetRecordRequest_builder{RecordId: &recordID}.Build())
	if err != nil {
		return BinaryFile{}, rpcError(
			err,
			"не удалось получить метаданные файла",
			map[codes.Code]string{
				codes.Unauthenticated: "сессия недействительна, войдите снова",
				codes.InvalidArgument: "некорректный идентификатор приватной записи",
				codes.NotFound:        "приватная запись не найдена",
			},
		)
	}
	record := resp.GetRecord()
	if record == nil {
		return BinaryFile{}, errors.New("сервер вернул пустую приватную запись")
	}
	if record.GetType() != pb.RecordType_RECORD_TYPE_BINARY {
		return BinaryFile{}, fmt.Errorf("приватная запись %s не содержит файл", recordID)
	}

	payload, err := crypto.DecryptRecordData[model.BinaryPayload](
		a.masterKey,
		a.session.MasterKeySalt,
		crypto.EncryptedRecordData{
			EncryptedDEK:     model.EncryptedBlob{Data: record.GetEncryptedDek()},
			EncryptedPayload: model.EncryptedBlob{Data: record.GetEncryptedPayload()},
		},
	)
	if err != nil {
		return BinaryFile{}, fmt.Errorf("не удалось расшифровать описание файла: %w", err)
	}

	stream, err := a.records.DownloadFile(ctx, pb.DownloadFileRequest_builder{RecordId: &recordID}.Build())
	if err != nil {
		return BinaryFile{}, rpcError(
			err,
			"не удалось начать скачивание файла",
			map[codes.Code]string{
				codes.Unauthenticated:    "сессия недействительна, войдите снова",
				codes.InvalidArgument:    "некорректный идентификатор приватной записи",
				codes.NotFound:           "приватная запись не найдена",
				codes.FailedPrecondition: "файл еще не загружен",
			},
		)
	}

	var encryptedFile bytes.Buffer
	for {
		chunk, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return BinaryFile{}, rpcError(
				recvErr,
				"не удалось скачать файл",
				map[codes.Code]string{
					codes.Unauthenticated:    "сессия недействительна, войдите снова",
					codes.InvalidArgument:    "некорректный идентификатор приватной записи",
					codes.NotFound:           "приватная запись не найдена",
					codes.FailedPrecondition: "файл еще не загружен",
				},
			)
		}
		if _, err = encryptedFile.Write(chunk.GetChunk()); err != nil {
			return BinaryFile{}, fmt.Errorf("не удалось собрать скачанный файл: %w", err)
		}
	}

	file, err := crypto.DecryptBinaryRecordFile(
		a.masterKey,
		a.session.MasterKeySalt,
		model.EncryptedBlob{Data: record.GetEncryptedDek()},
		model.EncryptedBlob{Data: encryptedFile.Bytes()},
	)
	if err != nil {
		return BinaryFile{}, fmt.Errorf("не удалось расшифровать файл: %w", err)
	}
	if int64(len(file)) != payload.Size {
		return BinaryFile{}, errors.New("размер расшифрованного файла не совпадает с метаданными")
	}

	return BinaryFile{
		RecordID:     record.GetRecordId(),
		Filename:     payload.Filename,
		ContentType:  payload.ContentType,
		DeclaredSize: payload.Size,
		Data:         file,
	}, nil
}
