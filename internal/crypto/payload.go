package crypto

import (
	"encoding/json"
	"fmt"

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

// EncryptPayload сериализует payload приватной записи в JSON и шифрует его через DEK.
func EncryptPayload[T any](payload T, dek []byte) (model.EncryptedBlob, error) {
	data, err := marshalPayload(payload)
	if err != nil {
		return model.EncryptedBlob{}, err
	}

	encrypted, err := Encrypt(data, dek)
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

// DecryptPayload расшифровывает payload приватной записи через DEK и десериализует JSON в доменную структуру.
func DecryptPayload[T any](encrypted model.EncryptedBlob, dek []byte) (T, error) {
	var payload T

	data, err := Decrypt(encrypted, dek)
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
