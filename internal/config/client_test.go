package config

import (
	"errors"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadClient_RequiresGRPCServerAddr проверяет обязательность адреса gRPC-сервера.
func TestLoadClient_RequiresGRPCServerAddr(t *testing.T) {
	// Arrange
	setArgs(t, "client")
	unsetConfigEnv(t)

	// Act
	_, err := LoadClient()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gRPC server address is required")
}

// TestLoadClient_ReturnsErrHelp проверяет штатную обработку запроса справки.
func TestLoadClient_ReturnsErrHelp(t *testing.T) {
	// Arrange
	setArgs(t, "client", "--help")
	unsetConfigEnv(t)

	// Act
	_, err := LoadClient()

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrHelp))
}

// Test_loadClient_RejectsPositionalArgument проверяет запрет позиционных аргументов CLI-клиента.
func Test_loadClient_RejectsPositionalArgument(t *testing.T) {
	// Arrange
	unsetConfigEnv(t)

	// Act
	_, err := loadClient([]string{"register"})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unexpected positional argument "register"`)
}

// Test_loadClient_LoadsConfigFlags проверяет загрузку конфигурации клиента из аргументов командной строки.
func Test_loadClient_LoadsConfigFlags(t *testing.T) {
	// Arrange
	unsetConfigEnv(t)

	// Act
	cfg, err := loadClient([]string{
		"--grpc-address",
		"127.0.0.1:3203",
		"--ca-cert",
		"certs/flag-ca.crt",
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:3203", cfg.GRPCServerAddr)
	assert.Equal(t, "certs/flag-ca.crt", cfg.CACertPath)
}

// Test_loadClientFromFlags_RequiresGRPCServerAddr проверяет обязательность адреса gRPC-сервера.
func Test_loadClientFromFlags_RequiresGRPCServerAddr(t *testing.T) {
	// Arrange
	flags := newTestClientFlagSet(t)
	parseTestFlags(t, flags)
	unsetConfigEnv(t)

	// Act
	_, err := loadClientFromFlags(flags)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gRPC server address is required")
}

func parseTestFlags(t *testing.T, flags *pflag.FlagSet, args ...string) {
	t.Helper()

	require.NoError(t, flags.Parse(args))
}

func newTestClientFlagSet(t *testing.T) *pflag.FlagSet {
	t.Helper()

	return NewClientFlagSet("client")
}

// Test_loadClientFromFlags_RequiresCACertPath проверяет обязательность CA-сертификата клиента.
func Test_loadClientFromFlags_RequiresCACertPath(t *testing.T) {
	// Arrange
	flags := newTestClientFlagSet(t)
	parseTestFlags(t, flags, "--grpc-address", "127.0.0.1:3203")
	unsetConfigEnv(t)

	// Act
	_, err := loadClientFromFlags(flags)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CA certificate path is required")
}

// Test_loadClientFromFlags_Priority проверяет приоритет источников конфигурации CLI-клиента.
func Test_loadClientFromFlags_Priority(t *testing.T) {
	// Arrange
	configPath := writeTempConfig(t, `
grpc_address: localhost:3202
ca_cert: certs/file-ca.crt
`)
	flags := newTestClientFlagSet(t)
	parseTestFlags(t, flags,
		"--config", configPath,
		"--grpc-address", "127.0.0.1:3203",
		"--ca-cert", "certs/flag-ca.crt",
	)
	unsetConfigEnv(t)
	t.Setenv("GOPHKEEPER_GRPC_ADDRESS", "127.0.0.1:3204")

	// Act
	cfg, err := loadClientFromFlags(flags)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:3204", cfg.GRPCServerAddr)
	assert.Equal(t, "certs/flag-ca.crt", cfg.CACertPath)
}

// Test_loadClientFromFlags_EmptyEnvironmentValueOverridesLowerPrioritySources проверяет, что пустая переменная
// окружения не откатывается к нижестоящему источнику.
func Test_loadClientFromFlags_EmptyEnvironmentValueOverridesLowerPrioritySources(t *testing.T) {
	// Arrange
	flags := newTestClientFlagSet(t)
	parseTestFlags(t, flags, "--grpc-address", "127.0.0.1:3203", "--ca-cert", "certs/flag-ca.crt")
	unsetConfigEnv(t)
	t.Setenv("GOPHKEEPER_CA_CERT", "")

	// Act
	_, err := loadClientFromFlags(flags)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CA certificate path is required")
}
