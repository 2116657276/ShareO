package config

import "testing"

func TestDatabaseConfigDSNUsesPostgresURLAndEscapesCredentials(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "shareo_app",
		Password: "p@ss word",
		DBName:   "shareo",
		SSLMode:  "disable",
		Timezone: "Asia/Shanghai",
	}
	got := cfg.DSN()
	want := "postgres://shareo_app:p%40ss%20word@127.0.0.1:5432/shareo?TimeZone=Asia/Shanghai&sslmode=disable"
	if got != want {
		t.Fatalf("DSN() = %q, want %q", got, want)
	}
}

func TestApplyEnvOverridesSupportsPostgresEndpointAndRole(t *testing.T) {
	t.Setenv("SHAREO_DB_USER", "shareo_app_override")
	t.Setenv("SHAREO_PG_HOST", "localhost")
	t.Setenv("SHAREO_PG_PORT", "55432")
	t.Setenv("SHAREO_PG_DATABASE", "shareo_test")
	t.Setenv("SHAREO_DB_PASSWORD", "secret")

	cfg := &Config{}
	applyEnvOverrides(cfg)

	if cfg.Database.User != "shareo_app_override" || cfg.Database.Host != "localhost" ||
		cfg.Database.Port != 55432 || cfg.Database.DBName != "shareo_test" ||
		cfg.Database.Password != "secret" {
		t.Fatalf("unexpected PostgreSQL overrides: %+v", cfg.Database)
	}
}
