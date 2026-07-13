package client

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew_ValidatesRequiredArguments проверяет ошибки при отсутствии адреса сервера или CA-сертификата.
func TestNew_ValidatesRequiredArguments(t *testing.T) {
	tests := []struct {
		name       string
		address    string
		caCertPath string
		wantErr    string
	}{
		{name: "empty address", caCertPath: "ca.crt", wantErr: "server address is required"},
		{name: "empty ca", address: "127.0.0.1:32000", wantErr: "CA certificate path is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			app, err := New(tt.address, tt.caCertPath)

			// Assert
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
			assert.Nil(t, app)
		})
	}
}

// TestNew_ReturnsCertificateError проверяет ошибку загрузки CA-сертификата.
func TestNew_ReturnsCertificateError(t *testing.T) {
	// Act
	app, err := New("127.0.0.1:32000", "missing-ca.crt")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load CA certificate")
	assert.Nil(t, app)
}

// TestNew_CreatesApp проверяет создание клиента без фактического подключения к серверу.
func TestNew_CreatesApp(t *testing.T) {
	// Arrange
	caCertPath := filepath.Join("..", "..", "..", "certs", "ca.crt")

	// Act
	app, err := New("127.0.0.1:32000", caCertPath)
	t.Cleanup(func() {
		require.NoError(t, app.Close())
	})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, app)
	assert.NotNil(t, app.auth)
	assert.NotNil(t, app.records)
}

// TestClose_AllowsNilApp проверяет, что закрытие nil-приложения безопасно.
func TestClose_AllowsNilApp(t *testing.T) {
	// Arrange
	var app *App

	// Act
	err := app.Close()

	// Assert
	require.NoError(t, err)
}

// TestApp_requireSession проверяет валидацию активной пользовательской сессии.
func TestApp_requireSession(t *testing.T) {
	tests := []struct {
		name    string
		app     *App
		wantErr bool
	}{
		{name: "empty", app: &App{}, wantErr: true},
		{name: "without access token", app: &App{loggedIn: true, masterKey: "key"}, wantErr: true},
		{
			name: "ok",
			app: &App{
				loggedIn:  true,
				session:   AuthSession{AccessToken: "access-token"},
				masterKey: "key",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := tt.app.requireSession()

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, "пользователь не вошел в аккаунт", err.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}
