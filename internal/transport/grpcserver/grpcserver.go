package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"zerogravity-82/goph-keeper/internal/logging"
	"zerogravity-82/goph-keeper/internal/pb"
)

const (
	shutdownTimeout = 10 * time.Second
)

// GRPCServer описывает gRPC-сервер.
type GRPCServer struct {
	addr           string
	creds          credentials.TransportCredentials
	authService    pb.AuthServer
	recordsService pb.RecordsServer
	tokenParser    accessTokenParser
	logger         *slog.Logger
}

// NewGRPCServer создает новый GRPCServer.
func NewGRPCServer(
	addr string,
	creds credentials.TransportCredentials,
	authService pb.AuthServer,
	recordsService pb.RecordsServer,
	tokenParser accessTokenParser,
	logger *slog.Logger,
) (*GRPCServer, error) {
	if addr == "" {
		return nil, errors.New("grpc server address is not provided")
	}
	if creds == nil {
		return nil, errors.New("transport credentials are not provided")
	}
	if authService == nil {
		return nil, errors.New("auth service is not provided")
	}
	if recordsService == nil {
		return nil, errors.New("records service is not provided")
	}
	if tokenParser == nil {
		return nil, errors.New("access token parser is not provided")
	}
	if logger == nil {
		logger = logging.NopLogger()
	}

	return &GRPCServer{
		addr:           addr,
		creds:          creds,
		authService:    authService,
		recordsService: recordsService,
		tokenParser:    tokenParser,
		logger:         logger,
	}, nil
}

// Run запускает gRPC-сервер и блокируется, пока не отменен контекст или сервер не остановится с ошибкой.
func (s *GRPCServer) Run(ctx context.Context) error {
	listen, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("grpc server error: %w", err)
	}
	srv := grpc.NewServer(
		grpc.Creds(s.creds),
		grpc.UnaryInterceptor(s.authenticateInterceptor),
	)
	pb.RegisterAuthServer(srv, s.authService)
	pb.RegisterRecordsServer(srv, s.recordsService)

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("starting grpc server", slog.String("address", s.addr))
		errCh <- srv.Serve(listen)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		err := s.shutdown(shutdownCtx, srv)
		if err == nil {
			s.logger.Info("grpc server stopped with graceful shutdown")
			return nil
		}
		return fmt.Errorf("grpc server stopped with error: %w", err)
	case err := <-errCh:
		if err == nil || errors.Is(err, grpc.ErrServerStopped) {
			s.logger.Info("grpc server stopped")
			return nil
		}
		return fmt.Errorf("grpc server error: %w", err)
	}
}

// shutdown выполняет graceful shutdown gRPC-сервера и принудительно останавливает его,
// если переданный контекст завершился раньше, чем GracefulStop.
func (s *GRPCServer) shutdown(ctx context.Context, srv *grpc.Server) error {
	doneCh := make(chan struct{})

	go func() {
		srv.GracefulStop()
		close(doneCh)
	}()

	select {
	case <-ctx.Done():
		srv.Stop()
		<-doneCh
		return fmt.Errorf("graceful shutdown timeout: %w", ctx.Err())
	case <-doneCh:
		return nil
	}
}
