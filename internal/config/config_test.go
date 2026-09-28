package config

import (
	"errors"
	"flag"
	"testing"
)

func TestLoad(t *testing.T) {
	empty := func(string) string { return "" }
	cfg, err := Load(nil, empty)
	if err != nil || cfg.Storage != "memory" {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
	const dsn = "postgres://user:password@localhost:5432/"
	cfg, err = Load([]string{"-storage=postgres"}, func(key string) string {
		if key == "DATABASE_URL" {
			return dsn
		}
		return ""
	})
	if err != nil || cfg.Storage != "postgres" || cfg.DatabaseURL != dsn {
		t.Fatalf("postgres configuration: %+v %v", cfg, err)
	}
	for _, args := range [][]string{{"-storage=unknown"}, {"-storage=postgres"}, {"-unknown"}, {"extra"}} {
		if _, err := Load(args, empty); err == nil {
			t.Errorf("accepted invalid arguments %v", args)
		}
	}
	if _, err := Load([]string{"-h"}, empty); !errors.Is(err, flag.ErrHelp) {
		t.Fatal(err)
	}
}
