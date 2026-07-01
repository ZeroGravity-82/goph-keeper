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

// TestEncryptDecryptPayloadRoundTrip проверяет шифрование и расшифровку payload приватной записи.
func TestEncryptDecryptPayloadRoundTrip(t *testing.T) {
	// Arrange
	payload := model.CredentialPayload{Login: "user", Password: "secret"}
	dek := testDEK(t)

	// Act
	encrypted, err := encryptPayload(payload, dek)
	require.NoError(t, err)
	decrypted, err := decryptPayload[model.CredentialPayload](encrypted, dek)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted.Data)
	assert.Equal(t, payload, decrypted)
}

// Test_encryptPayload_FailWithUnsupportedPayload проверяет ошибку шифрования неподдерживаемого payload.
func Test_encryptPayload_FailWithUnsupportedPayload(t *testing.T) {
	// Arrange
	payload := struct {
		Value string
	}{Value: "unsupported"}
	dek := testDEK(t)

	// Act
	encrypted, err := encryptPayload(payload, dek)

	// Assert
	require.Error(t, err)
	assert.Empty(t, encrypted.Data)
}

// Test_encryptPayload_FailWithInvalidDEK проверяет ошибку шифрования при DEK некорректной длины.
func Test_encryptPayload_FailWithInvalidDEK(t *testing.T) {
	// Arrange
	payload := model.TextPayload{Text: "secret"}

	// Act
	encrypted, err := encryptPayload(payload, []byte("short-dek"))

	// Assert
	require.Error(t, err)
	assert.Empty(t, encrypted.Data)
}

// Test_decryptPayload_FailWithWrongDEK проверяет ошибку расшифровки payload неправильным DEK.
func Test_decryptPayload_FailWithWrongDEK(t *testing.T) {
	// Arrange
	payload := model.TextPayload{Text: "secret"}
	dek := testDEK(t)
	wrongDEK := testDEK(t)
	encrypted, err := encryptPayload(payload, dek)
	require.NoError(t, err)

	// Act
	decrypted, err := decryptPayload[model.TextPayload](encrypted, wrongDEK)

	// Assert
	require.Error(t, err)
	assert.Empty(t, decrypted)
}

// Test_decryptPayload_FailWithInvalidJSON проверяет ошибку, если расшифрованный payload не является корректным JSON.
func Test_decryptPayload_FailWithInvalidJSON(t *testing.T) {
	// Arrange
	dek := testDEK(t)
	encrypted, err := encrypt([]byte("{"), dek)
	require.NoError(t, err)

	// Act
	decrypted, err := decryptPayload[model.TextPayload](encrypted, dek)

	// Assert
	require.Error(t, err)
	assert.Empty(t, decrypted)
}

// Test_decryptPayload_FailWithUnsupportedPayload проверяет ошибку расшифровки в неподдерживаемый тип.
func Test_decryptPayload_FailWithUnsupportedPayload(t *testing.T) {
	// Arrange
	dek := testDEK(t)
	encrypted, err := encrypt([]byte(`{"value":"unsupported"}`), dek)
	require.NoError(t, err)

	// Act
	decrypted, err := decryptPayload[struct{ Value string }](encrypted, dek)

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

// TestEncryptDecryptRecordData проверяет полный цикл шифрования и расшифровки данных приватной записи.
func TestEncryptDecryptRecordData(t *testing.T) {
	// Arrange
	masterKey := "correct horse battery staple"
	salt := []byte("1234567890abcdef")
	payload := model.CredentialPayload{Login: "user", Password: "secret"}

	// Act
	encrypted, err := EncryptRecordData(masterKey, salt, payload)
	require.NoError(t, err)
	decrypted, err := DecryptRecordData[model.CredentialPayload](masterKey, salt, encrypted)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted.EncryptedDEK.Data)
	assert.NotEmpty(t, encrypted.EncryptedPayload.Data)
	assert.Equal(t, payload, decrypted)
}

// TestEncryptDecryptBinaryRecordData проверяет полный цикл шифрования и расшифровки бинарной приватной записи.
func TestEncryptDecryptBinaryRecordData(t *testing.T) {
	// Arrange
	masterKey := "correct horse battery staple"
	salt := []byte("1234567890abcdef")
	payload := model.BinaryPayload{Filename: "passport.pdf", ContentType: "application/pdf", Size: 11}
	file := []byte("file-secret")

	// Act
	encrypted, err := EncryptBinaryRecordData(masterKey, salt, payload, file)
	require.NoError(t, err)
	decryptedPayload, err := DecryptRecordData[model.BinaryPayload](masterKey, salt, EncryptedRecordData{
		EncryptedDEK:     encrypted.EncryptedDEK,
		EncryptedPayload: encrypted.EncryptedPayload,
	})
	require.NoError(t, err)
	decryptedFile, err := DecryptBinaryRecordFile(masterKey, salt, encrypted.EncryptedDEK, encrypted.EncryptedFile)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted.EncryptedDEK.Data)
	assert.NotEmpty(t, encrypted.EncryptedPayload.Data)
	assert.NotEmpty(t, encrypted.EncryptedFile.Data)
	assert.Equal(t, payload, decryptedPayload)
	assert.Equal(t, file, decryptedFile)
}

// TestDecryptBinaryRecordFile_FailWithWrongMasterKey проверяет ошибку расшифровки файла неправильным мастер-ключом.
func TestDecryptBinaryRecordFile_FailWithWrongMasterKey(t *testing.T) {
	// Arrange
	masterKey := "correct horse battery staple"
	salt := []byte("1234567890abcdef")
	payload := model.BinaryPayload{Filename: "passport.pdf", ContentType: "application/pdf", Size: 11}
	encrypted, err := EncryptBinaryRecordData(masterKey, salt, payload, []byte("file-secret"))
	require.NoError(t, err)

	// Act
	file, err := DecryptBinaryRecordFile("wrong master key", salt, encrypted.EncryptedDEK, encrypted.EncryptedFile)

	// Assert
	require.Error(t, err)
	assert.Nil(t, file)
}

