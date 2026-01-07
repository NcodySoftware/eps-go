package sqlite

import (
	"context"
	sqlp "database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/stackerr"
)

type sqliteDB struct {
	inner *sqlp.DB
}

func New(dbpath string) (sql.Database, error) {
	connstr := fmt.Sprintf(
		"file:%s?_foreign_keys=on&_busy_timeout=5000", dbpath,
	)
	db, err := sqlp.Open("sqlite", connstr)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	return sqliteDB{
		inner: db,
	}, nil
}

func (s sqliteDB) Close(ctx context.Context) error {
	_ = ctx
	s.inner.Close()
	return nil
}

func (s sqliteDB) QueryRow(
	ctx context.Context, query string, args ...any,
) sql.Row {
	return sqliteRow{s.inner.QueryRowContext(ctx, query, args...)}
}

func (s sqliteDB) Query(
	ctx context.Context, query string, args ...any,
) (sql.Rows, error) {
	rows, err := s.inner.QueryContext(ctx, query, args...)
	if err != nil && errors.Is(err, sqlp.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	return rows, err
}

func (s sqliteDB) Exec(
	ctx context.Context, query string, args ...any,
) (int64, error) {
	r, err := s.inner.ExecContext(ctx, query, args...)
	if err != nil && errors.Is(err, sqlp.ErrNoRows) {
		return 0, sql.ErrNoRows
	}
	if err != nil {
		return 0, err
	}
	nr, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	return nr, nil
}

func (s sqliteDB) Commit(ctx context.Context) error {
	return nil
}

func (s sqliteDB) Begin(ctx context.Context) (sql.Transaction, error) {
	tx, err := s.inner.BeginTx(ctx, &sqlp.TxOptions{})
	if err != nil {
		return nil, err
	}
	return SqliteTx{Tx: tx}, nil
}

type SqliteTx struct {
	*sqlp.Tx
	isChild bool
}

func (s SqliteTx) Rollback(ctx context.Context) error {
	_ = ctx
	return s.Tx.Rollback()
}

func (s SqliteTx) Commit(ctx context.Context) error {
	return s.Tx.Commit()
}

func (s SqliteTx) QueryRow(
	ctx context.Context, query string, args ...any,
) sql.Row {
	return sqliteRow{s.Tx.QueryRowContext(ctx, query, args...)}
}

func (s SqliteTx) Query(
	ctx context.Context, query string, args ...any,
) (sql.Rows, error) {
	rows, err := s.Tx.QueryContext(ctx, query, args...)
	if err != nil && errors.Is(err, sqlp.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	return rows, err
}

func (s SqliteTx) Exec(
	ctx context.Context, query string, args ...any,
) (int64, error) {
	r, err := s.Tx.ExecContext(ctx, query, args...)
	if err != nil && errors.Is(err, sqlp.ErrNoRows) {
		return 0, sql.ErrNoRows
	}
	if err != nil {
		return 0, err
	}
	nrows, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	return nrows, nil
}

func (s SqliteTx) Begin(ctx context.Context) (sql.Transaction, error) {
	return s, sql.ErrNestedTransactionNotSupported
}

// Close is no-op to fulfill the Database interface
func (s SqliteTx) Close(ctx context.Context) error {
	return nil
}

type sqliteRow struct {
	*sqlp.Row
}

func (s sqliteRow) Scan(dest ...any) error {
	err := s.Row.Scan(dest...)
	if err != nil && errors.Is(err, sqlp.ErrNoRows) {
		return sql.ErrNoRows
	}
	return err
}
