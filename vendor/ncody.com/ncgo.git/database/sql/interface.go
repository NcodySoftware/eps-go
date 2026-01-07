package sql

import "context"

type Database interface {
	// Query executes a Query and returns the rows
	// the Rows object must be closed to release the underlying
	// database connection back to the pool
	Query(ctx context.Context, sql string, args ...any) (Rows, error)

	// QueryRow executes the query and return the first row
	QueryRow(ctx context.Context, sql string, args ...any) Row

	// Exec will execute the sql command and return the amount of affected
	// rows
	Exec(ctx context.Context, sql string, args ...any) (int64, error)

	// Commit commits the changes of the transaction to the database
	Commit(ctx context.Context) error

	// Begin aquire one connection from the pool and
	// starts a new transaction
	//
	// Calling Begin on transaction will return error
	Begin(ctx context.Context) (Transaction, error)

	// Rollback rollbacks the changes of the transaction
	// Rollback(ctx context.Context) error

	// Close will release the resources of the pool
	Close(ctx context.Context) error
}

type Row interface {
	Scan(dest ...any) error
}

type Rows interface {
	Close() error
	Next() bool
	Scan(dest ...any) error
}

type Transaction interface {
	Database
	Rollback(ctx context.Context) error
}
