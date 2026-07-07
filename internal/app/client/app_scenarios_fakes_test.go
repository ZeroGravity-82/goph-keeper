package client

import (
	"context"
	"fmt"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"zerogravity-82/goph-keeper/internal/crypto"
	"zerogravity-82/goph-keeper/internal/pb"
)

type authClientFake struct {
	registerReq               *pb.RegisterRequest
	loginResp                 *pb.LoginResponse
	refreshReq                *pb.RefreshRequest
	refreshResp               *pb.RefreshResponse
	refreshError              error
	logoutReq                 *pb.LogoutRequest
	changeMasterKeyReq        *pb.ChangeMasterKeyRequest
	changeMasterKeyErr        error
	changeMasterKeyCalls      int
	changeMasterKeyUnauthOnce bool
	records                   *recordsClientFake
}

func (f *authClientFake) Register(
	_ context.Context,
	req *pb.RegisterRequest,
	_ ...grpc.CallOption,
) (*pb.RegisterResponse, error) {
	f.registerReq = req
	return pb.RegisterResponse_builder{
		AccessToken:   new("registered-access-token"),
		RefreshToken:  new("registered-refresh-token"),
		MasterKeySalt: req.GetMasterKeySalt(),
	}.Build(), nil
}

func (f *authClientFake) Login(
	context.Context,
	*pb.LoginRequest,
	...grpc.CallOption,
) (*pb.LoginResponse, error) {
	return f.loginResp, nil
}

func (f *authClientFake) Refresh(
	_ context.Context,
	req *pb.RefreshRequest,
	_ ...grpc.CallOption,
) (*pb.RefreshResponse, error) {
	f.refreshReq = req
	return f.refreshResp, f.refreshError
}

func (f *authClientFake) Logout(
	_ context.Context,
	req *pb.LogoutRequest,
	_ ...grpc.CallOption,
) (*pb.LogoutResponse, error) {
	f.logoutReq = req
	return pb.LogoutResponse_builder{}.Build(), nil
}

func (f *authClientFake) ChangeMasterKey(
	_ context.Context,
	req *pb.ChangeMasterKeyRequest,
	_ ...grpc.CallOption,
) (*pb.ChangeMasterKeyResponse, error) {
	f.changeMasterKeyCalls++
	f.changeMasterKeyReq = req
	if f.changeMasterKeyUnauthOnce && f.changeMasterKeyCalls == 1 {
		return nil, status.Error(codes.Unauthenticated, "access token is invalid")
	}
	if f.changeMasterKeyErr != nil {
		return nil, f.changeMasterKeyErr
	}
	if f.records != nil {
		for _, item := range req.GetRecords() {
			record := f.records.records[item.GetRecordId()]
			if record == nil {
				continue
			}
			f.records.records[item.GetRecordId()] = pb.Record_builder{
				RecordId:         new(record.GetRecordId()),
				Type:             new(record.GetType()),
				Title:            new(record.GetTitle()),
				Description:      new(record.GetDescription()),
				EncryptedDek:     item.GetEncryptedDek(),
				EncryptedPayload: record.GetEncryptedPayload(),
				Version:          new(record.GetVersion() + 1),
				CreatedAt:        record.GetCreatedAt(),
				UpdatedAt:        fixedUpdatedRecordTimestamp(),
				File:             record.GetFile(),
			}.Build()
		}
	}
	return pb.ChangeMasterKeyResponse_builder{
		AccessToken:  new("changed-master-key-access-token"),
		RefreshToken: new("changed-master-key-refresh-token"),
	}.Build(), nil
}

type recordsClientFake struct {
	nextID           int
	nextUploadID     int
	records          map[string]*pb.Record
	encryptedFiles   map[string][]byte
	multipartUploads map[string]*multipartUploadFake
	listCalls        int
	listUnauthOnce   bool
}

func newRecordsClientFake() *recordsClientFake {
	return &recordsClientFake{
		records:          make(map[string]*pb.Record),
		encryptedFiles:   make(map[string][]byte),
		multipartUploads: make(map[string]*multipartUploadFake),
	}
}

