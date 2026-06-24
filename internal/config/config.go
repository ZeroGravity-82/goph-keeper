package config

import (
	"errors"
	"fmt"
	"net"
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

	defaultLoggingLevel      = "info"
	defaultLoggingFormat     = "json"
	defaultLoggingAddSource  = false
	defaultGRPCServerAddr    = "localhost:3201"
	defaultTLSCertPath       = "certs/server.crt"
	defaultTLSKeyPath        = "certs/server.key"
	defaultCACertPath        = "certs/ca.crt"
	defaultFileStorageUseSSL = false
)

// Logging описывает настройки логирования сервиса.
//
// Format - формат логов: text или json.
//
// Level - минимальный уровень логирования: debug, info, warn или error.
//
// AddSource - признак необходимости добавлять в лог место вызова.
type Logging struct {
	Format    string `koanf:"format"`
	Level     string `koanf:"level"`
	AddSource bool   `koanf:"add_source"`
}

// FileStorage описывает настройки S3-совместимого хранилища файлов.
//
// Endpoint - адрес хранилища файлов в формате host:port.
//
// AccessKey - идентификатор ключа доступа к хранилищу файлов.
//
// SecretKey - секретная часть ключа доступа к хранилищу файлов.
//
// Bucket - имя bucket для хранения зашифрованных файлов.
//
// UseSSL - флаг необходимости использовать HTTPS при подключении к хранилищу файлов.
type FileStorage struct {
	Endpoint  string `koanf:"endpoint"`
	AccessKey string `koanf:"access_key"`
	SecretKey string `koanf:"secret_key"`
	Bucket    string `koanf:"bucket"`
	UseSSL    bool   `koanf:"use_ssl"`
}

// Config описывает конфигурацию сервиса.
//
// GRPCServerAddr - адрес gRPC-сервера в формате host:port.
//
// TLSCertPath - путь к TLS-сертификату gRPC-сервера.
//
// TLSKeyPath - путь к приватному TLS-ключу gRPC-сервера.
//
// DatabaseURI - строка подключения к базе данных.
//
// JWTSecret - секрет для подписи JWT.
//
// FileStorage - настройки S3-совместимого хранилища файлов.
//
// Logging - настройки логирования сервиса.
type Config struct {
	GRPCServerAddr string      `koanf:"grpc_address"`
	TLSCertPath    string      `koanf:"tls_cert"`
	TLSKeyPath     string      `koanf:"tls_key"`
	DatabaseURI    string      `koanf:"database_uri"`
	JWTSecret      string      `koanf:"jwt_secret"`
	FileStorage    FileStorage `koanf:"file_storage"`
	Logging        Logging     `koanf:"logging"`
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
	var grpcServerAddrFlag string
	flags.Func("grpc-address", grpcServerAddrUsage(), grpcServerAddrFlagParser(&grpcServerAddrFlag))
	flags.String("tls-cert", "", "TLS certificate path for gRPC server")
	flags.String("tls-key", "", "TLS private key path for gRPC server")
	flags.String("database-uri", "", "database connection URI")
	flags.String("jwt-secret", "", "JWT signing secret")
	flags.String("file-storage.endpoint", "", "S3-compatible file storage address")
	flags.String("file-storage.access-key", "", "S3-compatible file storage access key")
	flags.String("file-storage.secret-key", "", "S3-compatible file storage secret key")
	flags.String("file-storage.bucket", "", "S3-compatible file storage bucket")
	flags.Bool("file-storage.use-ssl", false, "use SSL for S3-compatible file storage")
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

func grpcServerAddrUsage() string {
	return fmt.Sprintf(`gRPC server address (default "%s")`, defaultGRPCServerAddr)
}

func grpcServerAddrFlagParser(grpcServerAddr *string) func(string) error {
	return func(flagValue string) error {
		if flagValue == "" {
			*grpcServerAddr = flagValue
			return nil
		}
		if err := validateServerAddr(flagValue); err != nil {
			return err
		}
		*grpcServerAddr = flagValue
		return nil
	}
}

func validateServerAddr(v string) error {
	host, port, err := net.SplitHostPort(v)
	if host == "" || port == "" || err != nil {
		return errors.New("server address must be in the format host:port (without specifying a scheme)")
	}
	return nil
}

func loadDefaults(k *koanf.Koanf) error {
	defaults := map[string]any{
		configKey("logging", "format"):       defaultLoggingFormat,
		configKey("logging", "level"):        defaultLoggingLevel,
		configKey("logging", "add_source"):   defaultLoggingAddSource,
		configKey("grpc_address"):            defaultGRPCServerAddr,
		configKey("tls_cert"):                defaultTLSCertPath,
		configKey("tls_key"):                 defaultTLSKeyPath,
		configKey("file_storage", "use_ssl"): defaultFileStorageUseSSL,
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
	if strings.HasPrefix(key, "file_storage_") {
		return configKey("file_storage", strings.TrimPrefix(key, "file_storage_"))
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
	if cfg.FileStorage.Endpoint == "" {
		return errors.New("file storage endpoint is required")
	}
	if cfg.FileStorage.AccessKey == "" {
		return errors.New("file storage access key is required")
	}
	if cfg.FileStorage.SecretKey == "" {
		return errors.New("file storage secret key is required")
	}
	if cfg.FileStorage.Bucket == "" {
		return errors.New("file storage bucket is required")
	}
	return nil
}
