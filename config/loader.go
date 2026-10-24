package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Option customizes Load.
type Option func(*loadOptions)

type loadOptions struct {
	requireFile    bool
	skipValidation bool
}

// WithRequireFile makes Load fail when the config file is missing.
func WithRequireFile() Option { return func(o *loadOptions) { o.requireFile = true } }

// WithoutValidation skips Validate, for CLI commands that do not boot the server (e.g. remote admin).
func WithoutValidation() Option { return func(o *loadOptions) { o.skipValidation = true } }

// Loader accumulates config sources (defaults, OTHRA_* env, bound flags) and decodes them.
type Loader struct {
	v *viper.Viper
}

// NewLoader returns a Loader seeded with defaults and OTHRA_* env bindings.
func NewLoader() *Loader {
	v := viper.NewWithOptions(viper.KeyDelimiter("."))
	setDefaults(v)
	bindEnv(v, "", reflect.TypeFor[Config]())
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
	return &Loader{v: v}
}

// BindFlag binds flag to the dotted config key; a changed flag beats env and file.
func (l *Loader) BindFlag(key string, flag *pflag.Flag) error {
	if flag == nil {
		return fmt.Errorf("config: bind %q: nil flag", key)
	}
	return l.v.BindPFlag(key, flag)
}

// Load reads the file at path (or the standard search when empty) and decodes into Config.
func (l *Loader) Load(path string, opts ...Option) (*Config, error) {
	o := loadOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	file := path
	if path == "" {
		file = findConfigFile()
	}
	if file != "" {
		l.v.SetConfigFile(file)
	}

	found := false
	if file != "" {
		switch err := l.v.ReadInConfig(); {
		case err == nil:
			found = true
		case isConfigNotFound(err):
		default:
			return nil, fmt.Errorf("config: read %q: %w", file, err)
		}
	}
	if o.requireFile && !found {
		return nil, fmt.Errorf("config: file required but not found (path=%q)", path)
	}

	var cfg Config
	decode := viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	))
	if err := l.v.Unmarshal(&cfg, decode); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}
	cfg.Redis.KeyPrefix = NormalizeRedisKeyPrefix(cfg.Redis.KeyPrefix)

	if o.skipValidation {
		return &cfg, nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Load resolves defaults <- file <- OTHRA_* env <- bound flags. An empty path searches standard locations.
func Load(path string, opts ...Option) (*Config, error) {
	return NewLoader().Load(path, opts...)
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("mode", string(ModeSelfhosted))
	v.SetDefault("env", string(EnvProduction))
	v.SetDefault("http.addr", ":5150")
	v.SetDefault("http.cookieSecure", true)
	v.SetDefault("scheduler.enabled", true)
	v.SetDefault("scheduler.shutdownGrace", "30s")
	v.SetDefault("queue.enabled", false)
	v.SetDefault("queue.driver", string(QueueDriverNATS))
	v.SetDefault("api.openapi.enabled", true)
	v.SetDefault("api.openapi.requireBasicAuth", true)
	v.SetDefault("mcp.enabled", false)
	v.SetDefault("auth.mode", string(AuthModeLocal))
	v.SetDefault("auth.sessionTTL", "24h")
	v.SetDefault("cli.sessionFile", "~/.othra/session.json")
	v.SetDefault("mail.driver", string(MailDriverConsole))
	v.SetDefault("mail.smtp.port", 587)
	v.SetDefault("mail.smtp.tls", string(MailTLSStartTLS))
	v.SetDefault("mail.smtp.timeout", "10s")
	v.SetDefault("rateLimit.driver", string(RateLimitDriverMemory))
	v.SetDefault("rateLimit.maxKeys", 100_000)
	v.SetDefault("rateLimit.auth.enabled", true)
	v.SetDefault("http.trustedProxyHops", 0)
	v.SetDefault("redis.keyPrefix", "othra:")
	v.SetDefault("redis.maxConnections", 10)
	v.SetDefault("redis.minIdleConnections", 0)
	v.SetDefault("redis.maxIdleTime", "5m")
	v.SetDefault("redis.connectTimeout", "5s")
	v.SetDefault("redis.readTimeout", "3s")
	v.SetDefault("redis.writeTimeout", "3s")
	v.SetDefault("nats.name", "othra")
	v.SetDefault("nats.connectTimeout", "10s")
	v.SetDefault("storage.app", "othra")
	v.SetDefault("storage.imageTransform", string(StorageImageTransformNone))
	v.SetDefault("storage.upload.urlTTL", "10m")
	v.SetDefault("storage.upload.pendingTTL", "1h")
	v.SetDefault("storage.private.signer", string(StoragePrivateSignerPresign))
	v.SetDefault("storage.private.urlTTL", "5m")
	v.SetDefault("s3.usePathStyle", true)
	v.SetDefault("s3.timeout", "10s")
	v.SetDefault("keys.rsaBits", 2048)
	v.SetDefault("keys.rotation.prePublish", "10m")
	v.SetDefault("keys.rotation.overlap", "168h")
	v.SetDefault("resourceServers.verification.interval", "1h")
	v.SetDefault("resourceServers.verification.staleAfter", "24h")
	v.SetDefault("resourceServers.verification.timeout", "5s")
	v.SetDefault("resourceServers.verification.allowPrivateHosts", false)
	v.SetDefault("endUsers.magicLink.ttl", "15m")
	v.SetDefault("endUsers.magicLink.resendAfter", "60s")
	v.SetDefault("endUsers.session.idle", "720h")
	v.SetDefault("endUsers.session.absolute", "2160h")
	v.SetDefault("endUsers.session.maxAccounts", 20)
	v.SetDefault("endUsers.csrf.maxAge", "1h")
	v.SetDefault("oauth.codeTTL", "60s")
	v.SetDefault("oauth.accessTokenTTL", "15m")
	v.SetDefault("oauth.idTokenTTL", "5m")
	v.SetDefault("oauth.refresh.idle", "720h")
	v.SetDefault("oauth.refresh.absolute", "2160h")
	v.SetDefault("oauth.cimd.fetchTimeout", "5s")
	v.SetDefault("oauth.cimd.maxBytes", 5120)
	v.SetDefault("oauth.cimd.cacheMin", "5m")
	v.SetDefault("oauth.cimd.cacheMax", "24h")
	v.SetDefault("oauth.cimd.allowPrivateHosts", false)
	v.SetDefault("db.driver", string(DBDriverPostgres))
	v.SetDefault("db.autoMigrate", true)
	v.SetDefault("db.rls.allowBypass", false)
	v.SetDefault("postgres.schema", "public")
	v.SetDefault("postgres.health.interval", "30s")
	v.SetDefault("postgres.health.timeout", "2s")
	v.SetDefault("postgres.connectTimeout", "5s")
	v.SetDefault("postgres.connectBackoff", "250ms")
	v.SetDefault("telemetry.metrics.exportInterval", "30s")
	v.SetDefault("telemetry.metrics.prometheus.addr", ":9090")
	v.SetDefault("telemetry.metrics.prometheus.path", "/metrics")
}

func findConfigFile() string {
	for _, p := range configCandidates() {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

// NOTE: priority order is the user config dir, then /etc, then the working directory.
func configCandidates() []string {
	var out []string
	if dir := userConfigDir(); dir != "" {
		out = append(out, filepath.Join(dir, "config.yaml"), filepath.Join(dir, "config.yml"))
	}
	if runtime.GOOS != "windows" {
		out = append(out, "/etc/othra/config.yaml", "/etc/othra/config.yml")
	}
	if wd, err := os.Getwd(); err == nil {
		out = append(out, filepath.Join(wd, "config.yaml"), filepath.Join(wd, "config.yml"))
	}
	return out
}

func userConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "othra")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "othra")
	}
	return ""
}

func isConfigNotFound(err error) bool {
	if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); ok {
		return true
	}
	return os.IsNotExist(err)
}
