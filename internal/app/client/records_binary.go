package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/crypto"
	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/pb"
)

const (
	binaryPlainFilePartSizeBytes int64 = 5 * 1024 * 1024
	binaryUploadChunkSizeBytes         = 512 * 1024
)

// Бинарный файл шифруется на клиенте не целиком, а частями. Каждая исходная часть получает собственные служебные данные
// AES-GCM, поэтому размер зашифрованной части равен размеру исходной части плюс crypto.EncryptedBlobOverhead().

// CreateBinaryInput содержит данные для создания бинарной приватной записи.
type CreateBinaryInput struct {
	Title       string
	Description string
	Filename    string
	ContentType string
	File        io.Reader
	FileSize    int64
	OnProgress  func(BinaryUploadProgress)
}

// UpdateBinaryInput содержит данные для обновления бинарной приватной записи.
type UpdateBinaryInput struct {
	RecordID        string
	ExpectedVersion int64
	Title           string
	Description     string
	Filename        string
	ContentType     string
	File            io.Reader
	FileSize        int64
	OnProgress      func(BinaryUploadProgress)
}

// BinaryUploadProgress описывает прогресс загрузки исходного файла до шифрования.
type BinaryUploadProgress struct {
	UploadedBytes int64
	TotalBytes    int64
}

// UpdateBinaryMetadataInput содержит данные для обновления открытых метаданных бинарной приватной записи.
type UpdateBinaryMetadataInput struct {
	RecordID        string
	ExpectedVersion int64
	Title           string
	Description     string
}

// BinaryRecord содержит расшифрованное описание бинарной приватной записи без содержимого файла.
type BinaryRecord struct {
	RecordID    string
	Version     int64
	Title       string
	Description string
	Filename    string
	ContentType string
	Size        int64
}

// BinaryFile содержит расшифрованный файл бинарной приватной записи.
type BinaryFile struct {
	RecordID     string
	Filename     string
	ContentType  string
	DeclaredSize int64
	Data         []byte
}

// CreateBinary валидирует метаданные и размер исходного файла, шифрует описание файла на клиенте и загружает
// зашифрованный файл на сервер через multipart-загрузку.
func (a *App) CreateBinary(ctx context.Context, in CreateBinaryInput) (CreateRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return CreateRecordOutput{}, err
	}
	if in.File == nil {
		return CreateRecordOutput{}, errors.New("файл не задан")
	}
	if in.FileSize == 0 {
		return CreateRecordOutput{}, errors.New("файл не должен быть пустым")
	}
	if err := validateBinaryFileSize(in.FileSize); err != nil {
		return CreateRecordOutput{}, err
	}
	if err := validateRecordMetadataSize(in.Title, in.Description); err != nil {
		return CreateRecordOutput{}, err
	}
	if err := validateBinaryPayloadSize(in.Filename, in.ContentType); err != nil {
		return CreateRecordOutput{}, err
	}

	encryption, err := crypto.NewBinaryRecordEncryption(
		a.masterKey,
		a.session.MasterKeySalt,
		model.BinaryPayload{
			Filename:    in.Filename,
			ContentType: in.ContentType,
			Size:        in.FileSize,
		},
	)
	if err != nil {
		return CreateRecordOutput{}, fmt.Errorf("не удалось зашифровать бинарную приватную запись: %w", err)
	}

	encryptedSize, err := crypto.EncryptedChunkedBlobSize(in.FileSize, binaryPlainFilePartSizeBytes)
	if err != nil {
		return CreateRecordOutput{}, fmt.Errorf("не удалось рассчитать размер зашифрованного файла: %w", err)
	}
	resp, err := a.createOrReplaceBinaryMultipart(ctx, binaryMultipartUploadInput{
		Title:            in.Title,
		Description:      in.Description,
		EncryptedDEK:     encryption.EncryptedDEK.Data,
		EncryptedPayload: encryption.EncryptedPayload.Data,
		File:             in.File,
		FileSize:         in.FileSize,
		Encryption:       encryption,
		EncryptedSize:    encryptedSize,
		OnProgress:       in.OnProgress,
	})
	if err != nil {
		return CreateRecordOutput{}, rpcError(
			err,
			"не удалось создать бинарную приватную запись",
			binaryRecordMutationErrorMessages,
		)
	}

	return CreateRecordOutput{RecordID: resp.recordID, Version: resp.version}, nil
}

