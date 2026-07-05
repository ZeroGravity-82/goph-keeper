package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_changeMasterKey_Cancel проверяет отмену смены мастер-ключа на любом шаге ввода.
func Test_changeMasterKey_Cancel(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "current master key", input: ":q\n"},
		{name: "new master key", input: "master-key\n:q\n"},
		{name: "confirm new master key", input: "master-key\nnew-master-key\n:q\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reader := bufio.NewReader(strings.NewReader(tt.input))
			var out bytes.Buffer

			// Act
			err := changeMasterKey(context.Background(), nil, reader, strings.NewReader(tt.input), &out)

			// Assert
			require.Error(t, err)
			assert.True(t, errors.Is(err, errActionCanceled))
			assert.Contains(t, out.String(), "Чтобы отменить действие, введите :q, cancel или отмена.")
		})
	}
}

// Test_changeMasterKey_ValidationErrors проверяет ошибки ввода до обращения к клиентскому приложению.
func Test_changeMasterKey_ValidationErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty current master key", input: "\n", want: "текущий мастер-ключ обязателен"},
		{name: "empty new master key", input: "master-key\n\n", want: "новый мастер-ключ обязателен"},
		{name: "new master key mismatch", input: "master-key\nnew-master-key\nother-master-key\n", want: "значения не совпадают"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reader := bufio.NewReader(strings.NewReader(tt.input))
			var out bytes.Buffer

			// Act
			err := changeMasterKey(context.Background(), nil, reader, strings.NewReader(tt.input), &out)

			// Assert
			require.Error(t, err)
			assert.Equal(t, tt.want, err.Error())
		})
	}
}
