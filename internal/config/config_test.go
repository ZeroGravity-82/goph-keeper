package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoad_HelpReturnsErrHelp проверяет, что запрос справки возвращает специальную ошибку.
func TestLoad_HelpReturnsErrHelp(t *testing.T) {
	// Arrange
	setArgs(t, "server", "-h")
	unsetConfigEnv(t)

	// Act
	_, err := Load()

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrHelp))
}

// TestLoad_RequiresDatabaseURI проверяет обязательность строки подключения к БД.
func TestLoad_RequiresDatabaseURI(t *testing.T) {
	// Arrange
	setArgs(t, "server")
	unsetConfigEnv(t)
	t.Setenv("GOPHKEEPER_JWT_SECRET", "secret")

	// Act
	_, err := Load()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database URI is required")
}

// TestLoad_RequiresJWTSecret проверяет обязательность JWT-секрета.
func TestLoad_RequiresJWTSecret(t *testing.T) {
	// Arrange
	setArgs(t, "server")
	unsetConfigEnv(t)
	t.Setenv("GOPHKEEPER_DATABASE_URI", "postgres://user:pass@localhost/db")

	// Act
	_, err := Load()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT secret is required")
}

// TestLoad_LoadsDefaults проверяет значения по умолчанию.
func TestLoad_LoadsDefaults(t *testing.T) {
	// Arrange
	setArgs(t, "server")
	unsetConfigEnv(t)
	t.Setenv("GOPHKEEPER_DATABASE_URI", "postgres://user:pass@localhost/db")
	t.Setenv("GOPHKEEPER_JWT_SECRET", "secret")

	// Act
	cfg, err := Load()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:3201", cfg.GRPCServerAddr)
	assert.Equal(t, "certs/server.crt", cfg.TLSCertPath)
	assert.Equal(t, "certs/server.key", cfg.TLSKeyPath)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, false, cfg.Logging.AddSource)
}

// TestLoad_Priority проверяет приоритет "дефолтное значение < значение из конфигурационного файла < флаг командной
// строки < переменная окружения".
func TestLoad_Priority(t *testing.T) {
	// Arrange
	configPath := writeTempConfig(t, `
database_uri: postgres://file-db
jwt_secret: file-secret
grpc_address: localhost:3202
tls_cert: certs/file-server.crt
tls_key: certs/file-server.key
logging:
  format: json
  level: warn
  add_source: false
`)
	setArgs(t,
		"server",
		"--config", configPath,
		"--database-uri", "postgres://flag-db",
		"--jwt-secret", "flag-secret",
		"--grpc-address", "127.0.0.1:3203",
		"--tls-cert", "certs/flag-server.crt",
		"--tls-key", "certs/flag-server.key",
		"--logging.level", "debug",
		"--logging.add-source",
	)
	unsetConfigEnv(t)
	t.Setenv("GOPHKEEPER_JWT_SECRET", "env-secret")
	t.Setenv("GOPHKEEPER_GRPC_ADDRESS", "127.0.0.1:3204")
	t.Setenv("GOPHKEEPER_TLS_CERT", "certs/env-server.crt")
	t.Setenv("GOPHKEEPER_LOGGING_LEVEL", "error")

	// Act
	cfg, err := Load()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "postgres://flag-db", cfg.DatabaseURI)
	assert.Equal(t, "env-secret", cfg.JWTSecret)
	assert.Equal(t, "127.0.0.1:3204", cfg.GRPCServerAddr)
	assert.Equal(t, "certs/env-server.crt", cfg.TLSCertPath)
	assert.Equal(t, "certs/flag-server.key", cfg.TLSKeyPath)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "error", cfg.Logging.Level)
	assert.True(t, cfg.Logging.AddSource)
}

// TestLoad_EmptyEnvironmentValueOverridesLowerPrioritySources проверяет, что пустая переменная окружения не
// откатывается к нижестоящему источнику.
func TestLoad_EmptyEnvironmentValueOverridesLowerPrioritySources(t *testing.T) {
	// Arrange
	configPath := writeTempConfig(t, `
database_uri: postgres://file-db
jwt_secret: file-secret
`)
	setArgs(t,
		"server",
		"--config", configPath,
		"--jwt-secret", "flag-secret",
	)
	unsetConfigEnv(t)
	t.Setenv("GOPHKEEPER_JWT_SECRET", "")

	// Act
	_, err := Load()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT secret is required")
}

func setArgs(t *testing.T, args ...string) {
	t.Helper()

	oldArgs := os.Args
	os.Args = args
	t.Cleanup(func() {
		os.Args = oldArgs
	})
}

func unsetConfigEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"GOPHKEEPER_DATABASE_URI",
		"GOPHKEEPER_JWT_SECRET",
		"GOPHKEEPER_GRPC_ADDRESS",
		"GOPHKEEPER_TLS_CERT",
		"GOPHKEEPER_TLS_KEY",
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