// UpdateBinaryMetadata обновляет открытые метаданные бинарной приватной записи, не меняя зашифрованное описание файла и
// сам файл.
func (a *App) UpdateBinaryMetadata(ctx context.Context, in UpdateBinaryMetadataInput) (UpdateRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return UpdateRecordOutput{}, err
	}
	if err := validateRecordMetadataSize(in.Title, in.Description); err != nil {
		return UpdateRecordOutput{}, err
	}

	var resp *pb.GetRecordResponse
	err := a.withAccessTokenRefresh(ctx, func(ctx context.Context) error {
		var err error
		resp, err = a.records.GetRecord(ctx, pb.GetRecordRequest_builder{RecordId: &in.RecordID}.Build())
		return err
	})
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

// UpdateBinary валидирует новые метаданные и файл, шифрует описание файла на клиенте и заменяет файл через
// multipart-загрузку с учетом ожидаемой версии записи.
func (a *App) UpdateBinary(ctx context.Context, in UpdateBinaryInput) (UpdateRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return UpdateRecordOutput{}, err
	}
	if in.File == nil {
		return UpdateRecordOutput{}, errors.New("файл не задан")
	}
	if in.FileSize == 0 {
		return UpdateRecordOutput{}, errors.New("файл не должен быть пустым")
	}
	if err := validateBinaryFileSize(in.FileSize); err != nil {
		return UpdateRecordOutput{}, err
	}
	if err := validateRecordMetadataSize(in.Title, in.Description); err != nil {
		return UpdateRecordOutput{}, err
	}
	if err := validateBinaryPayloadSize(in.Filename, in.ContentType); err != nil {
		return UpdateRecordOutput{}, err
	}

	encryption, err := crypto.NewBinaryRecordEncryption(
		a.masterKey,
		a.session.MasterKeySalt,
		model.BinaryPayload{
			Filename:    in.Filename,
			ContentType: in.ContentType,
			Size:        in.FileSize,
		},
	)
	if err != nil {
		return UpdateRecordOutput{}, fmt.Errorf("не удалось зашифровать новый файл бинарной приватной записи: %w", err)
	}

	encryptedSize, err := crypto.EncryptedChunkedBlobSize(in.FileSize, binaryPlainFilePartSizeBytes)
	if err != nil {
		return UpdateRecordOutput{}, fmt.Errorf("не удалось рассчитать размер зашифрованного файла: %w", err)
	}
	resp, err := a.createOrReplaceBinaryMultipart(ctx, binaryMultipartUploadInput{
		RecordID:         in.RecordID,
		ExpectedVersion:  in.ExpectedVersion,
		Title:            in.Title,
		Description:      in.Description,
		EncryptedDEK:     encryption.EncryptedDEK.Data,
		EncryptedPayload: encryption.EncryptedPayload.Data,
		File:             in.File,
		FileSize:         in.FileSize,
		Encryption:       encryption,
		EncryptedSize:    encryptedSize,
		OnProgress:       in.OnProgress,
	})
	if err != nil {
		return UpdateRecordOutput{}, rpcError(
			err,
			"не удалось заменить файл бинарной приватной записи",
			binaryRecordMutationErrorMessages,
		)
	}

	return UpdateRecordOutput{RecordID: resp.recordID, Version: resp.version}, nil
}

// binaryMultipartUploadInput содержит подготовленные данные для создания или замены бинарной записи через
// multipart-загрузку.
type binaryMultipartUploadInput struct {
	RecordID         string
	ExpectedVersion  int64
	Title            string
	Description      string
	EncryptedDEK     []byte
	EncryptedPayload []byte
	File             io.Reader
	FileSize         int64
	Encryption       crypto.BinaryRecordEncryption
	EncryptedSize    int64
	OnProgress       func(BinaryUploadProgress)
}

