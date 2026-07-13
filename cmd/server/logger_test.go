package main

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/config"
)

// Test_parseLogLevel проверяет поддерживаемые уровни логирования и ошибку для неизвестного значения.
func Test_parseLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    slog.Level
		wantErr string
	}{
		{name: "debug", value: "debug", want: slog.LevelDebug},
		{name: "info upper case", value: "INFO", want: slog.LevelInfo},
		{name: "warn", value: "warn", want: slog.LevelWarn},
		{name: "error", value: "error", want: slog.LevelError},
		{name: "unknown", value: "trace", wantErr: `unsupported log level: "trace"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := parseLogLevel(tt.value)

			// Assert
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test_newLogger проверяет создание логгера для поддерживаемых форматов и ошибку для неизвестного формата.
func Test_newLogger(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Logging
		wantErr string
	}{
		{name: "json", cfg: config.Logging{Level: "info", Format: "json"}},
		{name: "text", cfg: config.Logging{Level: "debug", Format: "TEXT", AddSource: true}},
		{
			name:    "bad format",
			cfg:     config.Logging{Level: "info", Format: "xml"},
			wantErr: `unsupported log format: "xml"`,
		},
		{
			name:    "bad level",
			cfg:     config.Logging{Level: "trace", Format: "json"},
			wantErr: `unsupported log level: "trace"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			logger, err := newLogger(tt.cfg)

			// Assert
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
				assert.Nil(t, logger)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, logger)
		})
	}
}
