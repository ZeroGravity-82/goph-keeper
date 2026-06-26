package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

// TestMarshalUnmarshalCredentialPayload проверяет преобразование в JSON и обратно для payload с учетными данными.
func TestMarshalUnmarshalCredentialPayload(t *testing.T) {
	// Arrange
	payload := model.CredentialPayload{Login: "user", Password: "secret"}

	// Act
	data, err := MarshalPayload(payload)
	require.NoError(t, err)
	decoded, err := UnmarshalPayload[model.CredentialPayload](data)

	// Assert
	require.NoError(t, err)
	assert.JSONEq(t, `{"login":"user","password":"secret"}`, string(data))
	assert.Equal(t, payload, decoded)
}

// TestMarshalUnmarshalTextPayload проверяет преобразование в JSON и обратно для текстового payload.
func TestMarshalUnmarshalTextPayload(t *testing.T) {
	// Arrange
	payload := model.TextPayload{Text: "secret text"}

	// Act
	data, err := MarshalPayload(payload)
	require.NoError(t, err)
	decoded, err := UnmarshalPayload[model.TextPayload](data)

	// Assert
	require.NoError(t, err)
	assert.JSONEq(t, `{"text":"secret text"}`, string(data))
	assert.Equal(t, payload, decoded)
}

// TestMarshalUnmarshalCardPayload проверяет преобразование в JSON и обратно для payload банковской карты.
func TestMarshalUnmarshalCardPayload(t *testing.T) {
	// Arrange
	payload := model.CardPayload{
		Number:     "4111111111111111",
		HolderName: "IVAN IVANOV",
		ExpiresAt:  "12/30",
		CVC:        "123",
	}

	// Act
	data, err := MarshalPayload(payload)
	require.NoError(t, err)
	decoded, err := UnmarshalPayload[model.CardPayload](data)

	// Assert
	require.NoError(t, err)
	assert.JSONEq(
		t,
		`{"number":"4111111111111111","holder_name":"IVAN IVANOV","expires_at":"12/30","cvc":"123"}`,
		string(data),
	)
	assert.Equal(t, payload, decoded)
}

// TestMarshalUnmarshalBinaryPayload проверяет преобразование в JSON и обратно для payload бинарной приватной записи.
func TestMarshalUnmarshalBinaryPayload(t *testing.T) {
	// Arrange
	payload := model.BinaryPayload{Filename: "passport.pdf", ContentType: "application/pdf", Size: 1024}

	// Act
	data, err := MarshalPayload(payload)
	require.NoError(t, err)
	decoded, err := UnmarshalPayload[model.BinaryPayload](data)

	// Assert
	require.NoError(t, err)
	assert.JSONEq(t, `{"filename":"passport.pdf","content_type":"application/pdf","size":1024}`, string(data))
	assert.Equal(t, payload, decoded)
}

// TestMarshalPayload_FailWithUnsupportedPayload проверяет ошибку при неподдерживаемом типе payload.
func TestMarshalPayload_FailWithUnsupportedPayload(t *testing.T) {
	// Arrange
	payload := struct {
		Value string
	}{Value: "unsupported"}

	// Act
	data, err := MarshalPayload(payload)

	// Assert
	require.Error(t, err)
	assert.Nil(t, data)
}

// TestUnmarshalPayload_FailWithUnsupportedPayload проверяет ошибку при неподдерживаемом типе результата.
func TestUnmarshalPayload_FailWithUnsupportedPayload(t *testing.T) {
	// Act
	payload, err := UnmarshalPayload[struct{ Value string }]([]byte(`{"value":"unsupported"}`))

	// Assert
	require.Error(t, err)
	assert.Empty(t, payload)
}

// TestUnmarshalPayload_FailWithInvalidJSON проверяет ошибки при некорректном JSON.
func TestUnmarshalPayload_FailWithInvalidJSON(t *testing.T) {
	// Arrange
	invalidJSON := []byte(`{"login":`)
	tests := []struct {
		name string
		fn   func([]byte) error
	}{
		{
			name: "credential",
			fn: func(data []byte) error {
				_, err := UnmarshalPayload[model.CredentialPayload](data)
				return err
			},
		},
		{
			name: "text",
			fn: func(data []byte) error {
				_, err := UnmarshalPayload[model.TextPayload](data)
				return err
			},
		},
		{
			name: "card",
			fn: func(data []byte) error {
				_, err := UnmarshalPayload[model.CardPayload](data)
				return err
			},
		},
		{
			name: "binary",
			fn: func(data []byte) error {
				_, err := UnmarshalPayload[model.BinaryPayload](data)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := tt.fn(invalidJSON)

			// Assert
			require.Error(t, err)
		})
	}
}
