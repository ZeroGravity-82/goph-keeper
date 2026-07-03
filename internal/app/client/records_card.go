package client

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"

	"zerogravity-82/goph-keeper/internal/crypto"
	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/pb"
)

// CreateCardInput содержит данные для создания приватной записи с данными банковской карты.
type CreateCardInput struct {
	Title       string
	Description string
	Number      string
	HolderName  string
	ExpiresAt   string
	CVC         string
}

// CardRecord содержит расшифрованную приватную запись с данными банковской карты.
type CardRecord struct {
	RecordID    string
	Version     int64
	Title       string
	Description string
	Number      string
	HolderName  string
	ExpiresAt   string
	CVC         string
}

// UpdateCardInput содержит данные для обновления приватной записи с данными банковской карты.
type UpdateCardInput struct {
	RecordID        string
	ExpectedVersion int64
	Title           string
	Description     string
	Number          string
	HolderName      string
	ExpiresAt       string
	CVC             string
}

// CreateCard шифрует payload на клиенте и создает приватную запись с данными банковской карты.
func (a *App) CreateCard(ctx context.Context, in CreateCardInput) (CreateRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return CreateRecordOutput{}, err
	}
	normalizedCard, err := validateAndNormalizeCard(in.Number, in.HolderName, in.ExpiresAt, in.CVC)
	if err != nil {
		return CreateRecordOutput{}, err
	}

	encrypted, err := crypto.EncryptRecordData(a.masterKey, a.session.MasterKeySalt, model.CardPayload{
		Number:     normalizedCard.number,
		HolderName: normalizedCard.holderName,
		ExpiresAt:  normalizedCard.expiresAt,
		CVC:        normalizedCard.cvc,
	})
	if err != nil {
		return CreateRecordOutput{}, fmt.Errorf("не удалось зашифровать приватную запись банковской карты: %w", err)
	}

	recordType := pb.RecordType_RECORD_TYPE_CARD
	var resp *pb.CreateRecordResponse
	err = a.withAccessTokenRefresh(ctx, func(ctx context.Context) error {
		var err error
		resp, err = a.records.CreateRecord(ctx, pb.CreateRecordRequest_builder{
			Type:             &recordType,
			Title:            &in.Title,
			Description:      &in.Description,
			EncryptedDek:     encrypted.EncryptedDEK.Data,
			EncryptedPayload: encrypted.EncryptedPayload.Data,
		}.Build())
		return err
	})
	if err != nil {
		return CreateRecordOutput{}, rpcError(
			err,
			"не удалось создать приватную запись банковской карты",
			map[codes.Code]string{
				codes.Unauthenticated: "сессия недействительна, войдите снова",
				codes.InvalidArgument: "некорректные данные приватной записи",
			},
		)
	}

	return CreateRecordOutput{RecordID: resp.GetRecordId(), Version: resp.GetVersion()}, nil
}

// UpdateCard шифрует обновленный payload на клиенте и обновляет приватную запись с данными банковской карты.
func (a *App) UpdateCard(ctx context.Context, in UpdateCardInput) (UpdateRecordOutput, error) {
	if err := a.requireSession(); err != nil {
		return UpdateRecordOutput{}, err
	}
	normalizedCard, err := validateAndNormalizeCard(in.Number, in.HolderName, in.ExpiresAt, in.CVC)
	if err != nil {
		return UpdateRecordOutput{}, err
	}

	encrypted, err := crypto.EncryptRecordData(a.masterKey, a.session.MasterKeySalt, model.CardPayload{
		Number:     normalizedCard.number,
		HolderName: normalizedCard.holderName,
		ExpiresAt:  normalizedCard.expiresAt,
		CVC:        normalizedCard.cvc,
	})
	if err != nil {
		return UpdateRecordOutput{}, fmt.Errorf("не удалось зашифровать приватную запись банковской карты: %w", err)
	}

	return a.updateRecord(
		ctx,
		in.RecordID,
		in.Title,
		in.Description,
		encrypted.EncryptedDEK.Data,
		encrypted.EncryptedPayload.Data,
		in.ExpectedVersion,
	)
}