// binaryMultipartUploadOutput содержит идентификатор и версию записи после завершения multipart-загрузки.
type binaryMultipartUploadOutput struct {
	recordID string
	version  int64
}

// createOrReplaceBinaryMultipart выполняет общий сценарий создания бинарной записи и замены файла:
// 1. открывает или восстанавливает multipart-загрузку на сервере;
// 2. получает список уже загруженных частей;
// 3. последовательно дочитывает исходный файл, пропуская части, которые сервер уже принял;
// 4. завершает загрузку после отправки всех недостающих частей.
func (a *App) createOrReplaceBinaryMultipart(
	ctx context.Context,
	in binaryMultipartUploadInput,
) (binaryMultipartUploadOutput, error) {
	partSize := chooseBinaryMultipartPartSize(in.FileSize, in.EncryptedSize)
	var startResp *pb.StartBinaryMultipartUploadResponse
	err := a.withAccessTokenRefresh(ctx, func(ctx context.Context) error {
		var err error
		startResp, err = a.records.StartBinaryMultipartUpload(ctx, pb.StartBinaryMultipartUploadRequest_builder{
			RecordId:         &in.RecordID,
			ExpectedVersion:  &in.ExpectedVersion,
			Title:            &in.Title,
			Description:      &in.Description,
			EncryptedDek:     in.EncryptedDEK,
			EncryptedPayload: in.EncryptedPayload,
			EncryptedSize:    &in.EncryptedSize,
			PartSize:         &partSize,
		}.Build())
		return err
	})
	if err != nil {
		return binaryMultipartUploadOutput{}, err
	}

	uploaded := uploadedMultipartParts(startResp.GetUploadedParts())
	if err = a.uploadBinaryMultipartParts(
		ctx,
		startResp.GetUploadId(),
		startResp.GetPartSize(),
		in.File,
		in.FileSize,
		in.Encryption,
		uploaded,
		in.OnProgress,
	); err != nil {
		return binaryMultipartUploadOutput{}, err
	}

	return a.completeBinaryMultipartUpload(ctx, startResp.GetUploadId())
}

// chooseBinaryMultipartPartSize выбирает размер зашифрованной части: весь файл для маленькой загрузки или один
// зашифрованный блок исходных данных для multipart-сценария.
func chooseBinaryMultipartPartSize(fileSize int64, encryptedSize int64) int64 {
	if fileSize <= binaryPlainFilePartSizeBytes {
		return encryptedSize
	}
	return binaryPlainFilePartSizeBytes + crypto.EncryptedBlobOverhead()
}

// uploadedMultipartParts преобразует список уже загруженных частей в мапу для быстрой проверки при восстановлении
// прерванной загрузки.
func uploadedMultipartParts(parts []*pb.MultipartUploadPart) map[int32]int64 {
	uploaded := make(map[int32]int64, len(parts))
	for _, part := range parts {
		uploaded[part.GetPartNumber()] = part.GetSize()
	}
	return uploaded
}

