package client

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_validateRecordLimits_OKWithMaxSizeData проверяет, что граничные значения клиентских лимитов допустимы.
func Test_validateRecordLimits_OKWithMaxSizeData(t *testing.T) {
	// Act
	metadataErr := validateRecordMetadataSize(
		strings.Repeat("a", recordTitleMaxSizeChars),
		strings.Repeat("a", recordDescriptionMaxSizeChars),
	)
	textErr := validateTextRecordPayloadSize(strings.Repeat("a", textRecordTextMaxSizeBytes))
	credentialErr := validateCredentialPayloadSize(
		strings.Repeat("a", credentialLoginMaxSizeChars),
		strings.Repeat("a", credentialPasswordMaxSizeChars),
	)
	cardHolderErr := validateCardHolderNameSize(strings.Repeat("A", cardHolderNameMaxSizeChars))
	binaryErr := validateBinaryPayloadSize(
		strings.Repeat("a", binaryFilenameMaxSizeChars),
		strings.Repeat("a", binaryContentTypeMaxSizeChars),
	)
	binaryFileErr := validateBinaryFileSize(plainBinaryRecordFileMaxSizeBytes)

	// Assert
	require.NoError(t, metadataErr)
	require.NoError(t, textErr)
	require.NoError(t, credentialErr)
	require.NoError(t, cardHolderErr)
	require.NoError(t, binaryErr)
	require.NoError(t, binaryFileErr)
}

// Test_validateRecordMetadataSize_FailsWithLongTitle проверяет клиентский лимит длины названия записи.
func Test_validateRecordMetadataSize_FailsWithLongTitle(t *testing.T) {
	// Act
	err := validateRecordMetadataSize(strings.Repeat("a", recordTitleMaxSizeChars+1), "")

	// Assert
	assert.EqualError(t, err, "название записи не должно превышать 128 символов")
}

// Test_validateTextRecordPayloadSize_FailsWithLargeText проверяет клиентский лимит текста приватной записи.
func Test_validateTextRecordPayloadSize_FailsWithLargeText(t *testing.T) {
	// Act
	err := validateTextRecordPayloadSize(strings.Repeat("a", textRecordTextMaxSizeBytes+1))

	// Assert
	assert.EqualError(t, err, "текст записи не должен превышать 256 КиБ")
}

// Test_validateCredentialPayloadSize_FailsWithLongPassword проверяет клиентский лимит пароля в payload.
func Test_validateCredentialPayloadSize_FailsWithLongPassword(t *testing.T) {
	// Act
	err := validateCredentialPayloadSize("login", strings.Repeat("a", credentialPasswordMaxSizeChars+1))

	// Assert
	assert.EqualError(t, err, "пароль записи с учетными данными не должен превышать 256 символов")
}

// Test_validateBinaryPayloadSize_FailsWithLongFilename проверяет клиентский лимит имени файла в payload.
func Test_validateBinaryPayloadSize_FailsWithLongFilename(t *testing.T) {
	// Act
	err := validateBinaryPayloadSize(strings.Repeat("a", binaryFilenameMaxSizeChars+1), "text/plain")

	// Assert
	assert.EqualError(t, err, "имя файла не должно превышать 255 символов")
}

// Test_validateBinaryFileSize_FailsWithLargeFile проверяет клиентский лимит размера исходного файла.
func Test_validateBinaryFileSize_FailsWithLargeFile(t *testing.T) {
	// Act
	err := validateBinaryFileSize(plainBinaryRecordFileMaxSizeBytes + 1)

	// Assert
	assert.EqualError(t, err, "размер файла не должен превышать 1024 МиБ")
}