// TestEncryptRecordData_FailWithInvalidMasterKey проверяет ошибку при некорректном мастер-ключе.
func TestEncryptRecordData_FailWithInvalidMasterKey(t *testing.T) {
	// Arrange
	salt := []byte("1234567890abcdef")
	payload := model.TextPayload{Text: "secret"}

	// Act
	encrypted, err := EncryptRecordData("", salt, payload)

	// Assert
	require.Error(t, err)
	assert.Empty(t, encrypted.EncryptedDEK.Data)
	assert.Empty(t, encrypted.EncryptedPayload.Data)
}

// TestEncryptRecordData_FailWithInvalidSalt проверяет ошибку при соли некорректной длины.
func TestEncryptRecordData_FailWithInvalidSalt(t *testing.T) {
	// Arrange
	payload := model.TextPayload{Text: "secret"}

	// Act
	encrypted, err := EncryptRecordData("master key", []byte("short"), payload)

	// Assert
	require.Error(t, err)
	assert.Empty(t, encrypted.EncryptedDEK.Data)
	assert.Empty(t, encrypted.EncryptedPayload.Data)
}

// TestEncryptRecordData_FailWithUnsupportedPayload проверяет ошибку при неподдерживаемом payload.
func TestEncryptRecordData_FailWithUnsupportedPayload(t *testing.T) {
	// Arrange
	masterKey := "correct horse battery staple"
	salt := []byte("1234567890abcdef")
	payload := struct {
		Value string
	}{Value: "unsupported"}

	// Act
	encrypted, err := EncryptRecordData(masterKey, salt, payload)

	// Assert
	require.Error(t, err)
	assert.Empty(t, encrypted.EncryptedDEK.Data)
	assert.Empty(t, encrypted.EncryptedPayload.Data)
}

// TestDecryptRecordData_FailWithWrongMasterKey проверяет ошибку расшифровки при неправильном мастер-ключе.
func TestDecryptRecordData_FailWithWrongMasterKey(t *testing.T) {
	// Arrange
	salt := []byte("1234567890abcdef")
	payload := model.TextPayload{Text: "secret"}
	encrypted, err := EncryptRecordData("master key", salt, payload)
	require.NoError(t, err)

	// Act
	decrypted, err := DecryptRecordData[model.TextPayload]("wrong master key", salt, encrypted)

	// Assert
	require.Error(t, err)
	assert.Empty(t, decrypted)
}

// TestDecryptRecordData_FailWithInvalidSalt проверяет ошибку при соли некорректной длины.
func TestDecryptRecordData_FailWithInvalidSalt(t *testing.T) {
	// Act
	decrypted, err := DecryptRecordData[model.TextPayload](
		"master key",
		[]byte("short"),
		EncryptedRecordData{
			EncryptedDEK:     model.EncryptedBlob{Data: []byte("encrypted-dek")},
			EncryptedPayload: model.EncryptedBlob{Data: []byte("encrypted-payload")},
		},
	)

	// Assert
	require.Error(t, err)
	assert.Empty(t, decrypted)
}

// TestDecryptRecordData_FailWithDamagedEncryptedDEK проверяет ошибку при поврежденном encrypted DEK.
func TestDecryptRecordData_FailWithDamagedEncryptedDEK(t *testing.T) {
	// Arrange
	masterKey := "correct horse battery staple"
	salt := []byte("1234567890abcdef")
	payload := model.TextPayload{Text: "secret"}
	encrypted, err := EncryptRecordData(masterKey, salt, payload)
	require.NoError(t, err)
	encrypted.EncryptedDEK.Data[len(encrypted.EncryptedDEK.Data)-1] ^= 1

	// Act
	decrypted, err := DecryptRecordData[model.TextPayload](masterKey, salt, encrypted)

	// Assert
	require.Error(t, err)
	assert.Empty(t, decrypted)
}

// TestDecryptRecordData_FailWithDamagedEncryptedPayload проверяет ошибку при поврежденном encrypted payload.
func TestDecryptRecordData_FailWithDamagedEncryptedPayload(t *testing.T) {
	// Arrange
	masterKey := "correct horse battery staple"
	salt := []byte("1234567890abcdef")
	payload := model.TextPayload{Text: "secret"}
	encrypted, err := EncryptRecordData(masterKey, salt, payload)
	require.NoError(t, err)
	encrypted.EncryptedPayload.Data[len(encrypted.EncryptedPayload.Data)-1] ^= 1

	// Act
	decrypted, err := DecryptRecordData[model.TextPayload](masterKey, salt, encrypted)

	// Assert
	require.Error(t, err)
	assert.Empty(t, decrypted)
}

// TestDecryptRecordData_FailWithUnsupportedPayload проверяет ошибку расшифровки в неподдерживаемый тип.
func TestDecryptRecordData_FailWithUnsupportedPayload(t *testing.T) {
	// Arrange
	masterKey := "correct horse battery staple"
	salt := []byte("1234567890abcdef")
	payload := model.TextPayload{Text: "secret"}
	encrypted, err := EncryptRecordData(masterKey, salt, payload)
	require.NoError(t, err)

	// Act
	decrypted, err := DecryptRecordData[struct{ Value string }](masterKey, salt, encrypted)

	// Assert
	require.Error(t, err)
	assert.Empty(t, decrypted)
}
