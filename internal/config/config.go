package config

import (
	"fmt"
	"os"
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
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	Charset         string `mapstructure:"charset"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		d.User, d.Password, d.Host, d.Port, d.DBName, d.Charset)
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
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
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
//	SHAREO_DB_PASSWORD       — database password
//	SHAREO_JWT_SECRET        — JWT signing secret
//	SHAREO_MINIO_ACCESS_KEY  — MinIO access key
//	SHAREO_MINIO_SECRET_KEY  — MinIO secret key
//	SHAREO_REDIS_PASSWORD    — Redis password
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SHAREO_DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
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
}

// Validate checks that required config fields are present and valid.
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port)
	}
	if c.Server.Mode == "" {
		c.Server.Mode = "debug"
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

	if c.Upload.MaxSize <= 0 {
		c.Upload.MaxSize = 50 * 1024 * 1024
	}

	return nil
}
