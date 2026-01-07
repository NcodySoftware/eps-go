package stackerr

import (
	"fmt"
	"runtime"
)

func Wrap(err error) error {
	if err == nil {
		return nil
	}
	pc, _, line, ok := runtime.Caller(1)
	if !ok {
		return fmt.Errorf("error calling runtime.Caller\n\t%w", err)
	}
	fn := runtime.FuncForPC(pc)
	return fmt.Errorf(
		"at %s:%d 0x%x\n\t%w",
		fn.Name(),
		line,
		pc,
		err,
	)
}