type multipartUploadFake struct {
	recordID      string
	version       int64
	encryptedSize int64
	partSize      int64
	parts         map[int32][]byte
}

func (f *recordsClientFake) nextRecordID() string {
	f.nextID++
	return fmt.Sprintf("record-%d", f.nextID)
}

func (f *recordsClientFake) nextMultipartUploadID() string {
	f.nextUploadID++
	return fmt.Sprintf("upload-%d", f.nextUploadID)
}

func fixedRecordTimestamp() *timestamppb.Timestamp {
	return timestamppb.New(time.Date(2026, time.July, 2, 10, 0, 0, 0, time.UTC))
}

func fixedUpdatedRecordTimestamp() *timestamppb.Timestamp {
	return timestamppb.New(time.Date(2026, time.July, 2, 10, 1, 0, 0, time.UTC))
}

func (f *recordsClientFake) CreateRecord(
	_ context.Context,
	req *pb.CreateRecordRequest,
	_ ...grpc.CallOption,
) (*pb.CreateRecordResponse, error) {
	recordID := f.nextRecordID()
	version := int64(1)
	now := fixedRecordTimestamp()
	f.records[recordID] = pb.Record_builder{
		RecordId:         &recordID,
		Type:             new(req.GetType()),
		Title:            new(req.GetTitle()),
		Description:      new(req.GetDescription()),
		EncryptedDek:     req.GetEncryptedDek(),
		EncryptedPayload: req.GetEncryptedPayload(),
		Version:          &version,
		CreatedAt:        now,
		UpdatedAt:        now,
	}.Build()
	return pb.CreateRecordResponse_builder{RecordId: &recordID, Version: &version}.Build(), nil
}

func (f *recordsClientFake) StartBinaryMultipartUpload(
	_ context.Context,
	req *pb.StartBinaryMultipartUploadRequest,
	_ ...grpc.CallOption,
) (*pb.StartBinaryMultipartUploadResponse, error) {
	recordID := req.GetRecordId()
	version := int64(1)
	now := fixedRecordTimestamp()
	updatedAt := now
	if recordID == "" {
		recordID = f.nextRecordID()
	} else {
		record, ok := f.records[recordID]
		if !ok {
			return nil, status.Error(codes.NotFound, "record not found")
		}
		if record.GetVersion() != req.GetExpectedVersion() {
			return nil, status.Error(codes.Aborted, "record version conflict")
		}
		version = record.GetVersion() + 1
		now = record.GetCreatedAt()
		updatedAt = fixedUpdatedRecordTimestamp()
	}
	uploadStatus := pb.UploadStatus_UPLOAD_STATUS_UPLOADING
	f.records[recordID] = pb.Record_builder{
		RecordId:         &recordID,
		Type:             new(pb.RecordType_RECORD_TYPE_BINARY),
		Title:            new(req.GetTitle()),
		Description:      new(req.GetDescription()),
		EncryptedDek:     req.GetEncryptedDek(),
		EncryptedPayload: req.GetEncryptedPayload(),
		Version:          &version,
		CreatedAt:        now,
		UpdatedAt:        updatedAt,
		File:             pb.RecordFile_builder{UploadStatus: &uploadStatus}.Build(),
	}.Build()
	uploadID := f.nextMultipartUploadID()
	partSize := req.GetPartSize()
	f.multipartUploads[uploadID] = &multipartUploadFake{
		recordID:      recordID,
		version:       version,
		encryptedSize: req.GetEncryptedSize(),
		partSize:      partSize,
		parts:         make(map[int32][]byte),
	}
	return pb.StartBinaryMultipartUploadResponse_builder{
		UploadId:     &uploadID,
		RecordId:     &recordID,
		Version:      &version,
		PartSize:     &partSize,
		UploadStatus: &uploadStatus,
	}.Build(), nil
}

