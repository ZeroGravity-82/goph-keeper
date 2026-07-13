package config

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

const (
	// envPrefix задает общий префикс переменных окружения приложения.
	envPrefix = "GOPHKEEPER_"

	// keyDelim задает разделитель вложенных ключей конфигурации для koanf.
	keyDelim = "."
)

var (
	// ErrHelp возвращается, когда пользователь запросил справку по флагам командной строки.
	ErrHelp = pflag.ErrHelp
)

type loadOptions[T any] struct {
	Flags      *pflag.FlagSet
	ConfigPath string
	Defaults   map[string]any
	Validate   func(T) error
}

// loadConfig собирает конфигурацию из дефолтных значений, конфигурационного YAML-файла, переменных окружения и флагов
// командной строки.
//
// Каждый следующий источник переопределяет значения предыдущего, после чего результат преобразуется в целевую
// структуру и проходит опциональную валидацию.
func loadConfig[T any](opts loadOptions[T]) (T, error) {
	var cfg T

	k := koanf.New(keyDelim)
	if err := loadDefaults(k, opts.Defaults); err != nil {
		return cfg, err
	}
	if opts.ConfigPath != "" {
		if err := k.Load(file.Provider(opts.ConfigPath), yaml.Parser()); err != nil {
			return cfg, fmt.Errorf("failed to load config file %q: %w", opts.ConfigPath, err)
		}
	}
	if err := k.Load(env.Provider(envPrefix, keyDelim, mapEnvKey), nil); err != nil {
		return cfg, fmt.Errorf("failed to load environment variables: %w", err)
	}
	if err := k.Load(posflag.ProviderWithFlag(opts.Flags, keyDelim, k, mapFlag(opts.Flags)), nil); err != nil {
		return cfg, fmt.Errorf("failed to load CLI flags: %w", err)
	}

	if err := k.Unmarshal("", &cfg); err != nil {
		return cfg, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	if opts.Validate != nil {
		if err := opts.Validate(cfg); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

func loadDefaults(k *koanf.Koanf, defaults map[string]any) error {
	if err := k.Load(confmap.Provider(defaults, keyDelim), nil); err != nil {
		return fmt.Errorf("failed to load default config: %w", err)
	}
	return nil
}

func mapFlag(flags *pflag.FlagSet) func(*pflag.Flag) (string, interface{}) {
	return func(flag *pflag.Flag) (string, interface{}) {
		key := strings.ReplaceAll(flag.Name, "-", "_")
		return key, posflag.FlagVal(flags, flag)
	}
}

func mapEnvKey(key string) string {
	key = strings.TrimPrefix(key, envPrefix)
	key = strings.ToLower(key)
	if strings.HasPrefix(key, "logging_") {
		return configKey("logging", strings.TrimPrefix(key, "logging_"))
	}
	if strings.HasPrefix(key, "file_storage_") {
		return configKey("file_storage", strings.TrimPrefix(key, "file_storage_"))
	}
	return key
}

func configKey(parts ...string) string {
	return strings.Join(parts, keyDelim)
}
