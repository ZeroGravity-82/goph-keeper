package client

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/crypto"
	"zerogravity-82/goph-keeper/internal/domain/model"
)

// Test_binaryFileStreamDecryption_WritesPlainChunks проверяет потоковую расшифровку зашифрованных данных в получатель.
func Test_binaryFileStreamDecryption_WritesPlainChunks(t *testing.T) {
	// Arrange
	file := []byte("abcdefg")
	encryptedFile, decryption := prepareBinaryFileStreamDecryption(t, file, 4)
	dst := bytes.NewBuffer(nil)
	stream := binaryFileStreamDecryption{
		decryption:       decryption,
		expectedSHA256:   encryptedFileSHA256Bytes(encryptedFile),
		plainSize:        int64(len(file)),
		plainPartSize:    4,
		decryptedFileDst: dst,
	}
	stream.Reset()

	// Act
	for len(encryptedFile) > 0 {
		size := min(5, len(encryptedFile))
		require.NoError(t, stream.Write(encryptedFile[:size]))
		encryptedFile = encryptedFile[size:]
	}
	err := stream.Finish()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, file, dst.Bytes())
}

func prepareBinaryFileStreamDecryption(
	t *testing.T,
	file []byte,
	plainPartSize int64,
) ([]byte, crypto.BinaryRecordFileDecryption) {
	t.Helper()

	masterKey := "мастер-ключ"
	salt := []byte("1234567890abcdef")
	encryption, err := crypto.NewBinaryRecordEncryption(
		masterKey,
		salt,
		model.BinaryPayload{Filename: "archive.bin", ContentType: "application/octet-stream", Size: int64(len(file))},
	)
	require.NoError(t, err)

	var encryptedFile []byte
	for partNumber, offset := int32(1), 0; offset < len(file); partNumber++ {
		to := min(offset+int(plainPartSize), len(file))
		encryptedPart, err := encryption.EncryptFileChunk(partNumber, file[offset:to])
		require.NoError(t, err)
		encryptedFile = append(encryptedFile, encryptedPart.Data...)
		offset = to
	}

	decryption, err := crypto.NewBinaryRecordFileDecryption(masterKey, salt, encryption.EncryptedDEK)
	require.NoError(t, err)
	return encryptedFile, decryption
}

// Test_binaryFileStreamDecryption_FailWithChecksumMismatch проверяет ошибку при несовпадении контрольной суммы.
func Test_binaryFileStreamDecryption_FailWithChecksumMismatch(t *testing.T) {
	// Arrange
	file := []byte("abcdefg")
	encryptedFile, decryption := prepareBinaryFileStreamDecryption(t, file, 4)
	dst := bytes.NewBuffer(nil)
	stream := binaryFileStreamDecryption{
		decryption:       decryption,
		expectedSHA256:   strings.Repeat("0", 64),
		plainSize:        int64(len(file)),
		plainPartSize:    4,
		decryptedFileDst: dst,
	}
	stream.Reset()

	// Act
	require.NoError(t, stream.Write(encryptedFile))
	err := stream.Finish()

	// Assert
	require.Error(t, err)
	assert.Equal(t, "контрольная сумма зашифрованного файла не совпала", err.Error())
}

// Test_binaryFileStreamDecryption_FailWithIncompleteStream проверяет ошибку при обрыве потока зашифрованных данных.
func Test_binaryFileStreamDecryption_FailWithIncompleteStream(t *testing.T) {
	// Arrange
	file := []byte("abcdefg")
	encryptedFile, decryption := prepareBinaryFileStreamDecryption(t, file, 4)
	dst := bytes.NewBuffer(nil)
	stream := binaryFileStreamDecryption{
		decryption:       decryption,
		expectedSHA256:   encryptedFileSHA256Bytes(encryptedFile),
		plainSize:        int64(len(file)),
		plainPartSize:    4,
		decryptedFileDst: dst,
	}
	stream.Reset()

	// Act
	require.NoError(t, stream.Write(encryptedFile[:len(encryptedFile)-1]))
	err := stream.Finish()

	// Assert
	require.Error(t, err)
	assert.Equal(t, "зашифрованный файл получен не полностью", err.Error())
}
