package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

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

const downloadChunkSize = 64 * 1024

// recordsUseCase описывает сценарии работы с приватными записями, которые нужны gRPC-сервису.
type recordsUseCase interface {
	CreateRecord(ctx context.Context, in usecase.CreateRecordInput) (usecase.CreateRecordOutput, error)
	CreateBinaryRecord(ctx context.Context, in usecase.CreateBinaryRecordInput) (usecase.CreateBinaryRecordOutput, error)
	ListRecords(ctx context.Context, in usecase.ListRecordsInput) (usecase.ListRecordsOutput, error)
	GetRecord(ctx context.Context, in usecase.GetRecordInput) (usecase.GetRecordOutput, error)
	UpdateRecord(ctx context.Context, in usecase.UpdateRecordInput) (usecase.UpdateRecordOutput, error)
	UpdateBinaryRecord(ctx context.Context, in usecase.UpdateBinaryRecordInput) (usecase.UpdateBinaryRecordOutput, error)
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
			return nil, status.Error(codes.InvalidArgument, "binary record requires CreateBinaryRecord")
		}
		s.logger.Error("failed to create record", slog.Any("err", err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	recordID := out.RecordID.String()
	return pb.CreateRecordResponse_builder{
		RecordId: &recordID,
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

	return usecase.CreateRecordInput{
		UserID:           userID,
		Type:             recordType,
		Title:            req.GetTitle(),
		Description:      req.GetDescription(),
		EncryptedDEK:     req.GetEncryptedDek(),
		EncryptedPayload: req.GetEncryptedPayload(),
	}, nil
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

// CreateBinaryRecord создает бинарную приватную запись вместе с загрузкой зашифрованного файла в хранилище.
func (s *RecordsService) CreateBinaryRecord(stream pb.Records_CreateBinaryRecordServer) error {
	streamInput, err := createBinaryRecordInputFromStream(stream)
	if err != nil {
		return err
	}

	out, usecaseErr := s.uc.CreateBinaryRecord(stream.Context(), streamInput.in)
	if usecaseErr != nil {
		_ = streamInput.fileReader.CloseWithError(usecaseErr)
	} else {
		_ = streamInput.fileReader.Close()
	}

	// Продюсер-горутина читает чанки из gRPC-стрима и пишет их в пайп, а usecase читает данные из пайпа.
	// После завершения usecase нужно дождаться продюсера, чтобы не потерять ошибку чтения стрима, а также чтобы
	// не оставить горутину после ответа клиенту.
	producerErr := waitBinaryRecordChunksProducer(streamInput)
	if producerErr != nil {
		if errors.Is(producerErr, usecase.ErrBinaryEncryptedSizeMismatch) ||
			errors.Is(producerErr, errInvalidBinaryRecordStream) {
			return status.Error(codes.InvalidArgument, producerErr.Error())
		}
		if usecaseErr == nil {
			s.logger.Error("failed to receive binary record chunks", slog.Any("err", producerErr))
			return status.Error(codes.Internal, "internal error")
		}
	}
	if usecaseErr != nil {
		if errors.Is(usecaseErr, usecase.ErrInvalidBinaryEncryptedSize) ||
			errors.Is(usecaseErr, usecase.ErrBinaryEncryptedSizeMismatch) ||
			errors.Is(usecaseErr, usecase.ErrUploadModeNotSupported) {
			return status.Error(codes.InvalidArgument, usecaseErr.Error())
		}
		s.logger.Error("failed to create binary record", slog.Any("err", usecaseErr))
		return status.Error(codes.Internal, "internal error")
	}

	recordID := out.RecordID.String()
	uploadStatus := uploadStatusToProto(out.UploadStatus)
	return stream.SendAndClose(pb.CreateBinaryRecordResponse_builder{
		RecordId:     &recordID,
		Version:      &out.Version,
		UploadStatus: &uploadStatus,
	}.Build())
}

// createBinaryRecordStreamInput содержит входные данные сценария и служебные объекты для чтения файла из стрима.
type createBinaryRecordStreamInput struct {
	in            usecase.CreateBinaryRecordInput
	fileReader    *io.PipeReader
	producerErrCh <-chan error
}

// createBinaryRecordInputFromStream читает первое сообщение стрима, валидирует метаданные и готовит пайп для файла.
//
// Первое сообщение должно содержать метаданные, потому что серверу нужны параметры приватной записи и файла до чтения
// чанков. Все последующие сообщения должны содержать чанк с частью зашифрованного файла. Сервер передает чанки в
// файловое хранилище потоково, не дожидаясь загрузки всего файла.
func createBinaryRecordInputFromStream(
	stream pb.Records_CreateBinaryRecordServer,
) (createBinaryRecordStreamInput, error) {
	userID, ok := authcontext.UserIDFromContext(stream.Context())
	if !ok {
		return createBinaryRecordStreamInput{}, status.Error(codes.Unauthenticated, "authentication is required")
	}

	first, err := stream.Recv()
	if errors.Is(err, io.EOF) {
		return createBinaryRecordStreamInput{}, status.Error(codes.InvalidArgument, "metadata is required")
	}
	if err != nil {
		return createBinaryRecordStreamInput{}, status.Error(codes.InvalidArgument, "failed to receive metadata")
	}
	if first.WhichPayload() != pb.CreateBinaryRecordRequest_Metadata_case {
		return createBinaryRecordStreamInput{}, status.Error(
			codes.InvalidArgument, "first message must contain metadata",
		)
	}
	metadata := first.GetMetadata()
	in, err := createBinaryRecordInputFromMetadata(userID, metadata)
	if err != nil {
		return createBinaryRecordStreamInput{}, err
	}

	// Пайп связывает чтение чанков из gRPC-стрима с io.Reader, который потом usecase передает в файловое хранилище.
	fileReader, fileWriter := io.Pipe()
	producerErrCh := make(chan error, 1)
	go func() {
		producerErrCh <- receiveBinaryRecordChunks(stream, fileWriter, in.EncryptedSize)
	}()
	in.EncryptedFile = fileReader

	return createBinaryRecordStreamInput{
		in:            in,
		fileReader:    fileReader,
		producerErrCh: producerErrCh,
	}, nil
}

// createBinaryRecordInputFromMetadata валидирует метаданные бинарной приватной записи и преобразует их во входной
// DTO сценария создания бинарной приватной записи.
func createBinaryRecordInputFromMetadata(
	userID uuid.UUID,
	metadata *pb.CreateBinaryRecordMetadata,
) (usecase.CreateBinaryRecordInput, error) {
	if metadata == nil {
		return usecase.CreateBinaryRecordInput{}, status.Error(codes.InvalidArgument, "metadata is required")
	}
	if strings.TrimSpace(metadata.GetTitle()) == "" {
		return usecase.CreateBinaryRecordInput{}, status.Error(codes.InvalidArgument, "title is required")
	}
	if len(metadata.GetEncryptedDek()) == 0 {
		return usecase.CreateBinaryRecordInput{}, status.Error(codes.InvalidArgument, "encrypted dek is required")
	}
	if len(metadata.GetEncryptedPayload()) == 0 {
		return usecase.CreateBinaryRecordInput{}, status.Error(codes.InvalidArgument, "encrypted payload is required")
	}
	uploadMode, ok := uploadModeFromProto(metadata.GetUploadMode())
	if !ok {
		return usecase.CreateBinaryRecordInput{}, status.Error(codes.InvalidArgument, "upload mode is invalid")
	}
	return usecase.CreateBinaryRecordInput{
		UserID:           userID,
		Title:            metadata.GetTitle(),
		Description:      metadata.GetDescription(),
		EncryptedDEK:     metadata.GetEncryptedDek(),
		EncryptedPayload: metadata.GetEncryptedPayload(),
		EncryptedSize:    metadata.GetEncryptedSize(),
		UploadMode:       uploadMode,
	}, nil
}

// uploadModeFromProto преобразует protobuf-режим загрузки файла в доменный режим загрузки.
func uploadModeFromProto(uploadMode pb.UploadMode) (model.UploadMode, bool) {
	switch uploadMode {
	case pb.UploadMode_UPLOAD_MODE_SINGLE_PART:
		return model.UploadModeSinglePart, true
	case pb.UploadMode_UPLOAD_MODE_MULTIPART:
		return model.UploadModeMultiPart, true
	default:
		return "", false
	}
}

// receiveBinaryRecordChunks принимает чанки зашифрованного файла из стрима и записывает их в пайп, из которого
// читает usecase.
func receiveBinaryRecordChunks(
	stream pb.Records_CreateBinaryRecordServer,
	fileWriter *io.PipeWriter,
	expectedSize int64,
) error {
	var receivedSize int64
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			if receivedSize != expectedSize {
				_ = fileWriter.CloseWithError(usecase.ErrBinaryEncryptedSizeMismatch)
				return usecase.ErrBinaryEncryptedSizeMismatch
			}
			return fileWriter.Close()
		}
		if err != nil {
			closeErr := fmt.Errorf("failed to receive file chunk: %w", errInvalidBinaryRecordStream)
			_ = fileWriter.CloseWithError(errReadBinaryRecordStream)
			return closeErr
		}
		if req.WhichPayload() != pb.CreateBinaryRecordRequest_Chunk_case {
			_ = fileWriter.CloseWithError(errReadBinaryRecordStream)
			return errInvalidBinaryRecordStream
		}

		chunk := req.GetChunk()
		receivedSize += int64(len(chunk))
		if receivedSize > int64(usecase.MaxEncryptedFileSize()) || receivedSize > expectedSize {
			_ = fileWriter.CloseWithError(usecase.ErrBinaryEncryptedSizeMismatch)
			return usecase.ErrBinaryEncryptedSizeMismatch
		}
		if _, err = fileWriter.Write(chunk); err != nil {
			_ = fileWriter.CloseWithError(err)
			return err
		}
	}
}

// waitBinaryRecordChunksProducer дожидается завершения горутины, принимающей чанки файла.
func waitBinaryRecordChunksProducer(streamInput createBinaryRecordStreamInput) error {
	if streamInput.producerErrCh == nil {
		return nil
	}
	err := <-streamInput.producerErrCh
	return err
}

// UpdateBinaryRecord обновляет бинарную приватную запись вместе с заменой зашифрованного файла в хранилище.
func (s *RecordsService) UpdateBinaryRecord(stream pb.Records_UpdateBinaryRecordServer) error {
	streamInput, err := updateBinaryRecordInputFromStream(stream)
	if err != nil {
		return err
	}

	out, usecaseErr := s.uc.UpdateBinaryRecord(stream.Context(), streamInput.in)
	if usecaseErr != nil {
		_ = streamInput.fileReader.CloseWithError(usecaseErr)
	} else {
		_ = streamInput.fileReader.Close()
	}

	producerErr := waitUpdateBinaryRecordChunksProducer(streamInput)
	if producerErr != nil {
		if errors.Is(producerErr, usecase.ErrBinaryEncryptedSizeMismatch) ||
			errors.Is(producerErr, errInvalidBinaryRecordStream) {
			return status.Error(codes.InvalidArgument, producerErr.Error())
		}
		if usecaseErr == nil {
			s.logger.Error("failed to receive binary record chunks", slog.Any("err", producerErr))
			return status.Error(codes.Internal, "internal error")
		}
	}
	if usecaseErr != nil {
		if errors.Is(usecaseErr, usecase.ErrRecordNotFound) {
			return status.Error(codes.NotFound, "record not found")
		}
		if errors.Is(usecaseErr, usecase.ErrRecordVersionConflict) {
			return status.Error(codes.Aborted, "record version conflict")
		}
		if errors.Is(usecaseErr, usecase.ErrRecordIsNotBinary) ||
			errors.Is(usecaseErr, usecase.ErrInvalidBinaryEncryptedSize) ||
			errors.Is(usecaseErr, usecase.ErrBinaryEncryptedSizeMismatch) ||
			errors.Is(usecaseErr, usecase.ErrUploadModeNotSupported) {
			return status.Error(codes.InvalidArgument, usecaseErr.Error())
		}
		s.logger.Error("failed to update binary record", slog.Any("err", usecaseErr))
		return status.Error(codes.Internal, "internal error")
	}

	recordID := out.RecordID.String()
	uploadStatus := uploadStatusToProto(out.UploadStatus)
	return stream.SendAndClose(pb.UpdateBinaryRecordResponse_builder{
		RecordId:     &recordID,
		Version:      &out.Version,
		UploadStatus: &uploadStatus,
	}.Build())
}

// updateBinaryRecordStreamInput содержит входные данные сценария и служебные объекты для чтения файла из стрима.
type updateBinaryRecordStreamInput struct {
	in            usecase.UpdateBinaryRecordInput
	fileReader    *io.PipeReader
	producerErrCh <-chan error
}

// updateBinaryRecordInputFromStream читает первое сообщение стрима, валидирует метаданные и готовит пайп для файла.
func updateBinaryRecordInputFromStream(
	stream pb.Records_UpdateBinaryRecordServer,
) (updateBinaryRecordStreamInput, error) {
	userID, ok := authcontext.UserIDFromContext(stream.Context())
	if !ok {
		return updateBinaryRecordStreamInput{}, status.Error(codes.Unauthenticated, "authentication is required")
	}

	first, err := stream.Recv()
	if errors.Is(err, io.EOF) {
		return updateBinaryRecordStreamInput{}, status.Error(codes.InvalidArgument, "metadata is required")
	}
	if err != nil {
		return updateBinaryRecordStreamInput{}, status.Error(codes.InvalidArgument, "failed to receive metadata")
	}
	if first.WhichPayload() != pb.UpdateBinaryRecordRequest_Metadata_case {
		return updateBinaryRecordStreamInput{}, status.Error(
			codes.InvalidArgument, "first message must contain metadata",
		)
	}
	metadata := first.GetMetadata()
	in, err := updateBinaryRecordInputFromMetadata(userID, metadata)
	if err != nil {
		return updateBinaryRecordStreamInput{}, err
	}

	fileReader, fileWriter := io.Pipe()
	producerErrCh := make(chan error, 1)
	go func() {
		producerErrCh <- receiveUpdatedBinaryRecordChunks(stream, fileWriter, in.EncryptedSize)
	}()
	in.EncryptedFile = fileReader

	return updateBinaryRecordStreamInput{
		in:            in,
		fileReader:    fileReader,
		producerErrCh: producerErrCh,
	}, nil
}

// updateBinaryRecordInputFromMetadata валидирует метаданные и преобразует их во входной DTO сценария.
func updateBinaryRecordInputFromMetadata(
	userID uuid.UUID,
	metadata *pb.UpdateBinaryRecordMetadata,
) (usecase.UpdateBinaryRecordInput, error) {
	if metadata == nil {
		return usecase.UpdateBinaryRecordInput{}, status.Error(codes.InvalidArgument, "metadata is required")
	}
	recordID, err := uuid.Parse(metadata.GetRecordId())
	if err != nil || recordID == uuid.Nil {
		return usecase.UpdateBinaryRecordInput{}, status.Error(codes.InvalidArgument, "record id is invalid")
	}
	if strings.TrimSpace(metadata.GetTitle()) == "" {
		return usecase.UpdateBinaryRecordInput{}, status.Error(codes.InvalidArgument, "title is required")
	}
	if len(metadata.GetEncryptedDek()) == 0 {
		return usecase.UpdateBinaryRecordInput{}, status.Error(codes.InvalidArgument, "encrypted dek is required")
	}
	if len(metadata.GetEncryptedPayload()) == 0 {
		return usecase.UpdateBinaryRecordInput{}, status.Error(codes.InvalidArgument, "encrypted payload is required")
	}
	uploadMode, ok := uploadModeFromProto(metadata.GetUploadMode())
	if !ok {
		return usecase.UpdateBinaryRecordInput{}, status.Error(codes.InvalidArgument, "upload mode is invalid")
	}
	if metadata.GetExpectedVersion() <= 0 {
		return usecase.UpdateBinaryRecordInput{}, status.Error(codes.InvalidArgument, "expected version is invalid")
	}
	return usecase.UpdateBinaryRecordInput{
		RecordID:         recordID,
		UserID:           userID,
		Title:            metadata.GetTitle(),
		Description:      metadata.GetDescription(),
		EncryptedDEK:     metadata.GetEncryptedDek(),
		EncryptedPayload: metadata.GetEncryptedPayload(),
		EncryptedSize:    metadata.GetEncryptedSize(),
		UploadMode:       uploadMode,
		ExpectedVersion:  metadata.GetExpectedVersion(),
	}, nil
}

// receiveUpdatedBinaryRecordChunks принимает чанки нового зашифрованного файла из стрима и записывает их в пайп.
func receiveUpdatedBinaryRecordChunks(
	stream pb.Records_UpdateBinaryRecordServer,
	fileWriter *io.PipeWriter,
	expectedSize int64,
) error {
	var receivedSize int64
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			if receivedSize != expectedSize {
				_ = fileWriter.CloseWithError(usecase.ErrBinaryEncryptedSizeMismatch)
				return usecase.ErrBinaryEncryptedSizeMismatch
			}
			return fileWriter.Close()
		}
		if err != nil {
			closeErr := fmt.Errorf("failed to receive file chunk: %w", errInvalidBinaryRecordStream)
			_ = fileWriter.CloseWithError(errReadBinaryRecordStream)
			return closeErr
		}
		if req.WhichPayload() != pb.UpdateBinaryRecordRequest_Chunk_case {
			_ = fileWriter.CloseWithError(errReadBinaryRecordStream)
			return errInvalidBinaryRecordStream
		}

		chunk := req.GetChunk()
		receivedSize += int64(len(chunk))
		if receivedSize > int64(usecase.MaxEncryptedFileSize()) || receivedSize > expectedSize {
			_ = fileWriter.CloseWithError(usecase.ErrBinaryEncryptedSizeMismatch)
			return usecase.ErrBinaryEncryptedSizeMismatch
		}
		if _, err = fileWriter.Write(chunk); err != nil {
			_ = fileWriter.CloseWithError(err)
			return err
		}
	}
}