// uploadBinaryMultipartParts читает исходный файл по частям, пропускает уже загруженные части, шифрует недостающие
// части и отправляет их на сервер.
func (a *App) uploadBinaryMultipartParts(
	ctx context.Context,
	uploadID string,
	partSize int64,
	file io.Reader,
	fileSize int64,
	encryption crypto.BinaryRecordEncryption,
	uploaded map[int32]int64,
	onProgress func(BinaryUploadProgress),
) error {
	partCount := int32((fileSize + binaryPlainFilePartSizeBytes - 1) / binaryPlainFilePartSizeBytes)
	var uploadedBytes int64
	notifyBinaryUploadProgress(onProgress, uploadedBytes, fileSize)
	for partNumber := int32(1); partNumber <= partCount; partNumber++ {
		plainPartSize := expectedPlainFilePartSize(fileSize, partNumber)
		encryptedPartSize := plainPartSize + crypto.EncryptedBlobOverhead()
		// Сервер возвращает размер зашифрованной части. Для всех частей, кроме последней, клиент ожидает один и тот же
		// размер: стандартный блок исходных данных плюс служебные данные AES-GCM.
		if partNumber < partCount && partSize != encryptedPartSize {
			return fmt.Errorf("сервер вернул некорректный размер части multipart-загрузки")
		}
		if uploaded[partNumber] == encryptedPartSize {
			// Даже если часть уже загружена, исходный файл читается последовательно, поэтому соответствующий диапазон
			// нужно пропустить перед переходом к следующей части.
			if err := discardPlainFilePart(file, plainPartSize); err != nil {
				return err
			}
			uploadedBytes += plainPartSize
			notifyBinaryUploadProgress(onProgress, uploadedBytes, fileSize)
			continue
		}
		plainPart, err := readPlainFilePart(file, plainPartSize)
		if err != nil {
			return err
		}
		encryptedPart, err := encryption.EncryptFileChunk(partNumber, plainPart)
		if err != nil {
			return fmt.Errorf("не удалось зашифровать часть файла: %w", err)
		}
		part := encryptedPart.Data
		if err := a.uploadBinaryMultipartPartRetry(ctx, uploadID, partNumber, part); err != nil {
			statusResp, statusErr := a.getBinaryMultipartUploadStatus(ctx, uploadID)
			if statusErr == nil {
				uploaded = uploadedMultipartParts(statusResp.GetUploadedParts())
				if uploaded[partNumber] == int64(len(part)) {
					uploadedBytes += plainPartSize
					notifyBinaryUploadProgress(onProgress, uploadedBytes, fileSize)
					continue
				}
			}
			return err
		}
		uploaded[partNumber] = int64(len(part))
		uploadedBytes += plainPartSize
		notifyBinaryUploadProgress(onProgress, uploadedBytes, fileSize)
	}
	return nil
}

// notifyBinaryUploadProgress сообщает вызывающему коду прогресс загрузки, если callback был передан.
func notifyBinaryUploadProgress(onProgress func(BinaryUploadProgress), uploadedBytes int64, totalBytes int64) {
	if onProgress == nil {
		return
	}
	if uploadedBytes > totalBytes {
		uploadedBytes = totalBytes
	}
	onProgress(BinaryUploadProgress{UploadedBytes: uploadedBytes, TotalBytes: totalBytes})
}

// expectedPlainFilePartSize возвращает ожидаемый размер исходной части файла по номеру части с учетом возможной
// короткой последней части.
func expectedPlainFilePartSize(fileSize int64, partNumber int32) int64 {
	offset := int64(partNumber-1) * binaryPlainFilePartSizeBytes
	remaining := fileSize - offset
	if remaining < binaryPlainFilePartSizeBytes {
		return remaining
	}
	return binaryPlainFilePartSizeBytes
}

// readPlainFilePart читает из исходного файла ровно одну часть заданного размера.
func readPlainFilePart(file io.Reader, size int64) ([]byte, error) {
	part := make([]byte, size)
	if _, err := io.ReadFull(file, part); err != nil {
		return nil, fmt.Errorf("не удалось прочитать часть файла: %w", err)
	}
	return part, nil
}

// discardPlainFilePart пропускает часть исходного файла, которая уже есть на сервере после восстановления
// multipart-загрузки.
func discardPlainFilePart(file io.Reader, size int64) error {
	if _, err := io.CopyN(io.Discard, file, size); err != nil {
		return fmt.Errorf("не удалось пропустить уже загруженную часть файла: %w", err)
	}
	return nil
}

