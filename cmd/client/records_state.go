package main

import (
	"context"
	"fmt"
	"io"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// recordsMenuState хранит состояние меню приватных записей в рамках текущего запуска CLI-клиента.
//
// items используется для отображения списка и выбора записи по номеру строки.
//
// cache хранит последние полученные данные по record_id, включая полную расшифрованную запись и ее версию для
// последующего обновления.
type recordsMenuState struct {
	items    []clientApp.RecordListItem
	cache    map[string]cachedRecord
	readonly bool
}

// cachedRecord объединяет краткую информацию из списка и полные данные конкретного типа записи.
//
// В один момент заполнено только одно из полей credential, text, card или binary - в зависимости от типа приватной
// записи.
type cachedRecord struct {
	item       clientApp.RecordListItem
	credential *clientApp.CredentialRecord
	text       *clientApp.TextRecord
	card       *clientApp.CardRecord
	binary     *clientApp.BinaryRecord
}

// newRecordsMenuState создает состояние меню приватных записей с пустым кешем.
func newRecordsMenuState() *recordsMenuState {
	return &recordsMenuState{cache: make(map[string]cachedRecord)}
}

// enterReadonly переводит меню записей в режим чтения и уведомляет пользователя один раз.
func (s *recordsMenuState) enterReadonly(out io.Writer) {
	if s.readonly {
		return
	}
	s.readonly = true
	fmt.Fprintln(out, "потеряна связь с сервером, включен режим чтения")
	fmt.Fprintln(out, "доступны только список и записи, уже загруженные за текущий запуск приложения")
}

// exitReadonly возвращает меню записей в обычный режим и уведомляет пользователя один раз.
func (s *recordsMenuState) exitReadonly(out io.Writer) {
	if !s.readonly {
		return
	}
	s.readonly = false
	fmt.Fprintln(out, "связь с сервером восстановлена, режим чтения выключен")
}

// ensureWritable проверяет, что текущее меню записей не находится в режиме чтения.
func (s *recordsMenuState) ensureWritable() error {
	if s.readonly {
		return errReadonlyMode
	}
	return nil
}

// refresh загружает список записей и синхронизирует краткие данные в кеше.
func (s *recordsMenuState) refresh(ctx context.Context, app *clientApp.App) error {
	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	items, err := app.ListRecords(callCtx)
	if err != nil {
		return err
	}
	s.items = items
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		seen[item.RecordID] = struct{}{}
		entry := s.cache[item.RecordID]
		entry.item = item
		s.cache[item.RecordID] = entry
	}
	for recordID := range s.cache {
		if _, ok := seen[recordID]; !ok {
			delete(s.cache, recordID)
		}
	}
	return nil
}

// storeCredential сохраняет запись с учетными данными и ее краткое представление в кеше.
func (s *recordsMenuState) storeCredential(record clientApp.CredentialRecord) {
	item := clientApp.RecordListItem{
		RecordID:    record.RecordID,
		Type:        "credential",
		Title:       record.Title,
		Description: record.Description,
	}
	s.storeItem(item)
	entry := s.cache[record.RecordID]
	entry.credential = &record
	s.cache[record.RecordID] = entry
}

// storeText сохраняет текстовую запись и ее краткое представление в кеше.
func (s *recordsMenuState) storeText(record clientApp.TextRecord) {
	item := clientApp.RecordListItem{
		RecordID:    record.RecordID,
		Type:        "text",
		Title:       record.Title,
		Description: record.Description,
	}
	s.storeItem(item)
	entry := s.cache[record.RecordID]
	entry.text = &record
	s.cache[record.RecordID] = entry
}

// storeCard сохраняет запись банковской карты и ее краткое представление в кеше.
func (s *recordsMenuState) storeCard(record clientApp.CardRecord) {
	item := clientApp.RecordListItem{
		RecordID:    record.RecordID,
		Type:        "card",
		Title:       record.Title,
		Description: record.Description,
	}
	s.storeItem(item)
	entry := s.cache[record.RecordID]
	entry.card = &record
	s.cache[record.RecordID] = entry
}

