package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// IsUniqueViolation reports the PostgreSQL constraint error without exposing
// the driver message to an HTTP client. Repositories keep the driver detail
// for logs; services map it to a stable domain error.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
