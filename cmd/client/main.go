package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
	"zerogravity-82/goph-keeper/internal/buildinfo"
	"zerogravity-82/goph-keeper/internal/config"
)

const defaultTimeout = 10 * time.Second

var (
	buildVersion string
	buildDate    string
)

func main() {
	cfg, err := config.LoadClient()
	if err != nil {
		if errors.Is(err, config.ErrHelp) {
			// Пользователь запросил справку по флагам командной строки; это штатное завершение.
			return
		}
		fmt.Fprintf(os.Stderr, "ошибка: %v\n", err)
		os.Exit(1)
	}

	if err = run(context.Background(), cfg, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "ошибка: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.ClientConfig, in io.Reader, out io.Writer) (err error) {
	app, err := newApp(cfg)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, app.Close())
	}()

	return runStartMenu(ctx, app, buildinfo.New(buildVersion, buildDate), in, out)
}

func newApp(clientCfg config.ClientConfig) (*clientApp.App, error) {
	return clientApp.New(clientCfg.GRPCServerAddr, clientCfg.CACertPath)
}