// waitUpdateBinaryRecordChunksProducer дожидается завершения горутины, принимающей чанки нового файла.
func waitUpdateBinaryRecordChunksProducer(streamInput updateBinaryRecordStreamInput) error {
	if streamInput.producerErrCh == nil {
		return nil
	}
	err := <-streamInput.producerErrCh
	return err
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
	recordID := item.ID.String()
	recordType := recordTypeToProto(item.Type)
	createdAt := timestamppb.New(item.CreatedAt)
	updatedAt := timestamppb.New(item.UpdatedAt)

	return pb.RecordListItem_builder{
		RecordId:    &recordID,
		Type:        &recordType,
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
	uploadStatus := uploadStatusToProto(file.UploadStatus)
	return pb.RecordFile_builder{UploadStatus: &uploadStatus}.Build()
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
	recordID := record.ID.String()
	recordType := recordTypeToProto(record.Type)
	createdAt := timestamppb.New(record.CreatedAt)
	updatedAt := timestamppb.New(record.UpdatedAt)

	return pb.Record_builder{
		RecordId:         &recordID,
		Type:             &recordType,
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
	uploadStatus := uploadStatusToProto(file.UploadStatus)
	return pb.RecordFile_builder{UploadStatus: &uploadStatus}.Build()
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

	recordID := out.RecordID.String()
	return pb.UpdateRecordResponse_builder{
		RecordId: &recordID,
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

	recordID := out.RecordID.String()
	return pb.DeleteRecordResponse_builder{RecordId: &recordID}.Build(), nil
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

	buffer := make([]byte, downloadChunkSize)
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