// GetCard получает приватную запись банковской карты и расшифровывает payload на клиенте.
func (a *App) GetCard(ctx context.Context, recordID string) (CardRecord, error) {
	var resp *pb.GetRecordResponse
	err := a.withAccessTokenRefreshRetry(ctx, func(ctx context.Context) error {
		var err error
		resp, err = a.records.GetRecord(ctx, pb.GetRecordRequest_builder{RecordId: &recordID}.Build())
		return err
	})
	if err != nil {
		return CardRecord{}, rpcError(
			err,
			"не удалось получить приватную запись банковской карты",
			map[codes.Code]string{
				codes.Unauthenticated: "сессия недействительна, войдите снова",
				codes.InvalidArgument: "некорректный идентификатор приватной записи",
				codes.NotFound:        "приватная запись не найдена",
			},
		)
	}
	record := resp.GetRecord()
	if record == nil {
		return CardRecord{}, errors.New("сервер вернул пустую приватную запись")
	}
	if record.GetType() != pb.RecordType_RECORD_TYPE_CARD {
		return CardRecord{}, fmt.Errorf("приватная запись %s не содержит данные банковской карты", recordID)
	}

	payload, err := crypto.DecryptRecordData[model.CardPayload](
		a.masterKey,
		a.session.MasterKeySalt,
		crypto.EncryptedRecordData{
			EncryptedDEK:     model.EncryptedBlob{Data: record.GetEncryptedDek()},
			EncryptedPayload: model.EncryptedBlob{Data: record.GetEncryptedPayload()},
		},
	)
	if err != nil {
		return CardRecord{}, fmt.Errorf("не удалось расшифровать приватную запись банковской карты: %w", err)
	}

	return CardRecord{
		RecordID:    record.GetRecordId(),
		Version:     record.GetVersion(),
		Title:       record.GetTitle(),
		Description: record.GetDescription(),
		Number:      payload.Number,
		HolderName:  payload.HolderName,
		ExpiresAt:   payload.ExpiresAt,
		CVC:         payload.CVC,
	}, nil
}

type normalizedCardData struct {
	number     string
	holderName string
	expiresAt  string
	cvc        string
}

func validateAndNormalizeCard(number string, holderName string, expiresAt string, cvc string) (normalizedCardData, error) {
	number = strings.TrimSpace(number)
	holderName = strings.ToUpper(strings.TrimSpace(holderName))
	expiresAt = strings.TrimSpace(expiresAt)
	cvc = strings.TrimSpace(cvc)

	normalizedNumber, err := NormalizeCardNumber(number)
	if err != nil {
		return normalizedCardData{}, err
	}
	if err = ValidateCardNumber(normalizedNumber); err != nil {
		return normalizedCardData{}, err
	}
	normalizedHolderName, err := NormalizeCardHolderName(holderName)
	if err != nil {
		return normalizedCardData{}, err
	}
	if err = ValidateCardExpiration(expiresAt); err != nil {
		return normalizedCardData{}, err
	}
	if err = ValidateCardCVC(cvc); err != nil {
		return normalizedCardData{}, err
	}

	return normalizedCardData{number: normalizedNumber, holderName: normalizedHolderName, expiresAt: expiresAt, cvc: cvc}, nil
}

// ValidateCardNumber проверяет номер банковской карты по контрольной сумме.
func ValidateCardNumber(number string) error {
	normalizedNumber, err := NormalizeCardNumber(number)
	if err != nil {
		return err
	}
	if !validLuhn(normalizedNumber) {
		return errors.New("неверный номер карты")
	}
	return nil
}

// NormalizeCardNumber проверяет номер банковской карты и возвращает его без пробелов.
func NormalizeCardNumber(number string) (string, error) {
	number = strings.TrimSpace(number)
	if len(number) == 16 && isDigits(number) {
		return number, nil
	}
	if len(number) == 19 &&
		number[4] == ' ' &&
		number[9] == ' ' &&
		number[14] == ' ' &&
		isDigits(number[:4]) &&
		isDigits(number[5:9]) &&
		isDigits(number[10:14]) &&
		isDigits(number[15:19]) {
		return strings.ReplaceAll(number, " ", ""), nil
	}
	return "", errors.New("номер карты должен содержать 16 цифр без пробелов или 4 группы по 4 цифры через пробел")
}

// NormalizeCardHolderName проверяет имя владельца банковской карты и возвращает его в верхнем регистре.
func NormalizeCardHolderName(holderName string) (string, error) {
	holderName = strings.ToUpper(strings.TrimSpace(holderName))
	if holderName == "" || !isLatinLettersAndSpaces(holderName) {
		return "", errors.New("имя владельца карты должно содержать только латинские буквы и пробелы")
	}
	return holderName, nil
}

// ValidateCardExpiration проверяет срок действия банковской карты в формате ММ/ГГ.
func ValidateCardExpiration(expiresAt string) error {
	expiresAt = strings.TrimSpace(expiresAt)
	if !validCardExpiration(expiresAt) {
		return errors.New("срок действия карты должен быть в формате ММ/ГГ")
	}
	return nil
}

// ValidateCardCVC проверяет CVC банковской карты.
func ValidateCardCVC(cvc string) error {
	cvc = strings.TrimSpace(cvc)
	if len(cvc) != 3 || !isDigits(cvc) {
		return errors.New("CVC должен содержать ровно 3 цифры")
	}
	return nil
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validLuhn(number string) bool {
	sum := 0
	double := false
	for i := len(number) - 1; i >= 0; i-- {
		digit := int(number[i] - '0')
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}
	return sum%10 == 0
}

func isLatinLettersAndSpaces(value string) bool {
	for _, r := range value {
		if r == ' ' {
			continue
		}
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func validCardExpiration(value string) bool {
	if len(value) != 5 || value[2] != '/' {
		return false
	}
	month := value[:2]
	year := value[3:]
	if !isDigits(month) || !isDigits(year) {
		return false
	}
	return month >= "01" && month <= "12"
}