// uploadBinaryMultipartPartRetry отправляет одну зашифрованную часть с повторами для временных ошибок и проверяет
// статус загрузки перед повторной отправкой.
func (a *App) uploadBinaryMultipartPartRetry(
	ctx context.Context,
	uploadID string,
	partNumber int32,
	part []byte,
) error {
	backoff := defaultUnaryRetryBackoff()
	for attempt := 0; ; attempt++ {
		err := a.uploadBinaryMultipartPart(ctx, uploadID, partNumber, part)
		if err == nil {
			return nil
		}
		if attempt >= len(backoff) || !isRetriableUnaryError(err) {
			return err
		}
		statusResp, statusErr := a.getBinaryMultipartUploadStatus(ctx, uploadID)
		if statusErr == nil {
			for _, uploadedPart := range statusResp.GetUploadedParts() {
				if uploadedPart.GetPartNumber() == partNumber && uploadedPart.GetSize() == int64(len(part)) {
					// Ответ на загрузку части мог потеряться после того, как сервер уже сохранил часть.
					return nil
				}
			}
		}
		if err = sleepContext(ctx, backoff[attempt]); err != nil {
			return err
		}
	}
}

// uploadBinaryMultipartPart отправляет одну зашифрованную часть через клиентский поток: сначала метаданные, затем
// фрагменты данных.
func (a *App) uploadBinaryMultipartPart(ctx context.Context, uploadID string, partNumber int32, part []byte) error {
	partSize := int64(len(part))
	return a.withAccessTokenRefresh(ctx, func(ctx context.Context) error {
		stream, err := a.records.UploadBinaryMultipartPart(ctx)
		if err != nil {
			return err
		}
		if err = stream.Send(pb.UploadBinaryMultipartPartRequest_builder{
			Metadata: pb.UploadBinaryMultipartPartMetadata_builder{
				UploadId:   &uploadID,
				PartNumber: &partNumber,
				PartSize:   &partSize,
			}.Build(),
		}.Build()); err != nil {
			return err
		}
		for from := 0; from < len(part); from += binaryUploadChunkSizeBytes {
			to := min(from+binaryUploadChunkSizeBytes, len(part))
			if err = stream.Send(pb.UploadBinaryMultipartPartRequest_builder{Chunk: part[from:to]}.Build()); err != nil {
				return err
			}
		}
		_, err = stream.CloseAndRecv()
		return err
	})
}

// getBinaryMultipartUploadStatus получает состояние multipart-загрузки и список уже принятых сервером частей.
func (a *App) getBinaryMultipartUploadStatus(
	ctx context.Context,
	uploadID string,
) (*pb.GetBinaryMultipartUploadStatusResponse, error) {
	var resp *pb.GetBinaryMultipartUploadStatusResponse
	err := a.withAccessTokenRefreshRetry(ctx, func(ctx context.Context) error {
		var err error
		resp, err = a.records.GetBinaryMultipartUploadStatus(ctx, pb.GetBinaryMultipartUploadStatusRequest_builder{
			UploadId: &uploadID,
		}.Build())
		return err
	})
	return resp, err
}

// completeBinaryMultipartUpload завершает multipart-загрузку и учитывает случай, когда сервер уже успел собрать файл
// при повторе запроса.
func (a *App) completeBinaryMultipartUpload(
	ctx context.Context,
	uploadID string,
) (binaryMultipartUploadOutput, error) {
	backoff := defaultUnaryRetryBackoff()
	for attempt := 0; ; attempt++ {
		var resp *pb.CompleteBinaryMultipartUploadResponse
		err := a.withAccessTokenRefresh(ctx, func(ctx context.Context) error {
			var err error
			resp, err = a.records.CompleteBinaryMultipartUpload(ctx, pb.CompleteBinaryMultipartUploadRequest_builder{
				UploadId: &uploadID,
			}.Build())
			return err
		})
		if err == nil {
			return binaryMultipartUploadOutput{recordID: resp.GetRecordId(), version: resp.GetVersion()}, nil
		}
		if status.Code(err) == codes.FailedPrecondition || isRetriableUnaryError(err) {
			statusResp, statusErr := a.getBinaryMultipartUploadStatus(ctx, uploadID)
			if statusErr == nil && statusResp.GetUploadStatus() == pb.UploadStatus_UPLOAD_STATUS_UPLOADED {
				// Complete мог успешно выполниться на сервере, но клиент получил сетевую ошибку или повторный
				// запрос пришел уже после сборки файла.
				return binaryMultipartUploadOutput{
					recordID: statusResp.GetRecordId(),
					version:  statusResp.GetVersion(),
				}, nil
			}
		}
		if attempt >= len(backoff) || !isRetriableUnaryError(err) {
			return binaryMultipartUploadOutput{}, err
		}
		if err = sleepContext(ctx, backoff[attempt]); err != nil {
			return binaryMultipartUploadOutput{}, err
		}
	}
}

