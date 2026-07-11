package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"os"
	"os/signal"
	"path/filepath"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

const minBinaryUploadProgressSizeBytes int64 = 1024 * 1024

// binaryTransferContext создает контекст, который отменяется по Ctrl+C во время передачи файла.
func binaryTransferContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, os.Interrupt)
}

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

	tempFile, err := os.CreateTemp(outputDir, ".gophkeeper-download-*")
	if err != nil {
		return fmt.Errorf("не удалось создать временный файл: %w", err)
	}
	tempPath := tempFile.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	transferCtx, stopTransfer := binaryTransferContext(ctx)
	file, err := app.DownloadBinaryFile(transferCtx, item.RecordID, tempFile)
	stopTransfer()
	closeErr := tempFile.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("не удалось закрыть временный файл: %w", closeErr)
	}
	outputPath := filepath.Join(outputDir, file.Filename)
	if err = confirmOutputFileOverwrite(reader, out, outputPath); err != nil {
		return err
	}
	if err = replaceDownloadedFile(tempPath, outputPath); err != nil {
		return fmt.Errorf("не удалось сохранить файл: %w", err)
	}
	removeTemp = false
	_, _ = fmt.Fprintf(out, "файл сохранен: %s\n", outputPath)
	_, _ = fmt.Fprintf(out, "исходное имя: %s\n", file.Filename)
	_, _ = fmt.Fprintf(out, "MIME-тип: %s\n", file.ContentType)
	_, _ = fmt.Fprintf(out, "размер: %d байт\n", file.DeclaredSize)
	return nil
}

// replaceDownloadedFile переносит полностью скачанный и проверенный временный файл в целевой путь.
func replaceDownloadedFile(tempPath string, outputPath string) error {
	if err := os.Remove(outputPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(tempPath, outputPath)
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
	file, fileSize, err := openInputFile(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	filename := filepath.Base(path)
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	progress := newBinaryUploadProgressPrinter(out)
	defer progress.finish()
	transferCtx, stopTransfer := binaryTransferContext(ctx)
	updated, err := app.UpdateBinary(transferCtx, clientApp.UpdateBinaryInput{
		RecordID:        record.RecordID,
		ExpectedVersion: record.Version,
		Title:           title,
		Description:     description,
		Filename:        filename,
		ContentType:     contentType,
		File:            file,
		FileSize:        fileSize,
		OnProgress:      progress.update,
	})
	stopTransfer()
	if err != nil {
		refreshRecordsAfterBinaryFailure(ctx, app, state)
		return err
	}
	record.Title = title
	record.Description = description
	record.Filename = filename
	record.ContentType = contentType
	record.Size = fileSize
	record.UploadStatus = clientApp.UploadStatusUploaded
	record.Version = updated.Version
	state.storeBinary(record)
	progress.finish()
	_, _ = fmt.Fprintf(out, "заменен файл бинарной приватной записи: %s\n", record.Title)
	return nil
}

func openInputFile(path string) (*os.File, int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, fmt.Errorf("не удалось проверить файл: %w", err)
	}
	if info.IsDir() {
		return nil, 0, errors.New("путь к файлу указывает на директорию")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("не удалось открыть файл: %w", err)
	}
	return file, info.Size(), nil
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

	file, fileSize, err := openInputFile(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	filename := filepath.Base(path)
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	progress := newBinaryUploadProgressPrinter(out)
	defer progress.finish()
	transferCtx, stopTransfer := binaryTransferContext(ctx)
	_, err = app.CreateBinary(transferCtx, clientApp.CreateBinaryInput{
		Title:       title,
		Description: description,
		Filename:    filename,
		ContentType: contentType,
		File:        file,
		FileSize:    fileSize,
		OnProgress:  progress.update,
	})
	stopTransfer()
	if err != nil {
		return err
	}
	progress.finish()
	_, _ = fmt.Fprintln(out, "создана файловая приватная запись")
	return nil
}

// binaryUploadProgressPrinter печатает прогресс загрузки бинарного файла в терминал.
type binaryUploadProgressPrinter struct {
	out     io.Writer
	printed bool
}

// newBinaryUploadProgressPrinter создает принтер прогресса загрузки файла в одну обновляемую строку.
func newBinaryUploadProgressPrinter(out io.Writer) *binaryUploadProgressPrinter {
	return &binaryUploadProgressPrinter{out: out}
}

// update печатает текущий прогресс загрузки исходного файла.
func (p *binaryUploadProgressPrinter) update(progress clientApp.BinaryUploadProgress) {
	if progress.TotalBytes < minBinaryUploadProgressSizeBytes {
		return
	}
	uploadedBytes := min(max(progress.UploadedBytes, 0), progress.TotalBytes)
	percent := float64(uploadedBytes) * 100 / float64(progress.TotalBytes)
	_, _ = fmt.Fprintf(
		p.out,
		"\rЗагрузка файла: %s из %s, %.1f%%",
		formatMiB(uploadedBytes),
		formatMiB(progress.TotalBytes),
		percent,
	)
	p.printed = true
}

// finish переводит вывод на новую строку после прогресса, чтобы итоговое сообщение не прилипало к нему.
func (p *binaryUploadProgressPrinter) finish() {
	if p.printed {
		_, _ = fmt.Fprintln(p.out)
		p.printed = false
	}
}

// formatMiB форматирует размер в МиБ для строки прогресса загрузки.
func formatMiB(size int64) string {
	mebibytes := float64(size) / 1024 / 1024
	if mebibytes == math.Trunc(mebibytes) {
		return fmt.Sprintf("%.0f МиБ", mebibytes)
	}
	return fmt.Sprintf("%.1f МиБ", mebibytes)
}
