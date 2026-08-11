package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	MinIO    MinIOConfig    `mapstructure:"minio"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Upload   UploadConfig   `mapstructure:"upload"`
}

type ServerConfig struct {
	Port           int      `mapstructure:"port"`
	Mode           string   `mapstructure:"mode"`
	TrustedOrigins []string `mapstructure:"trusted_origins"`
}

type DatabaseConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	SSLMode         string `mapstructure:"sslmode"`
	Timezone        string `mapstructure:"timezone"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

func (d DatabaseConfig) DSN() string {
	sslMode := d.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	timezone := d.Timezone
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	// gorm.io/driver/postgres inspects the raw TimeZone query value before
	// pgx parses the URL. Keep the IANA slash unescaped so its timezone
	// registration receives "Asia/Shanghai", not "Asia%2FShanghai".
	encodedTimezone := strings.ReplaceAll(url.QueryEscape(timezone), "%2F", "/")
	query := "TimeZone=" + encodedTimezone + "&sslmode=" + url.QueryEscape(sslMode)
	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(d.User, d.Password),
		Host:     fmt.Sprintf("%s:%d", d.Host, d.Port),
		Path:     "/" + d.DBName,
		RawQuery: query,
	}
	return dsn.String()
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type MinIOConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	UseSSL    bool   `mapstructure:"use_ssl"`
}

type JWTConfig struct {
	Secret           string `mapstructure:"secret"`
	ExpireHours      int    `mapstructure:"expire_hours"`
	LoginCacheTTLMin int    `mapstructure:"login_cache_ttl_min"`
}

func (j JWTConfig) ExpireDuration() time.Duration {
	return time.Duration(j.ExpireHours) * time.Hour
}

type UploadConfig struct {
	MaxSize      int64    `mapstructure:"max_size"`
	AllowedTypes []string `mapstructure:"allowed_types"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Override sensitive values from environment variables
	applyEnvOverrides(cfg)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// applyEnvOverrides allows environment variables to override sensitive config values.
// Supported env vars:
//
//	SHAREO_DB_USER           — PostgreSQL application role
//	SHAREO_PG_HOST/PORT      — PostgreSQL endpoint
//	SHAREO_PG_DATABASE       — PostgreSQL database name
//	SHAREO_DB_PASSWORD       — database password
//	SHAREO_JWT_SECRET        — JWT signing secret
//	SHAREO_MINIO_ACCESS_KEY  — MinIO access key
//	SHAREO_MINIO_SECRET_KEY  — MinIO secret key
//	SHAREO_REDIS_PASSWORD    — Redis password
//	SHAREO_TRUSTED_ORIGINS   — comma-separated exact origins for browser/WS requests
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SHAREO_DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("SHAREO_PG_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("SHAREO_PG_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Database.Port = port
		}
	}
	if v := os.Getenv("SHAREO_PG_DATABASE"); v != "" {
		cfg.Database.DBName = v
	}
	if v := os.Getenv("SHAREO_DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("SHAREO_DB_SSLMODE"); v != "" {
		cfg.Database.SSLMode = v
	}
	if v := os.Getenv("SHAREO_DB_TIMEZONE"); v != "" {
		cfg.Database.Timezone = v
	}
	if v := os.Getenv("SHAREO_JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("SHAREO_MINIO_ACCESS_KEY"); v != "" {
		cfg.MinIO.AccessKey = v
	}
	if v := os.Getenv("SHAREO_MINIO_SECRET_KEY"); v != "" {
		cfg.MinIO.SecretKey = v
	}
	if v := os.Getenv("SHAREO_REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("SHAREO_TRUSTED_ORIGINS"); v != "" {
		cfg.Server.TrustedOrigins = strings.Split(v, ",")
	}
}

// Validate checks that required config fields are present and valid.
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port)
	}
	if c.Server.Mode == "" {
		c.Server.Mode = "debug"
	}
	for i, origin := range c.Server.TrustedOrigins {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin == "" || strings.Contains(origin, "*") {
			return fmt.Errorf("server.trusted_origins must contain exact non-empty origins")
		}
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
			parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("invalid trusted origin %q", origin)
		}
		c.Server.TrustedOrigins[i] = origin
	}

	if c.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if c.Database.Port <= 0 {
		return fmt.Errorf("database.port is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("database.user is required")
	}
	if c.Database.DBName == "" {
		return fmt.Errorf("database.dbname is required")
	}
	if c.Database.SSLMode == "" {
		c.Database.SSLMode = "disable"
	}
	if c.Database.Timezone == "" {
		c.Database.Timezone = "Asia/Shanghai"
	}
	if c.Database.ConnMaxLifetime < 60 {
		c.Database.ConnMaxLifetime = 300 // 默认 5 分钟，防止连接永不过期
	}

	if c.Redis.Host == "" {
		return fmt.Errorf("redis.host is required")
	}
	if c.Redis.Port <= 0 {
		return fmt.Errorf("redis.port is required")
	}

	if c.MinIO.Endpoint == "" {
		return fmt.Errorf("minio.endpoint is required")
	}
	if c.MinIO.Bucket == "" {
		return fmt.Errorf("minio.bucket is required")
	}

	if c.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret is required")
	}
	if c.JWT.ExpireHours <= 0 {
		c.JWT.ExpireHours = 72
	}
	if c.JWT.LoginCacheTTLMin <= 0 {
		c.JWT.LoginCacheTTLMin = 30
	}

	if c.Upload.MaxSize <= 0 {
		c.Upload.MaxSize = 50 * 1024 * 1024
	}

	return nil
}
