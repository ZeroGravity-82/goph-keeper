package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/logging"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/transport/grpcserver/authcontext"
	"zerogravity-82/goph-keeper/internal/usecase"
)

var (
	errInvalidBinaryRecordStream = errors.New("binary record stream is invalid")
	errReadBinaryRecordStream    = errors.New("failed to read binary record stream")
)

const (
	downloadChunkSizeBytes             = 64 * 1024
	recordTitleMaxSizeChars            = 128
	recordDescriptionMaxSizeChars      = 1024
	recordEncryptedDEKMaxSizeBytes     = 128
	recordEncryptedPayloadMaxSizeBytes = 1024 * 1024
	encryptedFileMaxSizeBytes          = 1025 * 1024 * 1024
	multipartPartMinSizeBytes          = 5 * 1024 * 1024
)

// recordsUseCase описывает сценарии работы с приватными записями: создание, чтение, обновление, удаление приватных
// записей и работу с бинарными файлами.
type recordsUseCase interface {
	CreateRecord(ctx context.Context, in usecase.CreateRecordInput) (usecase.CreateRecordOutput, error)
	StartBinaryMultipartUpload(
		ctx context.Context,
		in usecase.StartBinaryMultipartUploadInput,
	) (usecase.StartBinaryMultipartUploadOutput, error)
	GetBinaryMultipartUploadStatus(
		ctx context.Context,
		in usecase.GetBinaryMultipartUploadStatusInput,
	) (usecase.GetBinaryMultipartUploadStatusOutput, error)
	UploadBinaryMultipartPart(
		ctx context.Context,
		in usecase.UploadBinaryMultipartPartInput,
	) (usecase.UploadBinaryMultipartPartOutput, error)
	CompleteBinaryMultipartUpload(
		ctx context.Context,
		in usecase.CompleteBinaryMultipartUploadInput,
	) (usecase.CompleteBinaryMultipartUploadOutput, error)
	AbortBinaryMultipartUpload(
		ctx context.Context,
		in usecase.AbortBinaryMultipartUploadInput,
	) (usecase.AbortBinaryMultipartUploadOutput, error)
	ListRecords(ctx context.Context, in usecase.ListRecordsInput) (usecase.ListRecordsOutput, error)
	GetRecord(ctx context.Context, in usecase.GetRecordInput) (usecase.GetRecordOutput, error)
	UpdateRecord(ctx context.Context, in usecase.UpdateRecordInput) (usecase.UpdateRecordOutput, error)
	DeleteRecord(ctx context.Context, in usecase.DeleteRecordInput) (usecase.DeleteRecordOutput, error)
	DownloadFile(ctx context.Context, in usecase.DownloadFileInput) (usecase.DownloadFileOutput, error)
}

// RecordsService реализует gRPC-сервис приватных записей.
type RecordsService struct {
	pb.UnimplementedRecordsServer

	uc     recordsUseCase
	logger *slog.Logger
}

// NewRecordsService создает RecordsService.
func NewRecordsService(uc recordsUseCase, logger *slog.Logger) (*RecordsService, error) {
	if uc == nil {
		return nil, errors.New("records use case is not provided")
	}
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &RecordsService{uc: uc, logger: logger}, nil
}

