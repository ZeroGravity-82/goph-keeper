package service

import (
	"context"
	"io"

	"google.golang.org/grpc/metadata"

	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/usecase"
)

type recordsUseCaseStub struct {
	createRecordInput        usecase.CreateRecordInput
	createRecordOutput       usecase.CreateRecordOutput
	createRecordErr          error
	startMultipartInput      usecase.StartBinaryMultipartUploadInput
	startMultipartOutput     usecase.StartBinaryMultipartUploadOutput
	startMultipartErr        error
	getMultipartStatusInput  usecase.GetBinaryMultipartUploadStatusInput
	getMultipartStatusOutput usecase.GetBinaryMultipartUploadStatusOutput
	getMultipartStatusErr    error
	uploadMultipartInput     usecase.UploadBinaryMultipartPartInput
	uploadMultipartFile      []byte
	uploadMultipartOutput    usecase.UploadBinaryMultipartPartOutput
	uploadMultipartErr       error
	completeMultipartInput   usecase.CompleteBinaryMultipartUploadInput
	completeMultipartOutput  usecase.CompleteBinaryMultipartUploadOutput
	completeMultipartErr     error
	abortMultipartInput      usecase.AbortBinaryMultipartUploadInput
	abortMultipartOutput     usecase.AbortBinaryMultipartUploadOutput
	abortMultipartErr        error
	listRecordsInput         usecase.ListRecordsInput
	listRecordsOutput        usecase.ListRecordsOutput
	listRecordsErr           error
	getRecordInput           usecase.GetRecordInput
	getRecordOutput          usecase.GetRecordOutput
	getRecordErr             error
	updateRecordInput        usecase.UpdateRecordInput
	updateRecordOutput       usecase.UpdateRecordOutput
	updateRecordErr          error
	deleteRecordInput        usecase.DeleteRecordInput
	deleteRecordOutput       usecase.DeleteRecordOutput
	deleteRecordErr          error
	downloadFileInput        usecase.DownloadFileInput
	downloadFileOutput       usecase.DownloadFileOutput
	downloadFileErr          error
}

func (s *recordsUseCaseStub) CreateRecord(
	_ context.Context,
	in usecase.CreateRecordInput,
) (usecase.CreateRecordOutput, error) {
	s.createRecordInput = in
	return s.createRecordOutput, s.createRecordErr
}

func (s *recordsUseCaseStub) StartBinaryMultipartUpload(
	_ context.Context,
	in usecase.StartBinaryMultipartUploadInput,
) (usecase.StartBinaryMultipartUploadOutput, error) {
	s.startMultipartInput = in
	return s.startMultipartOutput, s.startMultipartErr
}

func (s *recordsUseCaseStub) GetBinaryMultipartUploadStatus(
	_ context.Context,
	in usecase.GetBinaryMultipartUploadStatusInput,
) (usecase.GetBinaryMultipartUploadStatusOutput, error) {
	s.getMultipartStatusInput = in
	return s.getMultipartStatusOutput, s.getMultipartStatusErr
}

func (s *recordsUseCaseStub) UploadBinaryMultipartPart(
	_ context.Context,
	in usecase.UploadBinaryMultipartPartInput,
) (usecase.UploadBinaryMultipartPartOutput, error) {
	s.uploadMultipartInput = in
	if in.Data != nil {
		data, err := io.ReadAll(in.Data)
		if err != nil {
			return usecase.UploadBinaryMultipartPartOutput{}, err
		}
		s.uploadMultipartFile = data
	}
	return s.uploadMultipartOutput, s.uploadMultipartErr
}

func (s *recordsUseCaseStub) CompleteBinaryMultipartUpload(
	_ context.Context,
	in usecase.CompleteBinaryMultipartUploadInput,
) (usecase.CompleteBinaryMultipartUploadOutput, error) {
	s.completeMultipartInput = in
	return s.completeMultipartOutput, s.completeMultipartErr
}

func (s *recordsUseCaseStub) AbortBinaryMultipartUpload(
	_ context.Context,
	in usecase.AbortBinaryMultipartUploadInput,
) (usecase.AbortBinaryMultipartUploadOutput, error) {
	s.abortMultipartInput = in
	return s.abortMultipartOutput, s.abortMultipartErr
}

func (s *recordsUseCaseStub) ListRecords(
	_ context.Context,
	in usecase.ListRecordsInput,
) (usecase.ListRecordsOutput, error) {
	s.listRecordsInput = in
	return s.listRecordsOutput, s.listRecordsErr
}

func (s *recordsUseCaseStub) GetRecord(
	_ context.Context,
	in usecase.GetRecordInput,
) (usecase.GetRecordOutput, error) {
	s.getRecordInput = in
	return s.getRecordOutput, s.getRecordErr
}

func (s *recordsUseCaseStub) UpdateRecord(
	_ context.Context,
	in usecase.UpdateRecordInput,
) (usecase.UpdateRecordOutput, error) {
	s.updateRecordInput = in
	return s.updateRecordOutput, s.updateRecordErr
}

func (s *recordsUseCaseStub) DeleteRecord(
	_ context.Context,
	in usecase.DeleteRecordInput,
) (usecase.DeleteRecordOutput, error) {
	s.deleteRecordInput = in
	return s.deleteRecordOutput, s.deleteRecordErr
}

func (s *recordsUseCaseStub) DownloadFile(
	_ context.Context,
	in usecase.DownloadFileInput,
) (usecase.DownloadFileOutput, error) {
	s.downloadFileInput = in
	return s.downloadFileOutput, s.downloadFileErr
}

type downloadFileTestStream struct {
	ctx    context.Context
	chunks [][]byte
}

func newDownloadFileTestStream(ctx context.Context) *downloadFileTestStream {
	return &downloadFileTestStream{ctx: ctx}
}

func (s *downloadFileTestStream) Send(resp *pb.DownloadFileResponse) error {
	s.chunks = append(s.chunks, append([]byte(nil), resp.GetChunk()...))
	return nil
}

func (s *downloadFileTestStream) SetHeader(metadata.MD) error {
	return nil
}

func (s *downloadFileTestStream) SendHeader(metadata.MD) error {
	return nil
}

func (s *downloadFileTestStream) SetTrailer(metadata.MD) {}

func (s *downloadFileTestStream) Context() context.Context {
	return s.ctx
}

func (s *downloadFileTestStream) SendMsg(any) error {
	return nil
}

func (s *downloadFileTestStream) RecvMsg(any) error {
	return nil
}