func (f *recordsClientFake) GetBinaryMultipartUploadStatus(
	_ context.Context,
	req *pb.GetBinaryMultipartUploadStatusRequest,
	_ ...grpc.CallOption,
) (*pb.GetBinaryMultipartUploadStatusResponse, error) {
	upload, ok := f.multipartUploads[req.GetUploadId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "multipart upload not found")
	}
	uploadStatus := pb.UploadStatus_UPLOAD_STATUS_UPLOADING
	if _, ok = f.encryptedFiles[upload.recordID]; ok {
		uploadStatus = pb.UploadStatus_UPLOAD_STATUS_UPLOADED
	}
	return pb.GetBinaryMultipartUploadStatusResponse_builder{
		UploadId:      new(req.GetUploadId()),
		RecordId:      &upload.recordID,
		Version:       &upload.version,
		EncryptedSize: &upload.encryptedSize,
		PartSize:      &upload.partSize,
		UploadStatus:  &uploadStatus,
		UploadedParts: multipartUploadFakeParts(upload),
	}.Build(), nil
}

func (f *recordsClientFake) UploadBinaryMultipartPart(
	ctx context.Context,
	_ ...grpc.CallOption,
) (grpc.ClientStreamingClient[pb.UploadBinaryMultipartPartRequest, pb.UploadBinaryMultipartPartResponse], error) {
	stream := &clientStreamFake[pb.UploadBinaryMultipartPartRequest, pb.UploadBinaryMultipartPartResponse]{ctx: ctx}
	var partMetadata *pb.UploadBinaryMultipartPartMetadata
	var part []byte
	stream.send = func(req *pb.UploadBinaryMultipartPartRequest) error {
		switch req.WhichPayload() {
		case pb.UploadBinaryMultipartPartRequest_Metadata_case:
			partMetadata = req.GetMetadata()
		case pb.UploadBinaryMultipartPartRequest_Chunk_case:
			part = append(part, req.GetChunk()...)
		default:
			return status.Error(codes.InvalidArgument, "empty stream message")
		}
		return nil
	}
	stream.closeAndRecv = func() (*pb.UploadBinaryMultipartPartResponse, error) {
		if partMetadata == nil {
			return nil, status.Error(codes.InvalidArgument, "metadata is required")
		}
		upload, ok := f.multipartUploads[partMetadata.GetUploadId()]
		if !ok {
			return nil, status.Error(codes.NotFound, "multipart upload not found")
		}
		if int64(len(part)) != partMetadata.GetPartSize() {
			return nil, status.Error(codes.InvalidArgument, "part size mismatch")
		}
		upload.parts[partMetadata.GetPartNumber()] = append([]byte(nil), part...)
		partNumber := partMetadata.GetPartNumber()
		return pb.UploadBinaryMultipartPartResponse_builder{
			UploadId: new(partMetadata.GetUploadId()),
			Part: pb.MultipartUploadPart_builder{
				PartNumber: &partNumber,
				Size:       new(int64(len(part))),
				Etag:       new(fmt.Sprintf("etag-%d", partNumber)),
			}.Build(),
		}.Build(), nil
	}
	return stream, nil
}

func (f *recordsClientFake) CompleteBinaryMultipartUpload(
	_ context.Context,
	req *pb.CompleteBinaryMultipartUploadRequest,
	_ ...grpc.CallOption,
) (*pb.CompleteBinaryMultipartUploadResponse, error) {
	upload, ok := f.multipartUploads[req.GetUploadId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "multipart upload not found")
	}
	var encryptedFile []byte
	for partNumber := int32(1); ; partNumber++ {
		part, ok := upload.parts[partNumber]
		if !ok {
			break
		}
		encryptedFile = append(encryptedFile, part...)
		if int64(len(encryptedFile)) == upload.encryptedSize {
			break
		}
	}
	if int64(len(encryptedFile)) != upload.encryptedSize {
		return nil, status.Error(codes.FailedPrecondition, "multipart upload is incomplete")
	}
	f.encryptedFiles[upload.recordID] = encryptedFile
	record := f.records[upload.recordID]
	uploadStatus := pb.UploadStatus_UPLOAD_STATUS_UPLOADED
	f.records[upload.recordID] = pb.Record_builder{
		RecordId:         new(record.GetRecordId()),
		Type:             new(record.GetType()),
		Title:            new(record.GetTitle()),
		Description:      new(record.GetDescription()),
		EncryptedDek:     record.GetEncryptedDek(),
		EncryptedPayload: record.GetEncryptedPayload(),
		Version:          new(record.GetVersion()),
		CreatedAt:        record.GetCreatedAt(),
		UpdatedAt:        record.GetUpdatedAt(),
		File:             pb.RecordFile_builder{UploadStatus: &uploadStatus}.Build(),
	}.Build()
	return pb.CompleteBinaryMultipartUploadResponse_builder{
		RecordId:     &upload.recordID,
		Version:      &upload.version,
		UploadStatus: &uploadStatus,
	}.Build(), nil
}