// storeBinary сохраняет бинарную запись и ее краткое представление в кеше.
func (s *recordsMenuState) storeBinary(record clientApp.BinaryRecord) {
	item := clientApp.RecordListItem{
		RecordID:    record.RecordID,
		Type:        "binary",
		Title:       record.Title,
		Description: record.Description,
	}
	s.storeItem(item)
	entry := s.cache[record.RecordID]
	entry.binary = &record
	s.cache[record.RecordID] = entry
}

// storeItem обновляет или добавляет краткое представление записи в текущий список.
func (s *recordsMenuState) storeItem(item clientApp.RecordListItem) {
	for i := range s.items {
		if s.items[i].RecordID == item.RecordID {
			s.items[i].Type = item.Type
			s.items[i].Title = item.Title
			s.items[i].Description = item.Description
			return
		}
	}
	s.items = append(s.items, item)
}

// cachedCredential возвращает запись с учетными данными из кеша или загружает ее с сервера.
func cachedCredential(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
) (clientApp.CredentialRecord, error) {
	if entry, ok := state.cache[item.RecordID]; ok && entry.credential != nil {
		return *entry.credential, nil
	}
	record, err := loadCredential(ctx, app, item.RecordID)
	if err != nil {
		return clientApp.CredentialRecord{}, err
	}
	state.storeCredential(record)
	return record, nil
}

// loadCredential загружает запись с учетными данными с тайм-аутом клиентского запроса.
func loadCredential(ctx context.Context, app *clientApp.App, recordID string) (clientApp.CredentialRecord, error) {
	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	return app.GetCredential(callCtx, recordID)
}

// cachedText возвращает текстовую запись из кеша или загружает ее с сервера.
func cachedText(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
) (clientApp.TextRecord, error) {
	if entry, ok := state.cache[item.RecordID]; ok && entry.text != nil {
		return *entry.text, nil
	}
	record, err := loadText(ctx, app, item.RecordID)
	if err != nil {
		return clientApp.TextRecord{}, err
	}
	state.storeText(record)
	return record, nil
}

// loadText загружает текстовую запись с тайм-аутом клиентского запроса.
func loadText(ctx context.Context, app *clientApp.App, recordID string) (clientApp.TextRecord, error) {
	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	return app.GetText(callCtx, recordID)
}

// cachedCard возвращает запись банковской карты из кеша или загружает ее с сервера.
func cachedCard(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
) (clientApp.CardRecord, error) {
	if entry, ok := state.cache[item.RecordID]; ok && entry.card != nil {
		return *entry.card, nil
	}
	record, err := loadCard(ctx, app, item.RecordID)
	if err != nil {
		return clientApp.CardRecord{}, err
	}
	state.storeCard(record)
	return record, nil
}

// loadCard загружает запись банковской карты с тайм-аутом клиентского запроса.
func loadCard(ctx context.Context, app *clientApp.App, recordID string) (clientApp.CardRecord, error) {
	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	return app.GetCard(callCtx, recordID)
}

// cachedBinary возвращает бинарную запись из кеша или загружает ее с сервера.
func cachedBinary(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
) (clientApp.BinaryRecord, error) {
	if entry, ok := state.cache[item.RecordID]; ok && entry.binary != nil {
		return *entry.binary, nil
	}
	record, err := loadBinary(ctx, app, item.RecordID)
	if err != nil {
		return clientApp.BinaryRecord{}, err
	}
	state.storeBinary(record)
	return record, nil
}

// loadBinary загружает метаданные бинарной записи с тайм-аутом клиентского запроса.
func loadBinary(ctx context.Context, app *clientApp.App, recordID string) (clientApp.BinaryRecord, error) {
	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	return app.GetBinary(callCtx, recordID)
}
