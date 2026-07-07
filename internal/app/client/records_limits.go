package client

import (
	"fmt"
	"unicode/utf8"
)

const (
	recordTitleMaxSizeChars           = 128
	recordDescriptionMaxSizeChars     = 1024
	textRecordTextMaxSizeBytes        = 256 * 1024
	credentialLoginMaxSizeChars       = 128
	credentialPasswordMaxSizeChars    = 256
	cardHolderNameMaxSizeChars        = 32
	binaryFilenameMaxSizeChars        = 255
	binaryContentTypeMaxSizeChars     = 255
	plainBinaryRecordFileMaxSizeBytes = 1024 * 1024 * 1024
)

func validateRecordMetadataSize(title string, description string) error {
	if utf8.RuneCountInString(title) > recordTitleMaxSizeChars {
		return fmt.Errorf("название записи не должно превышать %d символов", recordTitleMaxSizeChars)
	}
	if utf8.RuneCountInString(description) > recordDescriptionMaxSizeChars {
		return fmt.Errorf("описание записи не должно превышать %d символов", recordDescriptionMaxSizeChars)
	}
	return nil
}

func validateTextRecordPayloadSize(text string) error {
	if len([]byte(text)) > textRecordTextMaxSizeBytes {
		return fmt.Errorf("текст записи не должен превышать %d кБ", textRecordTextMaxSizeBytes/1024)
	}
	return nil
}

func validateCredentialPayloadSize(login string, password string) error {
	if utf8.RuneCountInString(login) > credentialLoginMaxSizeChars {
		return fmt.Errorf("логин записи с учетными данными не должен превышать %d символов", credentialLoginMaxSizeChars)
	}
	if utf8.RuneCountInString(password) > credentialPasswordMaxSizeChars {
		return fmt.Errorf("пароль записи с учетными данными не должен превышать %d символов", credentialPasswordMaxSizeChars)
	}
	return nil
}

func validateCardHolderNameSize(holderName string) error {
	if utf8.RuneCountInString(holderName) > cardHolderNameMaxSizeChars {
		return fmt.Errorf("имя владельца карты не должно превышать %d символа", cardHolderNameMaxSizeChars)
	}
	return nil
}

func validateBinaryPayloadSize(filename string, contentType string) error {
	if utf8.RuneCountInString(filename) > binaryFilenameMaxSizeChars {
		return fmt.Errorf("имя файла не должно превышать %d символов", binaryFilenameMaxSizeChars)
	}
	if utf8.RuneCountInString(contentType) > binaryContentTypeMaxSizeChars {
		return fmt.Errorf("тип содержимого файла не должен превышать %d символов", binaryContentTypeMaxSizeChars)
	}
	return nil
}

func validateBinaryFileSize(size int64) error {
	if size < 0 {
		return fmt.Errorf("размер файла некорректен")
	}
	if size > plainBinaryRecordFileMaxSizeBytes {
		return fmt.Errorf("размер файла не должен превышать %d МБ", plainBinaryRecordFileMaxSizeBytes/1024/1024)
	}
	return nil
}
