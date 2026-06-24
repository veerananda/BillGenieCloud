package config

import (
	"os"
	"testing"
)

func TestBuildDatabaseDSN_FromDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://user:pass@host.example.com:25060/mydb")
	t.Setenv("DATABASE_HOST", "ignored")

	cfg := &Config{Environment: "production"}
	dsn := buildDatabaseDSN(cfg)

	if dsn != "postgresql://user:pass@host.example.com:25060/mydb?sslmode=require" {
		t.Fatalf("unexpected DSN: %s", dsn)
	}
}

func TestBuildDatabaseDSN_FromDatabaseURLWithSSL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://user:pass@host.example.com:25060/mydb?sslmode=require")

	cfg := &Config{Environment: "production"}
	dsn := buildDatabaseDSN(cfg)

	if dsn != "postgresql://user:pass@host.example.com:25060/mydb?sslmode=require" {
		t.Fatalf("unexpected DSN: %s", dsn)
	}
}

func TestBuildDatabaseDSN_FromIndividualFields(t *testing.T) {
	os.Unsetenv("DATABASE_URL")

	cfg := &Config{
		Environment: "development",
		DBHost:      "localhost",
		DBUser:      "user",
		DBPassword:  "password",
		DBName:      "restaurant_db",
		DBPort:      "5432",
	}
	dsn := buildDatabaseDSN(cfg)

	want := "host=localhost user=user password=password dbname=restaurant_db port=5432 sslmode=disable"
	if dsn != want {
		t.Fatalf("got %q want %q", dsn, want)
	}
}
