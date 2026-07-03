package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_validateAndNormalizeCard_Valid проверяет нормализацию корректных данных банковской карты.
func Test_validateAndNormalizeCard_Valid(t *testing.T) {
	// Act
	card, err := validateAndNormalizeCard("4111 1111 1111 1111", "ivan ivanov", "12/30", "123")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "4111111111111111", card.number)
	assert.Equal(t, "IVAN IVANOV", card.holderName)
	assert.Equal(t, "12/30", card.expiresAt)
	assert.Equal(t, "123", card.cvc)
}

// Test_validateAndNormalizeCard_InvalidNumberChecksum проверяет, что полная подготовка карты к сохранению отклоняет
// номер с неверной контрольной суммой.
func Test_validateAndNormalizeCard_InvalidNumberChecksum(t *testing.T) {
	// Act
	_, err := validateAndNormalizeCard("4111111111111112", "IVAN IVANOV", "12/30", "123")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "неверный номер карты", err.Error())
}

// TestValidateCardNumber_InvalidChecksum проверяет валидацию только поля с номером номера карты.
func TestValidateCardNumber_InvalidChecksum(t *testing.T) {
	// Act
	err := ValidateCardNumber("4254 3255 8845 1111")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "неверный номер карты", err.Error())
}

// Test_validateAndNormalizeCard_InvalidNumberFormat проверяет ошибку для номера карты с некорректной группировкой.
func Test_validateAndNormalizeCard_InvalidNumberFormat(t *testing.T) {
	// Act
	_, err := validateAndNormalizeCard("4111 11111111 1111", "IVAN IVANOV", "12/30", "123")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "номер карты должен содержать 16 цифр без пробелов или 4 группы по 4 цифры через пробел", err.Error())
}

// Test_validateAndNormalizeCard_InvalidHolderName проверяет запрет нелатинских символов в имени владельца.
func Test_validateAndNormalizeCard_InvalidHolderName(t *testing.T) {
	// Act
	_, err := validateAndNormalizeCard("4111111111111111", "ИВАН ИВАНОВ", "12/30", "123")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "имя владельца карты должно содержать только латинские буквы и пробелы", err.Error())
}

// Test_validateAndNormalizeCard_InvalidExpiration проверяет строгий формат срока действия карты.
func Test_validateAndNormalizeCard_InvalidExpiration(t *testing.T) {
	// Act
	_, err := validateAndNormalizeCard("4111111111111111", "IVAN IVANOV", "13/30", "123")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "срок действия карты должен быть в формате ММ/ГГ", err.Error())
}

// Test_validateAndNormalizeCard_InvalidCVC проверяет строгий формат CVC.
func Test_validateAndNormalizeCard_InvalidCVC(t *testing.T) {
	// Act
	_, err := validateAndNormalizeCard("4111111111111111", "IVAN IVANOV", "12/30", "1234")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "CVC должен содержать ровно 3 цифры", err.Error())
}
