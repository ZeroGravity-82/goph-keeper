package grpcserver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"zerogravity-82/goph-keeper/internal/pb"
)

// TestGRPCServer_RunStopsOnContextCancel проверяет graceful shutdown запущенного сервера при отмене контекста.
func TestGRPCServer_RunStopsOnContextCancel(t *testing.T) {
	// Arrange
	srv, err := NewGRPCServer(
		"127.0.0.1:0",
		insecure.NewCredentials(),
		&pb.UnimplementedAuthServer{},
		&pb.UnimplementedRecordsServer{},
		&accessTokenParserStub{},
		&userSessionCheckerStub{},
		nil,
	)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	time.AfterFunc(10*time.Millisecond, cancel)

	// Act
	err = srv.Run(ctx)

	// Assert
	require.NoError(t, err)
}

// TestGRPCServer_shutdownStopsIdleServer проверяет graceful shutdown gRPC-сервера без активных соединений.
func TestGRPCServer_shutdownStopsIdleServer(t *testing.T) {
	// Arrange
	srv := &GRPCServer{}
	grpcSrv := grpc.NewServer()

	// Act
	err := srv.shutdown(context.Background(), grpcSrv)

	// Assert
	require.NoError(t, err)
}
