package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/logging"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/transport/grpcserver/authcontext"
	"zerogravity-82/goph-keeper/internal/usecase"
)

type recordsUseCase interface {
	CreateRecord(ctx context.Context, in usecase.CreateRecordInput) (usecase.CreateRecordOutput, error)
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
