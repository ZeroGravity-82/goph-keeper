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

// MarshalPayload сериализует незашифрованный payload приватной записи в стабильный JSON-формат.
func MarshalPayload[T any](payload T) ([]byte, error) {
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

// UnmarshalPayload десериализует payload приватной записи, заданный в JSON-формате, в доменную структуру.
func UnmarshalPayload[T any](data []byte) (T, error) {
	var payload T

	switch any(payload).(type) {
	case model.CredentialPayload:
		var jsonPayload credentialPayloadJSON
		if err := unmarshalPayload(data, &jsonPayload); err != nil {
			return payload, err
		}
		return any(model.CredentialPayload{
			Login:    jsonPayload.Login,
			Password: jsonPayload.Password,
		}).(T), nil
	case model.TextPayload:
		var jsonPayload textPayloadJSON
		if err := unmarshalPayload(data, &jsonPayload); err != nil {
			return payload, err
		}
		return any(model.TextPayload{
			Text: jsonPayload.Text,
		}).(T), nil
	case model.CardPayload:
		var jsonPayload cardPayloadJSON
		if err := unmarshalPayload(data, &jsonPayload); err != nil {
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
		if err := unmarshalPayload(data, &jsonPayload); err != nil {
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

func unmarshalPayload(data []byte, payload any) error {
	if err := json.Unmarshal(data, payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	return nil
}
