package repository

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsUniqueViolation(t *testing.T) {
	if !IsUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("expected PostgreSQL unique violation to be recognized")
	}
	if IsUniqueViolation(&pgconn.PgError{Code: "23503"}) {
		t.Fatal("foreign-key violation must not be treated as unique")
	}
	if IsUniqueViolation(errors.New("duplicate key")) {
		t.Fatal("untyped driver text must not be treated as unique")
	}
}
