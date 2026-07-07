//go:build integration

package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/config"
)

// TestApp_NewRunAndClose_Integration проверяет сборку серверного приложения на реальных PostgreSQL и MinIO.
func TestApp_NewRunAndClose_Integration(t *testing.T) {
	// Arrange
	cfg := integrationServerConfig(t)

	// Act
	app, err := New(cfg, nil)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, app)
	t.Cleanup(func() {
		require.NoError(t, app.Close())
	})

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)

	// Act
	err = app.Run(ctx)

	// Assert
	require.NoError(t, err)
}

// TestNew_IntegrationFailsWithInvalidDatabaseURI проверяет ошибку подключения к БД на некорректном DSN.
func TestNew_IntegrationFailsWithInvalidDatabaseURI(t *testing.T) {
	// Arrange
	cfg := integrationServerConfig(t)
	cfg.DatabaseURI = "postgres://invalid"

	// Act
	app, err := New(cfg, nil)

	// Assert
	require.Error(t, err)
	require.Nil(t, app)
}

func integrationServerConfig(t *testing.T) config.ServerConfig {
	t.Helper()

	databaseURI := os.Getenv("TEST_DATABASE_URI")
	if databaseURI == "" {
		t.Skip("TEST_DATABASE_URI is not set")
	}
	fileStorageEndpoint := os.Getenv("TEST_FILE_STORAGE_ENDPOINT")
	if fileStorageEndpoint == "" {
		t.Skip("TEST_FILE_STORAGE_ENDPOINT is not set")
	}
	fileStorageAccessKey := os.Getenv("TEST_FILE_STORAGE_ACCESS_KEY")
	fileStorageSecretKey := os.Getenv("TEST_FILE_STORAGE_SECRET_KEY")
	fileStorageBucket := os.Getenv("TEST_FILE_STORAGE_BUCKET")
	require.NotEmpty(t, fileStorageAccessKey)
	require.NotEmpty(t, fileStorageSecretKey)
	require.NotEmpty(t, fileStorageBucket)

	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	return config.ServerConfig{
		GRPCServerAddr:  "127.0.0.1:0",
		TLSCertPath:     filepath.Join(repoRoot, "certs", "server.crt"),
		TLSKeyPath:      filepath.Join(repoRoot, "certs", "server.key"),
		DatabaseURI:     databaseURI,
		JWTSecret:       "test-jwt-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		FileStorage: config.FileStorage{
			Endpoint:  fileStorageEndpoint,
			AccessKey: fileStorageAccessKey,
			SecretKey: fileStorageSecretKey,
			Bucket:    fileStorageBucket,
			UseSSL:    false,
		},
	}
}
