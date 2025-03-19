package interfaces

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type DatabaseService interface {
	InitDatabase(ctx context.Context)
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row
	Execute(ctx context.Context, query string, args ...interface{}) (int64, error)
	CloseDatabase(ctx context.Context)
	BeginTransaction(ctx context.Context) (pgx.Tx, error)
}
