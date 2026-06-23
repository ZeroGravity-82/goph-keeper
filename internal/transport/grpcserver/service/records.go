package service

import (
	"context"
	"errors"
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

type recordsUseCase interface {
	CreateRecord(ctx context.Context, in usecase.CreateRecordInput) (usecase.CreateRecordOutput, error)
	ListRecords(ctx context.Context, in usecase.ListRecordsInput) (usecase.ListRecordsOutput, error)
	GetRecord(ctx context.Context, in usecase.GetRecordInput) (usecase.GetRecordOutput, error)
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

// CreateRecord создает приватную запись пользователя.
func (s *RecordsService) CreateRecord(ctx context.Context, req *pb.CreateRecordRequest) (*pb.CreateRecordResponse, error) {
	in, err := createRecordInputFromRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	out, err := s.uc.CreateRecord(ctx, in)
	if err != nil {
		s.logger.Error("failed to create record", slog.Any("err", err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	recordID := out.RecordID.String()
	return pb.CreateRecordResponse_builder{
		RecordId: &recordID,
		Version:  &out.Version,
	}.Build(), nil
}

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

// ListRecords возвращает список приватных записей пользователя.
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
func recordListItemFileToProto(file *model.RecordListItemFile) *pb.RecordFile {
	if file == nil {
		return nil
	}
	uploadStatus := uploadStatusToProto(file.UploadStatus)
	return pb.RecordFile_builder{UploadStatus: &uploadStatus}.Build()
}

func uploadStatusToProto(uploadStatus model.UploadStatus) pb.UploadStatus {
	switch uploadStatus {
	case model.UploadStatusPending:
		return pb.UploadStatus_UPLOAD_STATUS_PENDING
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

// GetRecord возвращает приватную запись пользователя.
func (s *RecordsService) GetRecord(ctx context.Context, req *pb.GetRecordRequest) (*pb.GetRecordResponse, error) {
	in, err := getRecordInputFromRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	out, err := s.uc.GetRecord(ctx, in)
	if err != nil {
		if errors.Is(err, model.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "record not found")
		}
		s.logger.Error("failed to get record", slog.Any("err", err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	return pb.GetRecordResponse_builder{Record: recordToProto(out.Record)}.Build(), nil
}

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

func timeToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func recordFileToProto(file *model.RecordFile) *pb.RecordFile {
	if file == nil {
		return nil
	}
	uploadStatus := uploadStatusToProto(file.UploadStatus)
	return pb.RecordFile_builder{UploadStatus: &uploadStatus}.Build()
}
