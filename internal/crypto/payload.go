package crypto

import (
	"encoding/json"
	"fmt"
	"strconv"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

type credentialPayloadJSON struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type textPayloadJSON struct {
	Text string `json:"text"`
}

type cardPayloadJSON struct {
	Number     string `json:"number"`
	HolderName string `json:"holder_name"`
	ExpiresAt  string `json:"expires_at"`
	CVC        string `json:"cvc"`
}

type binaryPayloadJSON struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// EncryptedBinaryRecordData содержит зашифрованные данные бинарной приватной записи.
type EncryptedBinaryRecordData struct {
	EncryptedDEK     model.EncryptedBlob
	EncryptedPayload model.EncryptedBlob
	EncryptedFile    model.EncryptedBlob
}

// EncryptedRecordData содержит зашифрованные данные приватной записи.
type EncryptedRecordData struct {
	EncryptedDEK     model.EncryptedBlob
	EncryptedPayload model.EncryptedBlob
}

// BinaryRecordEncryption содержит зашифрованные метаданные бинарной записи и DEK для потокового шифрования файла.
type BinaryRecordEncryption struct {
	EncryptedDEK     model.EncryptedBlob
	EncryptedPayload model.EncryptedBlob
	dek              []byte
}

// BinaryRecordFileDecryption содержит DEK для расшифровки файла бинарной записи по частям.
type BinaryRecordFileDecryption struct {
	dek []byte
}

// EncryptRecordData шифрует payload приватной записи и DEK, которым он был зашифрован.
func EncryptRecordData[T any](
	masterKey string,
	salt []byte,
	payload T,
) (EncryptedRecordData, error) {
	kek, err := deriveKEK(masterKey, salt)
	if err != nil {
		return EncryptedRecordData{}, fmt.Errorf("failed to derive KEK: %w", err)
	}

	dek, err := generateDEK()
	if err != nil {
		return EncryptedRecordData{}, fmt.Errorf("failed to generate DEK: %w", err)
	}

	encryptedPayload, err := encryptPayload(payload, dek)
	if err != nil {
		return EncryptedRecordData{}, fmt.Errorf("failed to encrypt record payload: %w", err)
	}
	encryptedDEK, err := encryptDEK(dek, kek)
	if err != nil {
		return EncryptedRecordData{}, fmt.Errorf("failed to encrypt DEK: %w", err)
	}

	return EncryptedRecordData{
		EncryptedDEK:     encryptedDEK,
		EncryptedPayload: encryptedPayload,
	}, nil
}

// encryptPayload сериализует payload приватной записи в JSON и шифрует его через DEK.
func encryptPayload[T any](payload T, dek []byte) (model.EncryptedBlob, error) {
	data, err := marshalPayload(payload)
	if err != nil {
		return model.EncryptedBlob{}, err
	}

	encrypted, err := encrypt(data, dek)
	if err != nil {
		return model.EncryptedBlob{}, fmt.Errorf("failed to encrypt payload: %w", err)
	}
	return encrypted, nil
}

// marshalPayload сериализует незашифрованный payload приватной записи в стабильный JSON-формат.
func marshalPayload[T any](payload T) ([]byte, error) {
	jsonPayload, err := payloadToJSON(payload)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(jsonPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return data, nil
}

// payloadToJSON преобразует доменный payload приватной записи в JSON DTO с явно заданным набором полей.
func payloadToJSON[T any](payload T) (any, error) {
	switch p := any(payload).(type) {
	case model.CredentialPayload:
		return credentialPayloadJSON{
			Login:    p.Login,
			Password: p.Password,
		}, nil
	case model.TextPayload:
		return textPayloadJSON{
			Text: p.Text,
		}, nil
	case model.CardPayload:
		return cardPayloadJSON{
			Number:     p.Number,
			HolderName: p.HolderName,
			ExpiresAt:  p.ExpiresAt,
			CVC:        p.CVC,
		}, nil
	case model.BinaryPayload:
		return binaryPayloadJSON{
			Filename:    p.Filename,
			ContentType: p.ContentType,
			Size:        p.Size,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported payload type %T", payload)
	}
}

// DecryptRecordData расшифровывает DEK и payload приватной записи.
func DecryptRecordData[T any](
	masterKey string,
	salt []byte,
	encrypted EncryptedRecordData,
) (T, error) {
	var payload T

	kek, err := deriveKEK(masterKey, salt)
	if err != nil {
		return payload, fmt.Errorf("failed to derive KEK: %w", err)
	}

	dek, err := decryptDEK(encrypted.EncryptedDEK, kek)
	if err != nil {
		return payload, fmt.Errorf("failed to decrypt DEK: %w", err)
	}

	payload, err = decryptPayload[T](encrypted.EncryptedPayload, dek)
	if err != nil {
		return payload, fmt.Errorf("failed to decrypt record payload: %w", err)
	}
	return payload, nil
}

// EncryptBinaryRecordData шифрует payload бинарной приватной записи, содержимое файла и DEK.
func EncryptBinaryRecordData(
	masterKey string,
	salt []byte,
	payload model.BinaryPayload,
	file []byte,
) (EncryptedBinaryRecordData, error) {
	encryption, err := NewBinaryRecordEncryption(masterKey, salt, payload)
	if err != nil {
		return EncryptedBinaryRecordData{}, err
	}

	encryptedFile, err := encryption.EncryptFileChunk(1, file)
	if err != nil {
		return EncryptedBinaryRecordData{}, fmt.Errorf("failed to encrypt binary record file: %w", err)
	}

	return EncryptedBinaryRecordData{
		EncryptedDEK:     encryption.EncryptedDEK,
		EncryptedPayload: encryption.EncryptedPayload,
		EncryptedFile:    encryptedFile,
	}, nil
}

// NewBinaryRecordEncryption подготавливает зашифрованные метаданные бинарной записи и DEK для шифрования файла.
func NewBinaryRecordEncryption(
	masterKey string,
	salt []byte,
	payload model.BinaryPayload,
) (BinaryRecordEncryption, error) {
	kek, err := deriveKEK(masterKey, salt)
	if err != nil {
		return BinaryRecordEncryption{}, fmt.Errorf("failed to derive KEK: %w", err)
	}

	dek, err := generateDEK()
	if err != nil {
		return BinaryRecordEncryption{}, fmt.Errorf("failed to generate DEK: %w", err)
	}

	encryptedPayload, err := encryptPayload(payload, dek)
	if err != nil {
		return BinaryRecordEncryption{}, fmt.Errorf("failed to encrypt binary record payload: %w", err)
	}
	encryptedDEK, err := encryptDEK(dek, kek)
	if err != nil {
		return BinaryRecordEncryption{}, fmt.Errorf("failed to encrypt DEK: %w", err)
	}

	return BinaryRecordEncryption{
		EncryptedDEK:     encryptedDEK,
		EncryptedPayload: encryptedPayload,
		dek:              dek,
	}, nil
}

// EncryptFileChunk шифрует одну часть файла бинарной записи в формате nonce + ciphertext.
func (e BinaryRecordEncryption) EncryptFileChunk(partNumber int32, chunk []byte) (model.EncryptedBlob, error) {
	encrypted, err := encryptWithAAD(chunk, e.dek, binaryFileChunkAAD(partNumber))
	if err != nil {
		return model.EncryptedBlob{}, fmt.Errorf("failed to encrypt binary record file chunk: %w", err)
	}
	return encrypted, nil
}

// DecryptBinaryRecordFile расшифровывает файл бинарной приватной записи через DEK, сохраненный в encrypted DEK.
func DecryptBinaryRecordFile(
	masterKey string,
	salt []byte,
	encryptedDEK model.EncryptedBlob,
	encryptedFile model.EncryptedBlob,
) ([]byte, error) {
	decryption, err := NewBinaryRecordFileDecryption(masterKey, salt, encryptedDEK)
	if err != nil {
		return nil, err
	}

	file, err := decryption.DecryptFileChunk(1, encryptedFile)
	if err != nil {
		file, fallbackErr := decrypt(encryptedFile, decryption.dek)
		if fallbackErr != nil {
			return nil, fmt.Errorf("failed to decrypt binary record file: %w", err)
		}
		return file, nil
	}
	return file, nil
}

// NewBinaryRecordFileDecryption подготавливает DEK для расшифровки файла бинарной записи.
func NewBinaryRecordFileDecryption(
	masterKey string,
	salt []byte,
	encryptedDEK model.EncryptedBlob,
) (BinaryRecordFileDecryption, error) {
	kek, err := deriveKEK(masterKey, salt)
	if err != nil {
		return BinaryRecordFileDecryption{}, fmt.Errorf("failed to derive KEK: %w", err)
	}

	dek, err := decryptDEK(encryptedDEK, kek)
	if err != nil {
		return BinaryRecordFileDecryption{}, fmt.Errorf("failed to decrypt DEK: %w", err)
	}
	return BinaryRecordFileDecryption{dek: dek}, nil
}

// DecryptFileChunk расшифровывает одну часть файла бинарной записи в формате nonce + ciphertext.
func (d BinaryRecordFileDecryption) DecryptFileChunk(
	partNumber int32,
	encryptedChunk model.EncryptedBlob,
) ([]byte, error) {
	file, err := decryptWithAAD(encryptedChunk, d.dek, binaryFileChunkAAD(partNumber))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt binary record file chunk: %w", err)
	}
	return file, nil
}

func binaryFileChunkAAD(partNumber int32) []byte {
	return []byte("binary-file-chunk:" + strconv.FormatInt(int64(partNumber), 10))
}

// DecryptBinaryRecordFileChunks расшифровывает файл, сохраненный как последовательность зашифрованных частей.
func DecryptBinaryRecordFileChunks(
	masterKey string,
	salt []byte,
	encryptedDEK model.EncryptedBlob,
	encryptedFile model.EncryptedBlob,
	plainSize int64,
	plainChunkSize int64,
) ([]byte, error) {
	if plainSize <= 0 {
		return nil, fmt.Errorf("plain size is invalid")
	}
	if plainChunkSize <= 0 {
		return nil, fmt.Errorf("plain chunk size is invalid")
	}

	if int64(len(encryptedFile.Data)) == plainSize+EncryptedBlobOverhead() {
		return DecryptBinaryRecordFile(masterKey, salt, encryptedDEK, encryptedFile)
	}

	expectedSize, err := EncryptedChunkedBlobSize(plainSize, plainChunkSize)
	if err != nil {
		return nil, err
	}
	if int64(len(encryptedFile.Data)) != expectedSize {
		return nil, fmt.Errorf("encrypted file size does not match declared plain size")
	}

	decryption, err := NewBinaryRecordFileDecryption(masterKey, salt, encryptedDEK)
	if err != nil {
		return nil, err
	}

	file := make([]byte, 0, plainSize)
	offset := 0
	remainingPlain := plainSize
	for partNumber := int32(1); remainingPlain > 0; partNumber++ {
		plainPartSize := min(plainChunkSize, remainingPlain)
		encryptedPartSize := plainPartSize + EncryptedBlobOverhead()
		to := offset + int(encryptedPartSize)
		if to > len(encryptedFile.Data) {
			return nil, fmt.Errorf("encrypted file chunk is incomplete")
		}
		chunk, err := decryption.DecryptFileChunk(partNumber, model.EncryptedBlob{Data: encryptedFile.Data[offset:to]})
		if err != nil {
			return nil, err
		}
		if int64(len(chunk)) != plainPartSize {
			return nil, fmt.Errorf("decrypted file chunk size does not match declared size")
		}
		file = append(file, chunk...)
		offset = to
		remainingPlain -= plainPartSize
	}
	return file, nil
}

// decryptPayload расшифровывает payload приватной записи через DEK и десериализует JSON в доменную структуру.
func decryptPayload[T any](encrypted model.EncryptedBlob, dek []byte) (T, error) {
	var payload T

	data, err := decrypt(encrypted, dek)
	if err != nil {
		return payload, fmt.Errorf("failed to decrypt payload: %w", err)
	}

	payload, err = unmarshalPayload[T](data)
	if err != nil {
		return payload, err
	}
	return payload, nil
}

// unmarshalPayload десериализует payload приватной записи, заданный в JSON-формате, в доменную структуру.
func unmarshalPayload[T any](data []byte) (T, error) {
	var payload T

	switch any(payload).(type) {
	case model.CredentialPayload:
		var jsonPayload credentialPayloadJSON
		if err := decodePayloadJSON(data, &jsonPayload); err != nil {
			return payload, err
		}
		return any(model.CredentialPayload{
			Login:    jsonPayload.Login,
			Password: jsonPayload.Password,
		}).(T), nil
	case model.TextPayload:
		var jsonPayload textPayloadJSON
		if err := decodePayloadJSON(data, &jsonPayload); err != nil {
			return payload, err
		}
		return any(model.TextPayload{
			Text: jsonPayload.Text,
		}).(T), nil
	case model.CardPayload:
		var jsonPayload cardPayloadJSON
		if err := decodePayloadJSON(data, &jsonPayload); err != nil {
			return payload, err
		}
		return any(model.CardPayload{
			Number:     jsonPayload.Number,
			HolderName: jsonPayload.HolderName,
			ExpiresAt:  jsonPayload.ExpiresAt,
			CVC:        jsonPayload.CVC,
		}).(T), nil
	case model.BinaryPayload:
		var jsonPayload binaryPayloadJSON
		if err := decodePayloadJSON(data, &jsonPayload); err != nil {
			return payload, err
		}
		return any(model.BinaryPayload{
			Filename:    jsonPayload.Filename,
			ContentType: jsonPayload.ContentType,
			Size:        jsonPayload.Size,
		}).(T), nil
	default:
		return payload, fmt.Errorf("unsupported payload type %T", payload)
	}
}

func decodePayloadJSON(data []byte, payload any) error {
	if err := json.Unmarshal(data, payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	return nil
}
