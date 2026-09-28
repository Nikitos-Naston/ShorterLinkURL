package config

import (
	"errors"
	"flag"
	"strings"
)

const Usage = "Usage: shortener [-storage memory|postgres]\nPostgreSQL connection string: DATABASE_URL\n"

type Config struct {
	Storage     string
	DatabaseURL string
	BaseURL     string
}

func Load(args []string, getenv func(string) string) (Config, error) {
	var cfg Config
	flags := flag.NewFlagSet("shortener", flag.ContinueOnError)
	flags.StringVar(&cfg.Storage, "storage", "memory", "storage: memory or postgres")
	if err := flags.Parse(args); err != nil {
		return cfg, err
	}
	if flags.NArg() != 0 {
		return cfg, errors.New("unexpected positional arguments")
	}
	cfg.DatabaseURL = getenv("DATABASE_URL")
	cfg.BaseURL = getenv("BASE_URL")
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8080"
	}
	if cfg.Storage != "memory" && cfg.Storage != "postgres" {
		return cfg, errors.New("storage must be memory or postgres")
	}
	if cfg.Storage == "postgres" && cfg.DatabaseURL == "" {
		return cfg, errors.New("DATABASE_URL is required for postgres")
	}
	return cfg, nil
}