// GetBinary получает бинарную приватную запись без скачивания файла и расшифровывает клиентские данные с описанием
// файла.
func (a *App) GetBinary(ctx context.Context, recordID string) (BinaryRecord, error) {
	if err := a.requireSession(); err != nil {
		return BinaryRecord{}, err
	}

	var resp *pb.GetRecordResponse
	err := a.withAccessTokenRefreshRetry(ctx, func(ctx context.Context) error {
		var err error
		resp, err = a.records.GetRecord(ctx, pb.GetRecordRequest_builder{RecordId: &recordID}.Build())
		return err
	})
	if err != nil {
		return BinaryRecord{}, rpcError(
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
		return BinaryRecord{}, errors.New("сервер вернул пустую приватную запись")
	}
	if record.GetType() != pb.RecordType_RECORD_TYPE_BINARY {
		return BinaryRecord{}, fmt.Errorf("приватная запись %s не является бинарной", recordID)
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
		return BinaryRecord{}, fmt.Errorf("не удалось расшифровать описание файла: %w", err)
	}

	return BinaryRecord{
		RecordID:    record.GetRecordId(),
		Version:     record.GetVersion(),
		Title:       record.GetTitle(),
		Description: record.GetDescription(),
		Filename:    payload.Filename,
		ContentType: payload.ContentType,
		Size:        payload.Size,
	}, nil
}

// DownloadBinaryFile получает метаданные записи, скачивает зашифрованный файл, расшифровывает его на клиенте и
// проверяет размер исходного файла.
func (a *App) DownloadBinaryFile(ctx context.Context, recordID string) (BinaryFile, error) {
	if err := a.requireSession(); err != nil {
		return BinaryFile{}, err
	}

	var resp *pb.GetRecordResponse
	err := a.withAccessTokenRefreshRetry(ctx, func(ctx context.Context) error {
		var err error
		resp, err = a.records.GetRecord(ctx, pb.GetRecordRequest_builder{RecordId: &recordID}.Build())
		return err
	})
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

	var encryptedFile bytes.Buffer
	err = a.withAccessTokenRefresh(ctx, func(ctx context.Context) error {
		encryptedFile.Reset()
		stream, err := a.records.DownloadFile(ctx, pb.DownloadFileRequest_builder{RecordId: &recordID}.Build())
		if err != nil {
			return err
		}
		for {
			chunk, recvErr := stream.Recv()
			if errors.Is(recvErr, io.EOF) {
				return nil
			}
			if recvErr != nil {
				return recvErr
			}
			if _, err = encryptedFile.Write(chunk.GetChunk()); err != nil {
				return fmt.Errorf("не удалось собрать скачанный файл: %w", err)
			}
		}
	})
	if err != nil {
		return BinaryFile{}, rpcError(
			err,
			"не удалось скачать файл",
			map[codes.Code]string{
				codes.Unauthenticated:    "сессия недействительна, войдите снова",
				codes.InvalidArgument:    "некорректный идентификатор приватной записи",
				codes.NotFound:           "приватная запись не найдена",
				codes.FailedPrecondition: "файл еще не загружен",
			},
		)
	}

	file, err := crypto.DecryptBinaryRecordFileChunks(
		a.masterKey,
		a.session.MasterKeySalt,
		model.EncryptedBlob{Data: record.GetEncryptedDek()},
		model.EncryptedBlob{Data: encryptedFile.Bytes()},
		payload.Size,
		binaryPlainFilePartSizeBytes,
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
