package buildinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNew_FillsMissingValues проверяет подстановку N/A для отсутствующих метаданных сборки.
func TestNew_FillsMissingValues(t *testing.T) {
	// Act
	info := New("", "")

	// Assert
	assert.Equal(t, Info{Version: "N/A", Date: "N/A"}, info)
}

// TestInfo_Title проверяет формат заголовка приложения с метаданными сборки.
func TestInfo_Title(t *testing.T) {
	// Arrange
	info := New("1.2.3", "2026-07-05")

	// Act
	title := info.Title("GophKeeper")

	// Assert
	assert.Equal(t, "GophKeeper (версия: 1.2.3, дата сборки: 2026-07-05)", title)
}
