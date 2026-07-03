package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// downloadSelectedBinaryFile скачивает файл выбранной бинарной записи в указанную директорию.
func downloadSelectedBinaryFile(
	ctx context.Context,
	app *clientApp.App,
	item clientApp.RecordListItem,
	reader *bufio.Reader,
	out io.Writer,
) error {
	printCancelHint(out)
	outputDir, err := promptRequiredRetryCancelable(
		reader,
		out,
		"Директория для сохранения файла: ",
		"директория для сохранения файла обязательна",
	)
	if err != nil {
		return err
	}
	if err = ensureDirectory(outputDir); err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	file, err := app.DownloadBinaryFile(callCtx, item.RecordID)
	if err != nil {
		return err
	}
	outputPath := filepath.Join(outputDir, file.Filename)
	if err = confirmOutputFileOverwrite(reader, out, outputPath); err != nil {
		return err
	}
	if err = os.WriteFile(outputPath, file.Data, 0o600); err != nil {
		return fmt.Errorf("не удалось сохранить файл: %w", err)
	}
	fmt.Fprintf(out, "файл сохранен: %s\n", outputPath)
	fmt.Fprintf(out, "исходное имя: %s\n", file.Filename)
	fmt.Fprintf(out, "MIME-тип: %s\n", file.ContentType)
	fmt.Fprintf(out, "размер: %d байт\n", file.DeclaredSize)
	return nil
}

// confirmOutputFileOverwrite запрашивает подтверждение, если файл для сохранения уже существует.
func confirmOutputFileOverwrite(reader *bufio.Reader, out io.Writer, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("не удалось проверить файл для сохранения: %w", err)
	}
	if info.IsDir() {
		return errors.New("путь для сохранения файла уже существует и является директорией")
	}
	confirmed, err := confirm(reader, out, fmt.Sprintf("Файл %q уже существует. Перезаписать?", path))
	if err != nil {
		return err
	}
	if !confirmed {
		return errActionCanceled
	}
	return nil
}

// ensureDirectory проверяет, что путь существует и указывает на директорию.
func ensureDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("не удалось проверить директорию для сохранения файла: %w", err)
	}
	if !info.IsDir() {
		return errors.New("путь для сохранения файла должен быть директорией")
	}
	return nil
}

// replaceSelectedBinaryFile заменяет файл выбранной бинарной записи и обновляет ее метаданные в кеше.
func replaceSelectedBinaryFile(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
	reader *bufio.Reader,
	out io.Writer,
) error {
	record, err := cachedBinary(ctx, app, state, item)
	if err != nil {
		return err
	}
	printCancelHint(out)
	title, err := promptWithDefaultCancelable(reader, out, "Новое название", record.Title)
	if err != nil {
		return err
	}
	description, err := promptWithDefaultCancelable(reader, out, "Новое описание", record.Description)
	if err != nil {
		return err
	}
	path, err := promptRequiredRetryCancelable(reader, out, "Путь к новому файлу: ", "путь к новому файлу обязателен")
	if err != nil {
		return err
	}
	file, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("не удалось прочитать файл: %w", err)
	}
	filename := filepath.Base(path)
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	updated, err := app.UpdateBinary(callCtx, clientApp.UpdateBinaryInput{
		RecordID:        record.RecordID,
		ExpectedVersion: record.Version,
		Title:           title,
		Description:     description,
		Filename:        filename,
		ContentType:     contentType,
		File:            file,
	})
	if err != nil {
		return err
	}
	record.Title = title
	record.Description = description
	record.Filename = filename
	record.ContentType = contentType
	record.Size = int64(len(file))
	record.Version = updated.Version
	state.storeBinary(record)
	fmt.Fprintf(out, "заменен файл бинарной приватной записи: %s\n", record.Title)
	return nil
}

// createBinary запрашивает метаданные и путь к файлу, затем создает зашифрованную файловую приватную запись.
func createBinary(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	printCancelHint(out)
	title, err := promptRequiredCancelable(reader, out, "Название: ")
	if err != nil {
		return err
	}
	description, err := promptCancelable(reader, out, "Описание: ")
	if err != nil {
		return err
	}
	path, err := promptRequiredRetryCancelable(reader, out, "Путь к файлу: ", "путь к файлу обязателен")
	if err != nil {
		return err
	}

	file, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("не удалось прочитать файл: %w", err)
	}

	filename := filepath.Base(path)
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	_, err = app.CreateBinary(callCtx, clientApp.CreateBinaryInput{
		Title:       title,
		Description: description,
		Filename:    filename,
		ContentType: contentType,
		File:        file,
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "создана файловая приватная запись")
	return nil
}
