package storage

import (
	"context"
	"testing"
)

func TestInvalidDSN(t *testing.T) {
	if _, err := NewPostgres(context.Background(), "postgres://%xx"); err == nil {
		t.Fatal("invalid DSN accepted")
	}
}