func (f *recordsClientFake) AbortBinaryMultipartUpload(
	context.Context,
	*pb.AbortBinaryMultipartUploadRequest,
	...grpc.CallOption,
) (*pb.AbortBinaryMultipartUploadResponse, error) {
	return nil, status.Error(codes.Unimplemented, "multipart upload is not implemented in fake")
}

func multipartUploadFakeParts(upload *multipartUploadFake) []*pb.MultipartUploadPart {
	partNumbers := make([]int, 0, len(upload.parts))
	for partNumber := range upload.parts {
		partNumbers = append(partNumbers, int(partNumber))
	}
	sort.Ints(partNumbers)
	parts := make([]*pb.MultipartUploadPart, 0, len(partNumbers))
	for _, partNumber := range partNumbers {
		number := int32(partNumber)
		parts = append(parts, pb.MultipartUploadPart_builder{
			PartNumber: &number,
			Size:       new(int64(len(upload.parts[number]))),
			Etag:       new(fmt.Sprintf("etag-%d", number)),
		}.Build())
	}
	return parts
}

func (f *recordsClientFake) ListRecords(
	context.Context,
	*pb.ListRecordsRequest,
	...grpc.CallOption,
) (*pb.ListRecordsResponse, error) {
	f.listCalls++
	if f.listUnauthOnce && f.listCalls == 1 {
		return nil, status.Error(codes.Unauthenticated, "access token is invalid")
	}

	ids := make([]string, 0, len(f.records))
	for id := range f.records {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	items := make([]*pb.RecordListItem, 0, len(ids))
	for _, id := range ids {
		record := f.records[id]
		items = append(items, pb.RecordListItem_builder{
			RecordId:    new(record.GetRecordId()),
			Type:        new(record.GetType()),
			Title:       new(record.GetTitle()),
			Description: new(record.GetDescription()),
			CreatedAt:   record.GetCreatedAt(),
			UpdatedAt:   record.GetUpdatedAt(),
		}.Build())
	}
	return pb.ListRecordsResponse_builder{Items: items}.Build(), nil
}

func (f *recordsClientFake) GetRecord(
	_ context.Context,
	req *pb.GetRecordRequest,
	_ ...grpc.CallOption,
) (*pb.GetRecordResponse, error) {
	record, ok := f.records[req.GetRecordId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "record not found")
	}
	return pb.GetRecordResponse_builder{Record: record}.Build(), nil
}

func (f *recordsClientFake) UpdateRecord(
	_ context.Context,
	req *pb.UpdateRecordRequest,
	_ ...grpc.CallOption,
) (*pb.UpdateRecordResponse, error) {
	record, ok := f.records[req.GetRecordId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "record not found")
	}
	if record.GetVersion() != req.GetExpectedVersion() {
		return nil, status.Error(codes.Aborted, "record version conflict")
	}

	version := record.GetVersion() + 1
	updatedAt := fixedUpdatedRecordTimestamp()
	f.records[req.GetRecordId()] = pb.Record_builder{
		RecordId:         new(record.GetRecordId()),
		Type:             new(record.GetType()),
		Title:            new(req.GetTitle()),
		Description:      new(req.GetDescription()),
		EncryptedDek:     req.GetEncryptedDek(),
		EncryptedPayload: req.GetEncryptedPayload(),
		Version:          &version,
		CreatedAt:        record.GetCreatedAt(),
		UpdatedAt:        updatedAt,
		File:             record.GetFile(),
	}.Build()
	return pb.UpdateRecordResponse_builder{
		RecordId: new(req.GetRecordId()),
		Version:  &version,
	}.Build(), nil
}

