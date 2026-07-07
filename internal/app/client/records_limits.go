package client

import (
	"fmt"
	"unicode/utf8"
)

const (
	bytesInKB                     = 1024
	bytesInMB                     = 1024 * 1024
	recordTitleMaxChars           = 128
	recordDescriptionMaxChars     = 1024
	textRecordTextMaxBytes        = 256 * 1024
	credentialLoginMaxChars       = 128
	credentialPasswordMaxChars    = 256
	cardHolderNameMaxChars        = 32
	binaryFilenameMaxChars        = 255
	binaryContentTypeMaxChars     = 255
	plainBinaryRecordFileMaxBytes = 100 * 1024 * 1024
)

func validateRecordMetadataSize(title string, description string) error {
	if utf8.RuneCountInString(title) > recordTitleMaxChars {
		return fmt.Errorf("название записи не должно превышать %d символов", recordTitleMaxChars)
	}
	if utf8.RuneCountInString(description) > recordDescriptionMaxChars {
		return fmt.Errorf("описание записи не должно превышать %d символов", recordDescriptionMaxChars)
	}
	return nil
}

func validateTextRecordPayloadSize(text string) error {
	if len([]byte(text)) > textRecordTextMaxBytes {
		return fmt.Errorf("текст записи не должен превышать %d кБ", kilobytes(textRecordTextMaxBytes))
	}
	return nil
}

func validateCredentialPayloadSize(login string, password string) error {
	if utf8.RuneCountInString(login) > credentialLoginMaxChars {
		return fmt.Errorf("логин записи с учетными данными не должен превышать %d символов", credentialLoginMaxChars)
	}
	if utf8.RuneCountInString(password) > credentialPasswordMaxChars {
		return fmt.Errorf("пароль записи с учетными данными не должен превышать %d символов", credentialPasswordMaxChars)
	}
	return nil
}

func kilobytes(bytes int) int {
	return bytes / bytesInKB
}

func validateCardHolderNameSize(holderName string) error {
	if utf8.RuneCountInString(holderName) > cardHolderNameMaxChars {
		return fmt.Errorf("имя владельца карты не должно превышать %d символа", cardHolderNameMaxChars)
	}
	return nil
}

func validateBinaryPayloadSize(filename string, contentType string) error {
	if utf8.RuneCountInString(filename) > binaryFilenameMaxChars {
		return fmt.Errorf("имя файла не должно превышать %d символов", binaryFilenameMaxChars)
	}
	if utf8.RuneCountInString(contentType) > binaryContentTypeMaxChars {
		return fmt.Errorf("тип содержимого файла не должен превышать %d символов", binaryContentTypeMaxChars)
	}
	return nil
}

func validateBinaryFileSize(file []byte) error {
	if len(file) > plainBinaryRecordFileMaxBytes {
		return fmt.Errorf("размер файла не должен превышать %d МБ", megabytes(plainBinaryRecordFileMaxBytes))
	}
	return nil
}

func megabytes(bytes int) int {
	return bytes / bytesInMB
}
