package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadServer_HelpReturnsErrHelp проверяет, что запрос справки возвращает специальную ошибку.
func TestLoadServer_HelpReturnsErrHelp(t *testing.T) {
	// Arrange
	unsetConfigEnv(t)
	setArgs(t, "server", "-h")

	// Act
	_, err := LoadServer()

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrHelp))
}

// TestLoadServer_RequiresDatabaseURI проверяет обязательность строки подключения к БД.
func TestLoadServer_RequiresDatabaseURI(t *testing.T) {
	// Arrange
	unsetConfigEnv(t)
	t.Setenv("GOPHKEEPER_JWT_SECRET", "secret")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_ENDPOINT", "localhost:9000")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_ACCESS_KEY", "access")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_SECRET_KEY", "secret")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_BUCKET", "gophkeeper")
	setArgs(t, "server")

	// Act
	_, err := LoadServer()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database URI is required")
}

// TestLoadServer_RequiresJWTSecret проверяет обязательность JWT-секрета.
func TestLoadServer_RequiresJWTSecret(t *testing.T) {
	// Arrange
	unsetConfigEnv(t)
	t.Setenv("GOPHKEEPER_DATABASE_URI", "postgres://user:pass@localhost/db")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_ENDPOINT", "localhost:9000")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_ACCESS_KEY", "access")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_SECRET_KEY", "secret")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_BUCKET", "gophkeeper")
	setArgs(t, "server")

	// Act
	_, err := LoadServer()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT secret is required")
}

// TestLoadServer_RequiresFileStorageFields проверяет обязательность настроек файлового хранилища.
func TestLoadServer_RequiresFileStorageFields(t *testing.T) {
	tests := []struct {
		name       string
		unsetKey   string
		wantErrMsg string
	}{
		{
			name:       "endpoint",
			unsetKey:   "GOPHKEEPER_FILE_STORAGE_ENDPOINT",
			wantErrMsg: "file storage endpoint is required",
		},
		{
			name:       "access key",
			unsetKey:   "GOPHKEEPER_FILE_STORAGE_ACCESS_KEY",
			wantErrMsg: "file storage access key is required",
		},
		{
			name:       "secret key",
			unsetKey:   "GOPHKEEPER_FILE_STORAGE_SECRET_KEY",
			wantErrMsg: "file storage secret key is required",
		},
		{
			name:       "bucket",
			unsetKey:   "GOPHKEEPER_FILE_STORAGE_BUCKET",
			wantErrMsg: "file storage bucket is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			unsetConfigEnv(t)
			t.Setenv("GOPHKEEPER_DATABASE_URI", "postgres://user:pass@localhost/db")
			t.Setenv("GOPHKEEPER_JWT_SECRET", "secret")
			t.Setenv("GOPHKEEPER_FILE_STORAGE_ENDPOINT", "localhost:9000")
			t.Setenv("GOPHKEEPER_FILE_STORAGE_ACCESS_KEY", "access")
			t.Setenv("GOPHKEEPER_FILE_STORAGE_SECRET_KEY", "secret")
			t.Setenv("GOPHKEEPER_FILE_STORAGE_BUCKET", "gophkeeper")
			t.Setenv(tt.unsetKey, "")
			setArgs(t, "server")

			// Act
			_, err := LoadServer()

			// Assert
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}
}

// TestLoadServer_RejectsInvalidGRPCServerAddr проверяет ошибку при некорректном формате адреса gRPC-сервера.
func TestLoadServer_RejectsInvalidGRPCServerAddr(t *testing.T) {
	// Arrange
	unsetConfigEnv(t)
	t.Setenv("GOPHKEEPER_DATABASE_URI", "postgres://user:pass@localhost/db")
	t.Setenv("GOPHKEEPER_JWT_SECRET", "secret")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_ENDPOINT", "localhost:9000")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_ACCESS_KEY", "access")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_SECRET_KEY", "secret")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_BUCKET", "gophkeeper")
	setArgs(t, "server", "--grpc-address", "http://localhost:3201")

	// Act
	_, err := LoadServer()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server address must be in the format host:port")
}

