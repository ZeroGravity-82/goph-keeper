package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

// TestPayloadJSONRoundTrip_CredentialPayload проверяет преобразование в JSON и обратно для payload с учетными данными.
func TestPayloadJSONRoundTrip_CredentialPayload(t *testing.T) {
	// Arrange
	payload := model.CredentialPayload{Login: "user", Password: "secret"}

	// Act
	data, err := marshalPayload(payload)
	require.NoError(t, err)
	decoded, err := unmarshalPayload[model.CredentialPayload](data)

	// Assert
	require.NoError(t, err)
	assert.JSONEq(t, `{"login":"user","password":"secret"}`, string(data))
	assert.Equal(t, payload, decoded)
}

// TestPayloadJSONRoundTrip_TextPayload проверяет преобразование в JSON и обратно для текстового payload.
func TestPayloadJSONRoundTrip_TextPayload(t *testing.T) {
	// Arrange
	payload := model.TextPayload{Text: "secret text"}

	// Act
	data, err := marshalPayload(payload)
	require.NoError(t, err)
	decoded, err := unmarshalPayload[model.TextPayload](data)

	// Assert
	require.NoError(t, err)
	assert.JSONEq(t, `{"text":"secret text"}`, string(data))
	assert.Equal(t, payload, decoded)
}

// TestPayloadJSONRoundTrip_CardPayload проверяет преобразование в JSON и обратно для payload банковской карты.
func TestPayloadJSONRoundTrip_CardPayload(t *testing.T) {
	// Arrange
	payload := model.CardPayload{
		Number:     "4111111111111111",
		HolderName: "IVAN IVANOV",
		ExpiresAt:  "12/30",
		CVC:        "123",
	}

	// Act
	data, err := marshalPayload(payload)
	require.NoError(t, err)
	decoded, err := unmarshalPayload[model.CardPayload](data)

	// Assert
	require.NoError(t, err)
	assert.JSONEq(
		t,
		`{"number":"4111111111111111","holder_name":"IVAN IVANOV","expires_at":"12/30","cvc":"123"}`,
		string(data),
	)
	assert.Equal(t, payload, decoded)
}

// TestPayloadJSONRoundTrip_BinaryPayload проверяет преобразование в JSON и обратно для payload бинарной приватной
// записи.
func TestPayloadJSONRoundTrip_BinaryPayload(t *testing.T) {
	// Arrange
	payload := model.BinaryPayload{Filename: "passport.pdf", ContentType: "application/pdf", Size: 1024}

	// Act
	data, err := marshalPayload(payload)
	require.NoError(t, err)
	decoded, err := unmarshalPayload[model.BinaryPayload](data)

	// Assert
	require.NoError(t, err)
	assert.JSONEq(t, `{"filename":"passport.pdf","content_type":"application/pdf","size":1024}`, string(data))
	assert.Equal(t, payload, decoded)
}

// TestEncryptDecryptPayload проверяет шифрование и расшифровку payload приватной записи.
func TestEncryptDecryptPayload(t *testing.T) {
	// Arrange
	payload := model.CredentialPayload{Login: "user", Password: "secret"}
	dek := testDEK(t)

	// Act
	encrypted, err := EncryptPayload(payload, dek)
	require.NoError(t, err)
	decrypted, err := DecryptPayload[model.CredentialPayload](encrypted, dek)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted.Data)
	assert.Equal(t, payload, decrypted)
}

// TestEncryptPayload_FailWithUnsupportedPayload проверяет ошибку шифрования неподдерживаемого payload.
func TestEncryptPayload_FailWithUnsupportedPayload(t *testing.T) {
	// Arrange
	payload := struct {
		Value string
	}{Value: "unsupported"}
	dek := testDEK(t)

	// Act
	encrypted, err := EncryptPayload(payload, dek)

	// Assert
	require.Error(t, err)
	assert.Empty(t, encrypted.Data)
}

