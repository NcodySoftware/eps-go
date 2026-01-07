package sql

import (
	"context"
	"errors"
	"fmt"

	"ncody.com/ncgo.git/stackerr"
)

type TxFunction = func(tx Database) error

func Execute(ctx context.Context, pool Database, fn TxFunction) (err error) {
	var fnPanicked = true
	tx, err := pool.Begin(ctx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		if !fnPanicked {
			panic(v)
		}
		err = tx.Rollback(ctx)
		if err != nil {
			err = stackerr.Wrap(errors.Join(err, fmt.Errorf("%v", v)))
			panic(err)
		}
		panic(v)
	}()
	err = fn(tx)
	fnPanicked = false
	if err != nil {
		err1 := tx.Rollback(ctx)
		if err1 != nil {
			return stackerr.Wrap(errors.Join(err1, err))
		}
		return err
	}
	err = tx.Commit(ctx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}