// CreateRecord создает приватную запись.
func (s *RecordsService) CreateRecord(ctx context.Context, req *pb.CreateRecordRequest) (*pb.CreateRecordResponse, error) {
	in, err := createRecordInputFromRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	out, err := s.uc.CreateRecord(ctx, in)
	if err != nil {
		if errors.Is(err, usecase.ErrBinaryRecordNotSupported) {
			return nil, status.Error(codes.InvalidArgument, "binary record requires StartBinaryMultipartUpload")
		}
		s.logger.Error("failed to create record", slog.Any("err", err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	return pb.CreateRecordResponse_builder{
		RecordId: new(out.RecordID.String()),
		Version:  &out.Version,
	}.Build(), nil
}

// createRecordInputFromRequest валидирует gRPC-запрос и преобразует его во входной DTO сценария создания приватной
// записи.
func createRecordInputFromRequest(ctx context.Context, req *pb.CreateRecordRequest) (usecase.CreateRecordInput, error) {
	if req == nil {
		return usecase.CreateRecordInput{}, status.Error(codes.InvalidArgument, "request is required")
	}
	userID, ok := authcontext.UserIDFromContext(ctx)
	if !ok {
		return usecase.CreateRecordInput{}, status.Error(codes.Unauthenticated, "authentication is required")
	}
	recordType, ok := recordTypeFromProto(req.GetType())
	if !ok {
		return usecase.CreateRecordInput{}, status.Error(codes.InvalidArgument, "record type is invalid")
	}
	if strings.TrimSpace(req.GetTitle()) == "" {
		return usecase.CreateRecordInput{}, status.Error(codes.InvalidArgument, "title is required")
	}
	if len(req.GetEncryptedDek()) == 0 {
		return usecase.CreateRecordInput{}, status.Error(codes.InvalidArgument, "encrypted dek is required")
	}
	if len(req.GetEncryptedPayload()) == 0 {
		return usecase.CreateRecordInput{}, status.Error(codes.InvalidArgument, "encrypted payload is required")
	}
	if err := validateRecordDataSize(
		req.GetTitle(),
		req.GetDescription(),
		req.GetEncryptedDek(),
		req.GetEncryptedPayload(),
	); err != nil {
		return usecase.CreateRecordInput{}, err
	}

	return usecase.CreateRecordInput{
		UserID:           userID,
		Type:             recordType,
		Title:            req.GetTitle(),
		Description:      req.GetDescription(),
		EncryptedDEK:     req.GetEncryptedDek(),
		EncryptedPayload: req.GetEncryptedPayload(),
	}, nil
}

// validateRecordDataSize проверяет размер открытых метаданных и зашифрованных данных из gRPC-запроса.
func validateRecordDataSize(title string, description string, encryptedDEK []byte, encryptedPayload []byte) error {
	if utf8.RuneCountInString(title) > recordTitleMaxSizeChars {
		return status.Error(codes.InvalidArgument, "title exceeds size limit")
	}
	if utf8.RuneCountInString(description) > recordDescriptionMaxSizeChars {
		return status.Error(codes.InvalidArgument, "description exceeds size limit")
	}
	if len(encryptedDEK) > recordEncryptedDEKMaxSizeBytes {
		return status.Error(codes.InvalidArgument, "encrypted dek exceeds size limit")
	}
	if len(encryptedPayload) > recordEncryptedPayloadMaxSizeBytes {
		return status.Error(codes.InvalidArgument, "encrypted payload exceeds size limit")
	}
	return nil
}

// validateBinaryEncryptedSize проверяет заявленный размер зашифрованного файла из метаданных gRPC-стрима.
func validateBinaryEncryptedSize(encryptedSize int64) error {
	if encryptedSize <= 0 {
		return status.Error(codes.InvalidArgument, "encrypted size is invalid")
	}
	if encryptedSize > encryptedFileMaxSizeBytes {
		return status.Error(codes.ResourceExhausted, "encrypted file exceeds size limit")
	}
	return nil
}

// validateMultipartPartSize проверяет размер части multipart-загрузки с учетом ограничений S3-совместимого API.
func validateMultipartPartSize(encryptedSize int64, partSize int64) error {
	if partSize <= 0 {
		return status.Error(codes.InvalidArgument, "multipart part size is invalid")
	}
	if partSize > encryptedSize {
		return status.Error(codes.InvalidArgument, "multipart part size exceeds encrypted file size")
	}
	if encryptedSize > multipartPartMinSizeBytes && partSize < multipartPartMinSizeBytes {
		return status.Error(codes.InvalidArgument, "multipart part size is below minimum")
	}
	return nil
}

// recordTypeFromProto преобразует protobuf-тип приватной записи в ее доменный тип.
func recordTypeFromProto(recordType pb.RecordType) (model.RecordType, bool) {
	switch recordType {
	case pb.RecordType_RECORD_TYPE_CREDENTIAL:
		return model.RecordTypeCredential, true
	case pb.RecordType_RECORD_TYPE_TEXT:
		return model.RecordTypeText, true
	case pb.RecordType_RECORD_TYPE_CARD:
		return model.RecordTypeCard, true
	case pb.RecordType_RECORD_TYPE_BINARY:
		return model.RecordTypeBinary, true
	default:
		return "", false
	}
}

// StartBinaryMultipartUpload создает бинарную приватную запись и начинает возобновляемую multipart-загрузку файла.
func (s *RecordsService) StartBinaryMultipartUpload(
	ctx context.Context,
	req *pb.StartBinaryMultipartUploadRequest,
) (*pb.StartBinaryMultipartUploadResponse, error) {
	in, err := startBinaryMultipartUploadInputFromRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	out, err := s.uc.StartBinaryMultipartUpload(ctx, in)
	if err != nil {
		return nil, s.multipartUploadStatusError("failed to start binary multipart upload", err)
	}

	return pb.StartBinaryMultipartUploadResponse_builder{
		UploadId:      new(out.UploadID.String()),
		RecordId:      new(out.RecordID.String()),
		Version:       &out.Version,
		PartSize:      &out.PartSize,
		UploadStatus:  new(uploadStatusToProto(out.UploadStatus)),
		UploadedParts: multipartPartsToProto(out.UploadedParts),
	}.Build(), nil
}

// startBinaryMultipartUploadInputFromRequest валидирует запрос начала multipart-загрузки и преобразует его в DTO.
func startBinaryMultipartUploadInputFromRequest(
	ctx context.Context,
	req *pb.StartBinaryMultipartUploadRequest,
) (usecase.StartBinaryMultipartUploadInput, error) {
	if req == nil {
		return usecase.StartBinaryMultipartUploadInput{}, status.Error(codes.InvalidArgument, "request is required")
	}
	userID, ok := authcontext.UserIDFromContext(ctx)
	if !ok {
		return usecase.StartBinaryMultipartUploadInput{}, status.Error(
			codes.Unauthenticated,
			"authentication is required",
		)
	}
	if strings.TrimSpace(req.GetTitle()) == "" {
		return usecase.StartBinaryMultipartUploadInput{}, status.Error(codes.InvalidArgument, "title is required")
	}
	if len(req.GetEncryptedDek()) == 0 {
		return usecase.StartBinaryMultipartUploadInput{}, status.Error(
			codes.InvalidArgument,
			"encrypted dek is required",
		)
	}
	if len(req.GetEncryptedPayload()) == 0 {
		return usecase.StartBinaryMultipartUploadInput{}, status.Error(
			codes.InvalidArgument,
			"encrypted payload is required",
		)
	}
	if err := validateRecordDataSize(
		req.GetTitle(),
		req.GetDescription(),
		req.GetEncryptedDek(),
		req.GetEncryptedPayload(),
	); err != nil {
		return usecase.StartBinaryMultipartUploadInput{}, err
	}
	if err := validateBinaryEncryptedSize(req.GetEncryptedSize()); err != nil {
		return usecase.StartBinaryMultipartUploadInput{}, err
	}
	if err := validateMultipartPartSize(req.GetEncryptedSize(), req.GetPartSize()); err != nil {
		return usecase.StartBinaryMultipartUploadInput{}, err
	}
	var recordID uuid.UUID
	if req.GetRecordId() != "" {
		var err error
		recordID, err = uuid.Parse(req.GetRecordId())
		if err != nil || recordID == uuid.Nil {
			return usecase.StartBinaryMultipartUploadInput{}, status.Error(
				codes.InvalidArgument,
				"record id is invalid",
			)
		}
		if req.GetExpectedVersion() <= 0 {
			return usecase.StartBinaryMultipartUploadInput{}, status.Error(
				codes.InvalidArgument,
				"expected version is invalid",
			)
		}
	}
	return usecase.StartBinaryMultipartUploadInput{
		UserID:           userID,
		RecordID:         recordID,
		ExpectedVersion:  req.GetExpectedVersion(),
		Title:            req.GetTitle(),
		Description:      req.GetDescription(),
		EncryptedDEK:     req.GetEncryptedDek(),
		EncryptedPayload: req.GetEncryptedPayload(),
		EncryptedSize:    req.GetEncryptedSize(),
		PartSize:         req.GetPartSize(),
	}, nil
}

// GetBinaryMultipartUploadStatus возвращает состояние multipart-загрузки и список уже загруженных частей.
func (s *RecordsService) GetBinaryMultipartUploadStatus(
	ctx context.Context,
	req *pb.GetBinaryMultipartUploadStatusRequest,
) (*pb.GetBinaryMultipartUploadStatusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	userID, uploadID, err := multipartUploadIDFromRequest(ctx, req.GetUploadId())
	if err != nil {
		return nil, err
	}

	out, err := s.uc.GetBinaryMultipartUploadStatus(ctx, usecase.GetBinaryMultipartUploadStatusInput{
		UserID:   userID,
		UploadID: uploadID,
	})
	if err != nil {
		return nil, s.multipartUploadStatusError("failed to get binary multipart upload status", err)
	}

	return pb.GetBinaryMultipartUploadStatusResponse_builder{
		UploadId:      new(out.UploadID.String()),
		RecordId:      new(out.RecordID.String()),
		Version:       &out.Version,
		EncryptedSize: &out.EncryptedSize,
		PartSize:      &out.PartSize,
		UploadStatus:  new(uploadStatusToProto(out.UploadStatus)),
		UploadedParts: multipartPartsToProto(out.UploadedParts),
	}.Build(), nil
}

// UploadBinaryMultipartPart загружает одну часть файла в активную сессию multipart-загрузки.
func (s *RecordsService) UploadBinaryMultipartPart(stream pb.Records_UploadBinaryMultipartPartServer) error {
	streamInput, err := uploadBinaryMultipartPartInputFromStream(stream)
	if err != nil {
		return err
	}

	out, usecaseErr := s.uc.UploadBinaryMultipartPart(stream.Context(), streamInput.in)
	if usecaseErr != nil {
		_ = streamInput.partReader.CloseWithError(usecaseErr)
	} else {
		_ = streamInput.partReader.Close()
	}

	producerErr := waitBinaryMultipartPartChunksProducer(streamInput)
	if producerErr != nil {
		if errors.Is(producerErr, usecase.ErrBinaryEncryptedSizeMismatch) ||
			errors.Is(producerErr, errInvalidBinaryRecordStream) {
			return status.Error(codes.InvalidArgument, producerErr.Error())
		}
		if usecaseErr == nil {
			s.logger.Error("failed to receive binary multipart part chunks", slog.Any("err", producerErr))
			return status.Error(codes.Internal, "internal error")
		}
	}
	if usecaseErr != nil {
		return s.multipartUploadStatusError("failed to upload binary multipart part", usecaseErr)
	}

	return stream.SendAndClose(pb.UploadBinaryMultipartPartResponse_builder{
		UploadId: new(out.UploadID.String()),
		Part:     multipartPartToProto(out.Part),
	}.Build())
}

// uploadBinaryMultipartPartStreamInput содержит входные данные сценария и служебные объекты для чтения части файла.
type uploadBinaryMultipartPartStreamInput struct {
	in            usecase.UploadBinaryMultipartPartInput
	partReader    *io.PipeReader
	producerErrCh <-chan error
}

// uploadBinaryMultipartPartInputFromStream читает метаданные части и готовит потоковое чтение ее содержимого.
func uploadBinaryMultipartPartInputFromStream(
	stream pb.Records_UploadBinaryMultipartPartServer,
) (uploadBinaryMultipartPartStreamInput, error) {
	userID, ok := authcontext.UserIDFromContext(stream.Context())
	if !ok {
		return uploadBinaryMultipartPartStreamInput{}, status.Error(codes.Unauthenticated, "authentication is required")
	}

	first, err := stream.Recv()
	if errors.Is(err, io.EOF) {
		return uploadBinaryMultipartPartStreamInput{}, status.Error(codes.InvalidArgument, "metadata is required")
	}
	if err != nil {
		return uploadBinaryMultipartPartStreamInput{}, status.Error(codes.InvalidArgument, "failed to receive metadata")
	}
	if first.WhichPayload() != pb.UploadBinaryMultipartPartRequest_Metadata_case {
		return uploadBinaryMultipartPartStreamInput{}, status.Error(
			codes.InvalidArgument, "first message must contain metadata",
		)
	}

	metadata := first.GetMetadata()
	in, err := uploadBinaryMultipartPartInputFromMetadata(userID, metadata)
	if err != nil {
		return uploadBinaryMultipartPartStreamInput{}, err
	}

	partReader, partWriter := io.Pipe()
	producerErrCh := make(chan error, 1)
	go func() {
		producerErrCh <- receiveBinaryMultipartPartChunks(stream, partWriter, in.PartSize)
	}()
	in.Data = partReader

	return uploadBinaryMultipartPartStreamInput{
		in:            in,
		partReader:    partReader,
		producerErrCh: producerErrCh,
	}, nil
}

// uploadBinaryMultipartPartInputFromMetadata валидирует метаданные части multipart-загрузки.
func uploadBinaryMultipartPartInputFromMetadata(
	userID uuid.UUID,
	metadata *pb.UploadBinaryMultipartPartMetadata,
) (usecase.UploadBinaryMultipartPartInput, error) {
	if metadata == nil {
		return usecase.UploadBinaryMultipartPartInput{}, status.Error(codes.InvalidArgument, "metadata is required")
	}
	uploadID, err := uuid.Parse(metadata.GetUploadId())
	if err != nil || uploadID == uuid.Nil {
		return usecase.UploadBinaryMultipartPartInput{}, status.Error(codes.InvalidArgument, "upload id is invalid")
	}
	if metadata.GetPartNumber() <= 0 {
		return usecase.UploadBinaryMultipartPartInput{}, status.Error(codes.InvalidArgument, "part number is invalid")
	}
	if metadata.GetPartSize() <= 0 {
		return usecase.UploadBinaryMultipartPartInput{}, status.Error(codes.InvalidArgument, "part size is invalid")
	}
	return usecase.UploadBinaryMultipartPartInput{
		UserID:     userID,
		UploadID:   uploadID,
		PartNumber: metadata.GetPartNumber(),
		PartSize:   metadata.GetPartSize(),
	}, nil
}

// receiveBinaryMultipartPartChunks принимает чанки части файла из стрима и записывает их в пайп.
func receiveBinaryMultipartPartChunks(
	stream pb.Records_UploadBinaryMultipartPartServer,
	partWriter *io.PipeWriter,
	expectedSize int64,
) error {
	var receivedSize int64
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			if receivedSize != expectedSize {
				_ = partWriter.CloseWithError(usecase.ErrBinaryEncryptedSizeMismatch)
				return usecase.ErrBinaryEncryptedSizeMismatch
			}
			return partWriter.Close()
		}
		if err != nil {
			closeErr := fmt.Errorf("failed to receive file part chunk: %w", errInvalidBinaryRecordStream)
			_ = partWriter.CloseWithError(errReadBinaryRecordStream)
			return closeErr
		}
		if req.WhichPayload() != pb.UploadBinaryMultipartPartRequest_Chunk_case {
			_ = partWriter.CloseWithError(errReadBinaryRecordStream)
			return errInvalidBinaryRecordStream
		}

		chunk := req.GetChunk()
		receivedSize += int64(len(chunk))
		if receivedSize > expectedSize {
			_ = partWriter.CloseWithError(usecase.ErrBinaryEncryptedSizeMismatch)
			return usecase.ErrBinaryEncryptedSizeMismatch
		}
		if _, err = partWriter.Write(chunk); err != nil {
			_ = partWriter.CloseWithError(err)
			return err
		}
	}
}

// waitBinaryMultipartPartChunksProducer дожидается завершения горутины, принимающей чанки части файла.
func waitBinaryMultipartPartChunksProducer(streamInput uploadBinaryMultipartPartStreamInput) error {
	if streamInput.producerErrCh == nil {
		return nil
	}
	err := <-streamInput.producerErrCh
	return err
}

// CompleteBinaryMultipartUpload завершает multipart-загрузку и переводит файл приватной записи в uploaded.
func (s *RecordsService) CompleteBinaryMultipartUpload(
	ctx context.Context,
	req *pb.CompleteBinaryMultipartUploadRequest,
) (*pb.CompleteBinaryMultipartUploadResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	userID, uploadID, err := multipartUploadIDFromRequest(ctx, req.GetUploadId())
	if err != nil {
		return nil, err
	}

	out, err := s.uc.CompleteBinaryMultipartUpload(ctx, usecase.CompleteBinaryMultipartUploadInput{
		UserID:   userID,
		UploadID: uploadID,
	})
	if err != nil {
		return nil, s.multipartUploadStatusError("failed to complete binary multipart upload", err)
	}

	return pb.CompleteBinaryMultipartUploadResponse_builder{
		RecordId:     new(out.RecordID.String()),
		Version:      &out.Version,
		UploadStatus: new(uploadStatusToProto(out.UploadStatus)),
	}.Build(), nil
}

// AbortBinaryMultipartUpload отменяет multipart-загрузку файла.
func (s *RecordsService) AbortBinaryMultipartUpload(
	ctx context.Context,
	req *pb.AbortBinaryMultipartUploadRequest,
) (*pb.AbortBinaryMultipartUploadResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	userID, uploadID, err := multipartUploadIDFromRequest(ctx, req.GetUploadId())
	if err != nil {
		return nil, err
	}

	out, err := s.uc.AbortBinaryMultipartUpload(ctx, usecase.AbortBinaryMultipartUploadInput{
		UserID:   userID,
		UploadID: uploadID,
	})
	if err != nil {
		return nil, s.multipartUploadStatusError("failed to abort binary multipart upload", err)
	}

	return pb.AbortBinaryMultipartUploadResponse_builder{
		UploadId:     new(out.UploadID.String()),
		UploadStatus: new(uploadStatusToProto(out.UploadStatus)),
	}.Build(), nil
}

// multipartUploadIDFromRequest извлекает пользователя из контекста и валидирует ID multipart-загрузки.
func multipartUploadIDFromRequest(ctx context.Context, uploadIDValue string) (uuid.UUID, uuid.UUID, error) {
	userID, ok := authcontext.UserIDFromContext(ctx)
	if !ok {
		return uuid.Nil, uuid.Nil, status.Error(codes.Unauthenticated, "authentication is required")
	}
	uploadID, err := uuid.Parse(uploadIDValue)
	if err != nil || uploadID == uuid.Nil {
		return uuid.Nil, uuid.Nil, status.Error(codes.InvalidArgument, "upload id is invalid")
	}
	return userID, uploadID, nil
}

func (s *RecordsService) multipartUploadStatusError(logMessage string, err error) error {
	if errors.Is(err, usecase.ErrMultipartUploadNotFound) {
		return status.Error(codes.NotFound, "multipart upload not found")
	}
	if errors.Is(err, usecase.ErrMultipartUploadNotActive) {
		return status.Error(codes.FailedPrecondition, "multipart upload is not active")
	}
	if errors.Is(err, usecase.ErrMultipartUploadIncomplete) {
		return status.Error(codes.FailedPrecondition, "multipart upload is incomplete")
	}
	if errors.Is(err, usecase.ErrMultipartUploadPartInvalid) ||
		errors.Is(err, usecase.ErrBinaryEncryptedSizeMismatch) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	if errors.Is(err, usecase.ErrRecordNotFound) {
		return status.Error(codes.NotFound, "record not found")
	}
	if errors.Is(err, usecase.ErrRecordVersionConflict) {
		return status.Error(codes.Aborted, "record version conflict")
	}
	if errors.Is(err, usecase.ErrRecordIsNotBinary) ||
		errors.Is(err, usecase.ErrRecordFileIsNotUploaded) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	s.logger.Error(logMessage, slog.Any("err", err))
	return status.Error(codes.Internal, "internal error")
}

func multipartPartsToProto(parts []usecase.MultipartUploadPartOutput) []*pb.MultipartUploadPart {
	out := make([]*pb.MultipartUploadPart, 0, len(parts))
	for _, part := range parts {
		out = append(out, multipartPartToProto(part))
	}
	return out
}

func multipartPartToProto(part usecase.MultipartUploadPartOutput) *pb.MultipartUploadPart {
	return pb.MultipartUploadPart_builder{
		PartNumber: &part.PartNumber,
		Size:       &part.Size,
		Etag:       &part.ETag,
	}.Build()
}

// ListRecords возвращает список приватных записей.
func (s *RecordsService) ListRecords(ctx context.Context, req *pb.ListRecordsRequest) (*pb.ListRecordsResponse, error) {
	in, err := listRecordsInputFromRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	out, err := s.uc.ListRecords(ctx, in)
	if err != nil {
		s.logger.Error("failed to list records", slog.Any("err", err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	items := make([]*pb.RecordListItem, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, recordListItemToProto(item))
	}
	return pb.ListRecordsResponse_builder{Items: items}.Build(), nil
}

// listRecordsInputFromRequest валидирует gRPC-запрос и преобразует его во входной DTO сценария получения списка
// приватных записей.
func listRecordsInputFromRequest(ctx context.Context, req *pb.ListRecordsRequest) (usecase.ListRecordsInput, error) {
	if req == nil {
		return usecase.ListRecordsInput{}, status.Error(codes.InvalidArgument, "request is required")
	}
	userID, ok := authcontext.UserIDFromContext(ctx)
	if !ok {
		return usecase.ListRecordsInput{}, status.Error(codes.Unauthenticated, "authentication is required")
	}
	return usecase.ListRecordsInput{UserID: userID}, nil
}

// recordListItemToProto преобразует краткое представление приватной записи в protobuf-модель.
func recordListItemToProto(item model.RecordListItem) *pb.RecordListItem {
	createdAt := timestamppb.New(item.CreatedAt)
	updatedAt := timestamppb.New(item.UpdatedAt)

	return pb.RecordListItem_builder{
		RecordId:    new(item.ID.String()),
		Type:        new(recordTypeToProto(item.Type)),
		Title:       &item.Title,
		Description: &item.Description,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		File:        recordListItemFileToProto(item.File),
	}.Build()
}

// recordTypeToProto преобразует доменный тип приватной записи в ее protobuf-тип.
func recordTypeToProto(recordType model.RecordType) pb.RecordType {
	switch recordType {
	case model.RecordTypeCredential:
		return pb.RecordType_RECORD_TYPE_CREDENTIAL
	case model.RecordTypeText:
		return pb.RecordType_RECORD_TYPE_TEXT
	case model.RecordTypeCard:
		return pb.RecordType_RECORD_TYPE_CARD
	case model.RecordTypeBinary:
		return pb.RecordType_RECORD_TYPE_BINARY
	default:
		return pb.RecordType_RECORD_TYPE_UNSPECIFIED
	}
}

// recordListItemFileToProto преобразует краткое представление файла приватной записи в protobuf-модель.
func recordListItemFileToProto(file *model.RecordListItemFile) *pb.RecordFile {
	if file == nil {
		return nil
	}
	return pb.RecordFile_builder{UploadStatus: new(uploadStatusToProto(file.UploadStatus))}.Build()
}

// uploadStatusToProto преобразует доменный статус загрузки файла в protobuf-статус.
func uploadStatusToProto(uploadStatus model.UploadStatus) pb.UploadStatus {
	switch uploadStatus {
	case model.UploadStatusUploading:
		return pb.UploadStatus_UPLOAD_STATUS_UPLOADING
	case model.UploadStatusUploaded:
		return pb.UploadStatus_UPLOAD_STATUS_UPLOADED
	case model.UploadStatusFailed:
		return pb.UploadStatus_UPLOAD_STATUS_FAILED
	default:
		return pb.UploadStatus_UPLOAD_STATUS_UNSPECIFIED
	}
}

// GetRecord возвращает приватную запись.
func (s *RecordsService) GetRecord(ctx context.Context, req *pb.GetRecordRequest) (*pb.GetRecordResponse, error) {
	in, err := getRecordInputFromRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	out, err := s.uc.GetRecord(ctx, in)
	if err != nil {
		if errors.Is(err, usecase.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "record not found")
		}
		s.logger.Error("failed to get record", slog.Any("err", err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	return pb.GetRecordResponse_builder{Record: recordToProto(out.Record)}.Build(), nil
}

// getRecordInputFromRequest валидирует gRPC-запрос и преобразует его во входной DTO сценария получения приватной
// записи.
func getRecordInputFromRequest(ctx context.Context, req *pb.GetRecordRequest) (usecase.GetRecordInput, error) {
	if req == nil {
		return usecase.GetRecordInput{}, status.Error(codes.InvalidArgument, "request is required")
	}
	userID, ok := authcontext.UserIDFromContext(ctx)
	if !ok {
		return usecase.GetRecordInput{}, status.Error(codes.Unauthenticated, "authentication is required")
	}
	recordID, err := uuid.Parse(req.GetRecordId())
	if err != nil || recordID == uuid.Nil {
		return usecase.GetRecordInput{}, status.Error(codes.InvalidArgument, "record id is invalid")
	}
	return usecase.GetRecordInput{RecordID: recordID, UserID: userID}, nil
}

// recordToProto преобразует доменную модель приватной записи в protobuf-модель.
func recordToProto(record model.Record) *pb.Record {
	createdAt := timestamppb.New(record.CreatedAt)
	updatedAt := timestamppb.New(record.UpdatedAt)

	return pb.Record_builder{
		RecordId:         new(record.ID.String()),
		Type:             new(recordTypeToProto(record.Type)),
		Title:            &record.Title,
		Description:      &record.Description,
		EncryptedDek:     record.EncryptedDEK.Data,
		EncryptedPayload: record.EncryptedPayload.Data,
		Version:          &record.Version,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
		DeletedAt:        timeToProto(record.DeletedAt),
		File:             recordFileToProto(record.File),
	}.Build()
}

// timeToProto преобразует опциональное время в штамп времени protobuf.
func timeToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// recordFileToProto преобразует технические данные файла приватной записи в protobuf-модель.
func recordFileToProto(file *model.RecordFile) *pb.RecordFile {
	if file == nil {
		return nil
	}
	return pb.RecordFile_builder{UploadStatus: new(uploadStatusToProto(file.UploadStatus))}.Build()
}

// UpdateRecord обновляет приватную запись.
func (s *RecordsService) UpdateRecord(
	ctx context.Context,
	req *pb.UpdateRecordRequest,
) (*pb.UpdateRecordResponse, error) {
	in, err := updateRecordInputFromRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	out, err := s.uc.UpdateRecord(ctx, in)
	if err != nil {
		if errors.Is(err, usecase.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "record not found")
		}
		if errors.Is(err, usecase.ErrRecordVersionConflict) {
			return nil, status.Error(codes.Aborted, "record version conflict")
		}
		s.logger.Error("failed to update record", slog.Any("err", err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	return pb.UpdateRecordResponse_builder{
		RecordId: new(out.RecordID.String()),
		Version:  &out.Version,
	}.Build(), nil
}

// updateRecordInputFromRequest валидирует gRPC-запрос и преобразует его во входной DTO сценария обновления приватной
// записи.
func updateRecordInputFromRequest(ctx context.Context, req *pb.UpdateRecordRequest) (usecase.UpdateRecordInput, error) {
	if req == nil {
		return usecase.UpdateRecordInput{}, status.Error(codes.InvalidArgument, "request is required")
	}
	userID, ok := authcontext.UserIDFromContext(ctx)
	if !ok {
		return usecase.UpdateRecordInput{}, status.Error(codes.Unauthenticated, "authentication is required")
	}
	recordID, err := uuid.Parse(req.GetRecordId())
	if err != nil || recordID == uuid.Nil {
		return usecase.UpdateRecordInput{}, status.Error(codes.InvalidArgument, "record id is invalid")
	}
	if strings.TrimSpace(req.GetTitle()) == "" {
		return usecase.UpdateRecordInput{}, status.Error(codes.InvalidArgument, "title is required")
	}
	if len(req.GetEncryptedDek()) == 0 {
		return usecase.UpdateRecordInput{}, status.Error(codes.InvalidArgument, "encrypted dek is required")
	}
	if len(req.GetEncryptedPayload()) == 0 {
		return usecase.UpdateRecordInput{}, status.Error(codes.InvalidArgument, "encrypted payload is required")
	}
	if err := validateRecordDataSize(
		req.GetTitle(),
		req.GetDescription(),
		req.GetEncryptedDek(),
		req.GetEncryptedPayload(),
	); err != nil {
		return usecase.UpdateRecordInput{}, err
	}
	if req.GetExpectedVersion() <= 0 {
		return usecase.UpdateRecordInput{}, status.Error(codes.InvalidArgument, "expected version is invalid")
	}

	return usecase.UpdateRecordInput{
		RecordID:         recordID,
		UserID:           userID,
		Title:            req.GetTitle(),
		Description:      req.GetDescription(),
		EncryptedDEK:     req.GetEncryptedDek(),
		EncryptedPayload: req.GetEncryptedPayload(),
		ExpectedVersion:  req.GetExpectedVersion(),
	}, nil
}

// DeleteRecord удаляет приватную запись.
func (s *RecordsService) DeleteRecord(
	ctx context.Context,
	req *pb.DeleteRecordRequest,
) (*pb.DeleteRecordResponse, error) {
	in, err := deleteRecordInputFromRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	out, err := s.uc.DeleteRecord(ctx, in)
	if err != nil {
		if errors.Is(err, usecase.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "record not found")
		}
		s.logger.Error("failed to delete record", slog.Any("err", err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	return pb.DeleteRecordResponse_builder{RecordId: new(out.RecordID.String())}.Build(), nil
}

// deleteRecordInputFromRequest валидирует gRPC-запрос и преобразует его во входной DTO сценария удаления приватной
// записи.
func deleteRecordInputFromRequest(ctx context.Context, req *pb.DeleteRecordRequest) (usecase.DeleteRecordInput, error) {
	if req == nil {
		return usecase.DeleteRecordInput{}, status.Error(codes.InvalidArgument, "request is required")
	}
	userID, ok := authcontext.UserIDFromContext(ctx)
	if !ok {
		return usecase.DeleteRecordInput{}, status.Error(codes.Unauthenticated, "authentication is required")
	}
	recordID, err := uuid.Parse(req.GetRecordId())
	if err != nil || recordID == uuid.Nil {
		return usecase.DeleteRecordInput{}, status.Error(codes.InvalidArgument, "record id is invalid")
	}
	return usecase.DeleteRecordInput{RecordID: recordID, UserID: userID}, nil
}

// DownloadFile возвращает зашифрованный файл приватной записи чанками фиксированного размера.
func (s *RecordsService) DownloadFile(req *pb.DownloadFileRequest, stream pb.Records_DownloadFileServer) error {
	in, err := downloadFileInputFromRequest(stream.Context(), req)
	if err != nil {
		return err
	}

	out, err := s.uc.DownloadFile(stream.Context(), in)
	if err != nil {
		if errors.Is(err, usecase.ErrRecordNotFound) {
			return status.Error(codes.NotFound, "record not found")
		}
		if errors.Is(err, usecase.ErrRecordIsNotBinary) {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, usecase.ErrRecordFileIsNotUploaded) {
			return status.Error(codes.FailedPrecondition, err.Error())
		}
		s.logger.Error("failed to download file", slog.Any("err", err))
		return status.Error(codes.Internal, "internal error")
	}
	if out.EncryptedFile == nil {
		s.logger.Error("failed to download file", slog.Any("err", "encrypted file reader is not provided"))
		return status.Error(codes.Internal, "internal error")
	}
	defer func() {
		if err = out.EncryptedFile.Close(); err != nil {
			s.logger.Error("failed to close encrypted file reader", slog.Any("err", err))
		}
	}()

	buffer := make([]byte, downloadChunkSizeBytes)
	for {
		n, readErr := out.EncryptedFile.Read(buffer)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buffer[:n])
			if err = stream.Send(pb.DownloadFileResponse_builder{Chunk: chunk}.Build()); err != nil {
				s.logger.Error("failed to send encrypted file chunk", slog.Any("err", err))
				return status.Error(codes.Internal, "internal error")
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			s.logger.Error("failed to read encrypted file", slog.Any("err", readErr))
			return status.Error(codes.Internal, "internal error")
		}
	}
}

// downloadFileInputFromRequest валидирует gRPC-запрос и преобразует его во входной DTO сценария скачивания файла.
func downloadFileInputFromRequest(ctx context.Context, req *pb.DownloadFileRequest) (usecase.DownloadFileInput, error) {
	if req == nil {
		return usecase.DownloadFileInput{}, status.Error(codes.InvalidArgument, "request is required")
	}
	userID, ok := authcontext.UserIDFromContext(ctx)
	if !ok {
		return usecase.DownloadFileInput{}, status.Error(codes.Unauthenticated, "authentication is required")
	}
	recordID, err := uuid.Parse(req.GetRecordId())
	if err != nil || recordID == uuid.Nil {
		return usecase.DownloadFileInput{}, status.Error(codes.InvalidArgument, "record id is invalid")
	}
	return usecase.DownloadFileInput{RecordID: recordID, UserID: userID}, nil
}
