//go:build integration

package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"

	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/usecase"
)

type fakeFileStorage struct {
	mu               sync.Mutex
	objects          map[string][]byte
	multipartParts   map[string]map[int32][]byte
	multipartObjects map[string]string
	err              error
}

func newFakeFileStorage() *fakeFileStorage {
	return &fakeFileStorage{
		objects:          make(map[string][]byte),
		multipartParts:   make(map[string]map[int32][]byte),
		multipartObjects: make(map[string]string),
	}
}

func (s *fakeFileStorage) ObjectKey(userID, recordID, fileID uuid.UUID) string {
	return fmt.Sprintf("users/%s/records/%s/files/%s/payload", userID, recordID, fileID)
}

func (s *fakeFileStorage) Get(_ context.Context, objectKey string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, ok := s.objects[objectKey]
	if !ok {
		return nil, fmt.Errorf("object %q not found", objectKey)
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), data...))), nil
}

func (s *fakeFileStorage) CreateMultipartUpload(_ context.Context, objectKey string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	uploadID := fmt.Sprintf("fake-storage-upload-id-%d", len(s.multipartParts)+1)
	s.multipartParts[uploadID] = make(map[int32][]byte)
	s.multipartObjects[uploadID] = objectKey
	return uploadID, nil
}

func (s *fakeFileStorage) PutMultipartPart(
	_ context.Context,
	_ string,
	uploadID string,
	partNumber int32,
	data io.Reader,
	size int64,
) (usecase.MultipartUploadPart, error) {
	if s.err != nil {
		return usecase.MultipartUploadPart{}, s.err
	}
	part, err := io.ReadAll(data)
	if err != nil {
		return usecase.MultipartUploadPart{}, err
	}
	if int64(len(part)) != size {
		return usecase.MultipartUploadPart{}, usecase.ErrMultipartUploadPartInvalid
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.multipartParts[uploadID]; !ok {
		return usecase.MultipartUploadPart{}, usecase.ErrMultipartUploadNotFound
	}
	s.multipartParts[uploadID][partNumber] = part
	return usecase.MultipartUploadPart{
		PartNumber: partNumber,
		Size:       size,
		ETag:       fmt.Sprintf("etag-%d", partNumber),
	}, nil
}

func (s *fakeFileStorage) CompleteMultipartUpload(
	_ context.Context,
	objectKey string,
	uploadID string,
	parts []usecase.MultipartUploadPart,
) error {
	if s.err != nil {
		return s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	storedParts, ok := s.multipartParts[uploadID]
	if !ok {
		return usecase.ErrMultipartUploadNotFound
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})
	var object []byte
	for _, part := range parts {
		object = append(object, storedParts[part.PartNumber]...)
	}
	s.objects[objectKey] = object
	return nil
}

func (s *fakeFileStorage) AbortMultipartUpload(_ context.Context, _ string, uploadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.multipartParts, uploadID)
	delete(s.multipartObjects, uploadID)
	return nil
}

type uploadBinaryMultipartPartIntegrationStream struct {
	ctx      context.Context
	requests []*pb.UploadBinaryMultipartPartRequest
	response *pb.UploadBinaryMultipartPartResponse
}

func newUploadBinaryMultipartPartIntegrationStream(
	ctx context.Context,
	uploadID string,
	partNumber int32,
	part []byte,
) *uploadBinaryMultipartPartIntegrationStream {
	partSize := int64(len(part))
	return &uploadBinaryMultipartPartIntegrationStream{
		ctx: ctx,
		requests: []*pb.UploadBinaryMultipartPartRequest{
			pb.UploadBinaryMultipartPartRequest_builder{
				Metadata: pb.UploadBinaryMultipartPartMetadata_builder{
					UploadId:   &uploadID,
					PartNumber: &partNumber,
					PartSize:   &partSize,
				}.Build(),
			}.Build(),
			pb.UploadBinaryMultipartPartRequest_builder{Chunk: part}.Build(),
		},
	}
}

func (s *uploadBinaryMultipartPartIntegrationStream) Recv() (*pb.UploadBinaryMultipartPartRequest, error) {
	if len(s.requests) == 0 {
		return nil, io.EOF
	}
	req := s.requests[0]
	s.requests = s.requests[1:]
	return req, nil
}

func (s *uploadBinaryMultipartPartIntegrationStream) SendAndClose(resp *pb.UploadBinaryMultipartPartResponse) error {
	s.response = resp
	return nil
}

func (s *uploadBinaryMultipartPartIntegrationStream) SetHeader(metadata.MD) error {
	return nil
}

func (s *uploadBinaryMultipartPartIntegrationStream) SendHeader(metadata.MD) error {
	return nil
}

func (s *uploadBinaryMultipartPartIntegrationStream) SetTrailer(metadata.MD) {}

func (s *uploadBinaryMultipartPartIntegrationStream) Context() context.Context {
	return s.ctx
}

func (s *uploadBinaryMultipartPartIntegrationStream) SendMsg(any) error {
	return nil
}

func (s *uploadBinaryMultipartPartIntegrationStream) RecvMsg(any) error {
	return nil
}