// TestLoadServer_LoadsDefaults проверяет значения по умолчанию.
func TestLoadServer_LoadsDefaults(t *testing.T) {
	// Arrange
	unsetConfigEnv(t)
	t.Setenv("GOPHKEEPER_DATABASE_URI", "postgres://user:pass@localhost/db")
	t.Setenv("GOPHKEEPER_JWT_SECRET", "secret")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_ENDPOINT", "localhost:9000")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_ACCESS_KEY", "access")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_SECRET_KEY", "secret")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_BUCKET", "gophkeeper")
	setArgs(t, "server")

	// Act
	cfg, err := LoadServer()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:3201", cfg.GRPCServerAddr)
	assert.Equal(t, "certs/server.crt", cfg.TLSCertPath)
	assert.Equal(t, "certs/server.key", cfg.TLSKeyPath)
	assert.Equal(t, 15*time.Minute, cfg.AccessTokenTTL)
	assert.Equal(t, 30*24*time.Hour, cfg.RefreshTokenTTL)
	assert.Equal(t, "localhost:9000", cfg.FileStorage.Endpoint)
	assert.Equal(t, "access", cfg.FileStorage.AccessKey)
	assert.Equal(t, "secret", cfg.FileStorage.SecretKey)
	assert.Equal(t, "gophkeeper", cfg.FileStorage.Bucket)
	assert.Equal(t, false, cfg.FileStorage.UseSSL)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, false, cfg.Logging.AddSource)
}

