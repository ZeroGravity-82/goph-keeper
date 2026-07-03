package main

import "errors"

var (
	errBackToRecordsList = errors.New("вернуться к списку записей")
	errExitApplication   = errors.New("завершить приложение")
	errActionCanceled    = errors.New("действие отменено")
)
