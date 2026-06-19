package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

var (
	// ErrHelp возвращается, когда пользователь запросил справку по флагам командной строки.
	ErrHelp = pflag.ErrHelp
)

const (
	envPrefix = "GOPHKEEPER_"
	keyDelim  = "."

	defaultLoggingLevel     = "info"
	defaultLoggingFormat    = "json"
	defaultLoggingAddSource = false
)

// Logging описывает настройки логирования сервиса.
type Logging struct {
	Format    string `koanf:"format"`
	Level     string `koanf:"level"`
	AddSource bool   `koanf:"add_source"`
}

// Config содержит параметры конфигурации сервиса.
type Config struct {
	DatabaseURI string  `koanf:"database_uri"`
	JWTSecret   string  `koanf:"jwt_secret"`
	Logging     Logging `koanf:"logging"`
}

// Load читает конфигурацию с учетом приоритета "дефолтное значение < значение из конфигурационного файла < флаг
// командной строки < переменная окружения".
func Load() (Config, error) {
	flags, configPath, err := parseFlags(os.Args[1:])
	if err != nil {
		return Config{}, err
	}

	k := koanf.New(keyDelim)
	if err = loadDefaults(k); err != nil {
		return Config{}, err
	}
	if configPath != "" {
		if err = k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
			return Config{}, fmt.Errorf("failed to load config file %q: %w", configPath, err)
		}
	}
	if err = k.Load(posflag.ProviderWithFlag(flags, keyDelim, k, mapFlag(flags)), nil); err != nil {
		return Config{}, fmt.Errorf("failed to load CLI flags: %w", err)
	}
	if err = k.Load(env.Provider(envPrefix, keyDelim, mapEnvKey), nil); err != nil {
		return Config{}, fmt.Errorf("failed to load environment variables: %w", err)
	}

	var cfg Config
	if err = k.Unmarshal("", &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	if err = validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func parseFlags(args []string) (*pflag.FlagSet, string, error) {
	flags := pflag.NewFlagSet("gophkeeper-server", pflag.ContinueOnError)
	flags.StringP("config", "c", "", "path to config file")
	flags.String("database-uri", "", "database connection URI")
	flags.String("jwt-secret", "", "JWT signing secret")
	flags.String("logging.format", "", "log format: text or json")
	flags.String("logging.level", "", "log level: debug, info, warn or error")
	flags.Bool("logging.add-source", false, "add source location to logs")

	if err := flags.Parse(args); err != nil {
		return nil, "", fmt.Errorf("failed to parse CLI flags: %w", err)
	}
	configPath, err := flags.GetString("config")
	if err != nil {
		return nil, "", fmt.Errorf("failed to read config flag: %w", err)
	}
	return flags, configPath, nil
}

func loadDefaults(k *koanf.Koanf) error {
	defaults := map[string]any{
		configKey("logging", "format"):     defaultLoggingFormat,
		configKey("logging", "level"):      defaultLoggingLevel,
		configKey("logging", "add_source"): defaultLoggingAddSource,
	}
	if err := k.Load(confmap.Provider(defaults, keyDelim), nil); err != nil {
		return fmt.Errorf("failed to load default config: %w", err)
	}
	return nil
}

func configKey(parts ...string) string {
	return strings.Join(parts, keyDelim)
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
	return key
}

func validate(cfg Config) error {
	if cfg.DatabaseURI == "" {
		return errors.New("database URI is required")
	}
	if cfg.JWTSecret == "" {
		return errors.New("JWT secret is required")
	}
	return nil
}
