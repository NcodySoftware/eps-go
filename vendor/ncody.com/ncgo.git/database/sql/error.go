package sql

import "errors"

var (
	ErrNoRows                        = errors.New("no rows")
	ErrNestedTransactionNotSupported = errors.New(
		"nested transaction not supported",
	)
)