// Test_validateServerAddr проверяет допустимые и недопустимые форматы сетевого адреса.
func Test_validateServerAddr(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{name: "host and port", addr: "localhost:3201"},
		{name: "ip and port", addr: "127.0.0.1:3201"},
		{name: "empty", wantErr: true},
		{name: "without port", addr: "localhost", wantErr: true},
		{name: "without host", addr: ":3201", wantErr: true},
		{name: "with scheme", addr: "http://localhost:3201", wantErr: true},
		{name: "only port separator", addr: "localhost:", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := validateServerAddr(tt.addr)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(
					t,
					"server address must be in the format host:port (without specifying a scheme)",
					err.Error(),
				)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestLoadServer_Priority проверяет приоритет "дефолтное значение < значение из конфигурационного файла < переменная
// окружения < флаг командной строки".
func TestLoadServer_Priority(t *testing.T) {
	// Arrange
	configPath := writeTempConfig(t, `
database_uri: postgres://file-db
jwt_secret: file-secret
grpc_address: localhost:3202
tls_cert: certs/file-server.crt
tls_key: certs/file-server.key
access_token_ttl: 20m
refresh_token_ttl: 720h
file_storage:
  endpoint: localhost:9000
  access_key: file-access
  secret_key: file-secret
  bucket: file-bucket
  use_ssl: true
logging:
  format: json
  level: warn
  add_source: false
`)
	unsetConfigEnv(t)
	t.Setenv("GOPHKEEPER_JWT_SECRET", "env-secret")
	t.Setenv("GOPHKEEPER_GRPC_ADDRESS", "127.0.0.1:3204")
	t.Setenv("GOPHKEEPER_TLS_CERT", "certs/env-server.crt")
	t.Setenv("GOPHKEEPER_ACCESS_TOKEN_TTL", "45m")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_ENDPOINT", "localhost:9002")
	t.Setenv("GOPHKEEPER_FILE_STORAGE_BUCKET", "env-bucket")
	t.Setenv("GOPHKEEPER_LOGGING_LEVEL", "error")
	setArgs(t,
		"server",
		"--config", configPath,
		"--database-uri", "postgres://flag-db",
		"--jwt-secret", "flag-secret",
		"--grpc-address", "127.0.0.1:3203",
		"--tls-cert", "certs/flag-server.crt",
		"--tls-key", "certs/flag-server.key",
		"--access-token-ttl", "30m",
		"--refresh-token-ttl", "840h",
		"--file-storage.endpoint", "localhost:9001",
		"--file-storage.access-key", "flag-access",
		"--file-storage.secret-key", "flag-object-secret",
		"--file-storage.bucket", "flag-bucket",
		"--file-storage.use-ssl=false",
		"--logging.level", "debug",
		"--logging.add-source",
	)

	// Act
	cfg, err := LoadServer()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "postgres://flag-db", cfg.DatabaseURI)
	assert.Equal(t, "flag-secret", cfg.JWTSecret)
	assert.Equal(t, "127.0.0.1:3203", cfg.GRPCServerAddr)
	assert.Equal(t, "certs/flag-server.crt", cfg.TLSCertPath)
	assert.Equal(t, "certs/flag-server.key", cfg.TLSKeyPath)
	assert.Equal(t, 30*time.Minute, cfg.AccessTokenTTL)
	assert.Equal(t, 840*time.Hour, cfg.RefreshTokenTTL)
	assert.Equal(t, "localhost:9001", cfg.FileStorage.Endpoint)
	assert.Equal(t, "flag-access", cfg.FileStorage.AccessKey)
	assert.Equal(t, "flag-object-secret", cfg.FileStorage.SecretKey)
	assert.Equal(t, "flag-bucket", cfg.FileStorage.Bucket)
	assert.Equal(t, false, cfg.FileStorage.UseSSL)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.True(t, cfg.Logging.AddSource)
}

// TestLoadServer_RejectsInvalidTokenTTL проверяет ошибку при неположительном времени жизни токенов.
func TestLoadServer_RejectsInvalidTokenTTL(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantErrMsg string
	}{
		{
			name:       "zero access token ttl",
			args:       []string{"--access-token-ttl", "0s"},
			wantErrMsg: "access token TTL must be positive",
		},
		{
			name:       "negative access token ttl",
			args:       []string{"--access-token-ttl", "-1s"},
			wantErrMsg: "access token TTL must be positive",
		},
		{
			name:       "zero refresh token ttl",
			args:       []string{"--refresh-token-ttl", "0s"},
			wantErrMsg: "refresh token TTL must be positive",
		},
		{
			name:       "negative refresh token ttl",
			args:       []string{"--refresh-token-ttl", "-1s"},
			wantErrMsg: "refresh token TTL must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			unsetConfigEnv(t)
			t.Setenv("GOPHKEEPER_DATABASE_URI", "postgres://user:pass@localhost/db")
			t.Setenv("GOPHKEEPER_JWT_SECRET", "secret")
			t.Setenv("GOPHKEEPER_FILE_STORAGE_ENDPOINT", "localhost:9000")
			t.Setenv("GOPHKEEPER_FILE_STORAGE_ACCESS_KEY", "access")
			t.Setenv("GOPHKEEPER_FILE_STORAGE_SECRET_KEY", "secret")
			t.Setenv("GOPHKEEPER_FILE_STORAGE_BUCKET", "gophkeeper")
			args := append([]string{"server"}, tt.args...)
			setArgs(t, args...)

			// Act
			_, err := LoadServer()

			// Assert
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}
}

// TestLoadServer_EmptyEnvironmentValueOverridesLowerPrioritySources проверяет, что пустая переменная окружения не
// откатывается к нижестоящему источнику.
func TestLoadServer_EmptyEnvironmentValueOverridesLowerPrioritySources(t *testing.T) {
	// Arrange
	configPath := writeTempConfig(t, `
database_uri: postgres://file-db
jwt_secret: file-secret
file_storage:
  endpoint: localhost:9000
  access_key: access
  secret_key: object-secret
  bucket: gophkeeper
`)
	unsetConfigEnv(t)
	t.Setenv("GOPHKEEPER_JWT_SECRET", "")
	setArgs(t,
		"server",
		"--config", configPath,
	)

	// Act
	_, err := LoadServer()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT secret is required")
}

func unsetConfigEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"GOPHKEEPER_DATABASE_URI",
		"GOPHKEEPER_JWT_SECRET",
		"GOPHKEEPER_GRPC_ADDRESS",
		"GOPHKEEPER_TLS_CERT",
		"GOPHKEEPER_TLS_KEY",
		"GOPHKEEPER_CA_CERT",
		"GOPHKEEPER_ACCESS_TOKEN_TTL",
		"GOPHKEEPER_REFRESH_TOKEN_TTL",
		"GOPHKEEPER_FILE_STORAGE_ENDPOINT",
		"GOPHKEEPER_FILE_STORAGE_ACCESS_KEY",
		"GOPHKEEPER_FILE_STORAGE_SECRET_KEY",
		"GOPHKEEPER_FILE_STORAGE_BUCKET",
		"GOPHKEEPER_FILE_STORAGE_USE_SSL",
		"GOPHKEEPER_LOGGING_FORMAT",
		"GOPHKEEPER_LOGGING_LEVEL",
		"GOPHKEEPER_LOGGING_ADD_SOURCE",
	} {
		unsetEnv(t, key)
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	oldValue, existed := os.LookupEnv(key)
	require.NoError(t, os.Unsetenv(key))
	t.Cleanup(func() {
		if existed {
			require.NoError(t, os.Setenv(key, oldValue))
			return
		}
		require.NoError(t, os.Unsetenv(key))
	})
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
	return path
}
