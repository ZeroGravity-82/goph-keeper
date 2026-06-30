package client

import (
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"zerogravity-82/goph-keeper/internal/pb"
)

// App объединяет gRPC-клиенты, пользовательскую сессию и клиентские сценарии приложения.
type App struct {
	conn      *grpc.ClientConn
	auth      pb.AuthClient
	records   pb.RecordsClient
	session   AuthSession
	masterKey string
	loggedIn  bool
}

// New создает CLI-приложение и подключается к gRPC-серверу по TLS.
func New(address, caCertPath string) (*App, error) {
	if address == "" {
		return nil, errors.New("server address is required")
	}
	if caCertPath == "" {
		return nil, errors.New("CA certificate path is required")
	}

	creds, err := credentials.NewClientTLSFromFile(caCertPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	return &App{
		conn:    conn,
		auth:    pb.NewAuthClient(conn),
		records: pb.NewRecordsClient(conn),
	}, nil
}

// Close закрывает соединение с сервером.
func (a *App) Close() error {
	if a == nil || a.conn == nil {
		return nil
	}
	return a.conn.Close()
}

func (a *App) requireSession() error {
	if !a.loggedIn || a.session.AccessToken == "" || a.masterKey == "" {
		return errors.New("пользователь не вошел в аккаунт")
	}
	return nil
}