// TestEncryptPayload_FailWithInvalidDEK проверяет ошибку шифрования при DEK некорректной длины.
func TestEncryptPayload_FailWithInvalidDEK(t *testing.T) {
	// Arrange
	payload := model.TextPayload{Text: "secret"}

	// Act
	encrypted, err := EncryptPayload(payload, []byte("short-dek"))

	// Assert
	require.Error(t, err)
	assert.Empty(t, encrypted.Data)
}

// TestDecryptPayload_FailWithWrongDEK проверяет ошибку расшифровки payload неправильным DEK.
func TestDecryptPayload_FailWithWrongDEK(t *testing.T) {
	// Arrange
	payload := model.TextPayload{Text: "secret"}
	dek := testDEK(t)
	wrongDEK := testDEK(t)
	encrypted, err := EncryptPayload(payload, dek)
	require.NoError(t, err)

	// Act
	decrypted, err := DecryptPayload[model.TextPayload](encrypted, wrongDEK)

	// Assert
	require.Error(t, err)
	assert.Empty(t, decrypted)
}

// TestDecryptPayload_FailWithInvalidJSON проверяет ошибку, если расшифрованный payload не является корректным JSON.
func TestDecryptPayload_FailWithInvalidJSON(t *testing.T) {
	// Arrange
	dek := testDEK(t)
	encrypted, err := Encrypt([]byte("{"), dek)
	require.NoError(t, err)

	// Act
	decrypted, err := DecryptPayload[model.TextPayload](encrypted, dek)

	// Assert
	require.Error(t, err)
	assert.Empty(t, decrypted)
}

// TestDecryptPayload_FailWithUnsupportedPayload проверяет ошибку расшифровки в неподдерживаемый тип.
func TestDecryptPayload_FailWithUnsupportedPayload(t *testing.T) {
	// Arrange
	dek := testDEK(t)
	encrypted, err := Encrypt([]byte(`{"value":"unsupported"}`), dek)
	require.NoError(t, err)

	// Act
	decrypted, err := DecryptPayload[struct{ Value string }](encrypted, dek)

	// Assert
	require.Error(t, err)
	assert.Empty(t, decrypted)
}

// Test_marshalPayload_FailWithUnsupportedPayload проверяет ошибку при неподдерживаемом типе payload.
func Test_marshalPayload_FailWithUnsupportedPayload(t *testing.T) {
	// Arrange
	payload := struct {
		Value string
	}{Value: "unsupported"}

	// Act
	data, err := marshalPayload(payload)

	// Assert
	require.Error(t, err)
	assert.Nil(t, data)
}

// Test_unmarshalPayload_FailWithUnsupportedPayload проверяет ошибку при неподдерживаемом типе результата.
func Test_unmarshalPayload_FailWithUnsupportedPayload(t *testing.T) {
	// Act
	payload, err := unmarshalPayload[struct{ Value string }]([]byte(`{"value":"unsupported"}`))

	// Assert
	require.Error(t, err)
	assert.Empty(t, payload)
}

// Test_unmarshalPayload_FailWithInvalidJSON проверяет ошибки при некорректном JSON.
func Test_unmarshalPayload_FailWithInvalidJSON(t *testing.T) {
	// Arrange
	invalidJSON := []byte(`{"login":`)
	tests := []struct {
		name string
		fn   func([]byte) error
	}{
		{
			name: "credential",
			fn: func(data []byte) error {
				_, err := unmarshalPayload[model.CredentialPayload](data)
				return err
			},
		},
		{
			name: "text",
			fn: func(data []byte) error {
				_, err := unmarshalPayload[model.TextPayload](data)
				return err
			},
		},
		{
			name: "card",
			fn: func(data []byte) error {
				_, err := unmarshalPayload[model.CardPayload](data)
				return err
			},
		},
		{
			name: "binary",
			fn: func(data []byte) error {
				_, err := unmarshalPayload[model.BinaryPayload](data)
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
