package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

// ClientConfig описывает конфигурацию CLI-клиента.
//
// GRPCServerAddr - адрес gRPC-сервера в формате host:port.
//
// CACertPath - путь к CA-сертификату для проверки TLS-сертификата сервера.
type ClientConfig struct {
	GRPCServerAddr string `koanf:"grpc_address"`
	CACertPath     string `koanf:"ca_cert"`
}

// NewClientFlagSet создает набор флагов CLI-клиента с общими конфигурационными параметрами.
func NewClientFlagSet(name string) *pflag.FlagSet {
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.StringP("config", "c", "", "path to config file")
	flags.String("grpc-address", "", "gRPC server address")
	flags.String("ca-cert", "", "CA certificate path for gRPC client")
	return flags
}

// LoadClient читает конфигурацию CLI-клиента из дефолтных значений, конфигурационного файла, переменных окружения и
// флагов командной строки.
func LoadClient() (ClientConfig, error) {
	return loadClient(os.Args[1:])
}

func loadClient(args []string) (ClientConfig, error) {
	flags := NewClientFlagSet("client")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return ClientConfig{}, ErrHelp
		}
		return ClientConfig{}, fmt.Errorf("failed to parse flags: %w", err)
	}
	if flags.NArg() > 0 {
		return ClientConfig{}, fmt.Errorf("unexpected positional argument %q", flags.Arg(0))
	}
	return loadClientFromFlags(flags)
}

func loadClientFromFlags(flags *pflag.FlagSet) (ClientConfig, error) {
	configPath, err := flags.GetString("config")
	if err != nil {
		return ClientConfig{}, fmt.Errorf("failed to read config flag: %w", err)
	}

	return loadConfig[ClientConfig](loadOptions[ClientConfig]{
		Flags:      flags,
		ConfigPath: configPath,
		Validate:   validateClientConfig,
	})
}

func validateClientConfig(cfg ClientConfig) error {
	if cfg.GRPCServerAddr == "" {
		return errors.New("gRPC server address is required")
	}
	if err := validateServerAddr(cfg.GRPCServerAddr); err != nil {
		return err
	}
	if cfg.CACertPath == "" {
		return errors.New("CA certificate path is required")
	}
	return nil
}