func (f *recordsClientFake) DeleteRecord(
	_ context.Context,
	req *pb.DeleteRecordRequest,
	_ ...grpc.CallOption,
) (*pb.DeleteRecordResponse, error) {
	if _, ok := f.records[req.GetRecordId()]; !ok {
		return nil, status.Error(codes.NotFound, "record not found")
	}
	delete(f.records, req.GetRecordId())
	return pb.DeleteRecordResponse_builder{RecordId: new(req.GetRecordId())}.Build(), nil
}

func (f *recordsClientFake) DownloadFile(
	ctx context.Context,
	req *pb.DownloadFileRequest,
	_ ...grpc.CallOption,
) (grpc.ServerStreamingClient[pb.DownloadFileResponse], error) {
	encryptedFile, ok := f.encryptedFiles[req.GetRecordId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "file not found")
	}
	response := pb.DownloadFileResponse_builder{
		Chunk: append([]byte(nil), encryptedFile...),
	}.Build()
	return &serverStreamFake[pb.DownloadFileResponse]{
		ctx:       ctx,
		responses: []*pb.DownloadFileResponse{response},
	}, nil
}

type clientStreamFake[Req any, Res any] struct {
	ctx          context.Context
	send         func(*Req) error
	closeAndRecv func() (*Res, error)
}

func (s *clientStreamFake[Req, Res]) Send(req *Req) error {
	return s.send(req)
}

func (s *clientStreamFake[Req, Res]) CloseAndRecv() (*Res, error) {
	return s.closeAndRecv()
}

func (s *clientStreamFake[Req, Res]) Header() (metadata.MD, error) {
	return nil, nil
}

func (s *clientStreamFake[Req, Res]) Trailer() metadata.MD {
	return nil
}

func (s *clientStreamFake[Req, Res]) CloseSend() error {
	return nil
}

func (s *clientStreamFake[Req, Res]) Context() context.Context {
	return s.ctx
}

func (s *clientStreamFake[Req, Res]) SendMsg(any) error {
	return nil
}

func (s *clientStreamFake[Req, Res]) RecvMsg(any) error {
	return io.EOF
}

type serverStreamFake[Res any] struct {
	ctx       context.Context
	responses []*Res
	next      int
}

func (s *serverStreamFake[Res]) Recv() (*Res, error) {
	if s.next >= len(s.responses) {
		return nil, io.EOF
	}
	resp := s.responses[s.next]
	s.next++
	return resp, nil
}

func (s *serverStreamFake[Res]) Header() (metadata.MD, error) {
	return nil, nil
}

func (s *serverStreamFake[Res]) Trailer() metadata.MD {
	return nil
}

func (s *serverStreamFake[Res]) CloseSend() error {
	return nil
}

func (s *serverStreamFake[Res]) Context() context.Context {
	return s.ctx
}

func (s *serverStreamFake[Res]) SendMsg(any) error {
	return nil
}

func (s *serverStreamFake[Res]) RecvMsg(any) error {
	return nil
}

func newStartedTestApp(t *testing.T) *App {
	t.Helper()

	salt, err := crypto.GenerateMasterKeySalt()
	require.NoError(t, err)
	verifier, err := crypto.EncryptMasterKeyVerifier("master-key", salt)
	require.NoError(t, err)
	recordsClient := newRecordsClientFake()
	return &App{
		auth:    &authClientFake{records: recordsClient},
		records: recordsClient,
		session: AuthSession{
			AccessToken:       "access-token",
			RefreshToken:      "refresh-token",
			MasterKeySalt:     salt,
			MasterKeyVerifier: verifier.Data,
		},
		masterKey: "master-key",
		loggedIn:  true,
	}
}
