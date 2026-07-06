package main

import "errors"

var (
	errBackToRecordsList = errors.New("вернуться к списку записей")
	errExitApplication   = errors.New("завершить приложение")
	errActionCanceled    = errors.New("действие отменено")
	errReadonlyMode      = errors.New("режим чтения: действие временно недоступно")
)
