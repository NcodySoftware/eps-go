package assert

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func Must(t *testing.T, err error) {
	if err == nil {
		return
	}
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		t.Fatalf("must: runtime call failed")
	}
	lastSlashIdx := strings.LastIndex(file, "/")
	if lastSlashIdx >= 0 && (len(file)-1) > lastSlashIdx {
		file = file[lastSlashIdx+1:]
	}
	t.Fatalf("\r%s:%d: %s\n", file, line, err.Error())
}

func MustIdx(t *testing.T, idx int, err error) {
	if err == nil {
		return
	}
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		t.Fatalf("must: runtime call failed")
	}
	lastSlashIdx := strings.LastIndex(file, "/")
	if lastSlashIdx >= 0 && (len(file)-1) > lastSlashIdx {
		file = file[lastSlashIdx+1:]
	}
	t.Fatalf("\r%s:%d: idx %d: %s\n", file, line, idx, err.Error())
}

func MustEqual[T any](t *testing.T, expected, actual T) {
	if reflect.DeepEqual(expected, actual) {
		return
	}
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		t.Fatalf("mustEqual: runtime call failed")
	}
	lastSlashIdx := strings.LastIndex(file, "/")
	if lastSlashIdx >= 0 && (len(file)-1) > lastSlashIdx {
		file = file[lastSlashIdx+1:]
	}
	t.Fatalf(
		"\r%s:%d: \nexpected:\t%v\nactual:\t\t%v\n", file, line, expected, actual,
	)
}

func MustEqualIdx[T any](t *testing.T, idx int, expected, actual T) {
	if reflect.DeepEqual(expected, actual) {
		return
	}
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		t.Fatalf("mustEqualIdx: runtime call failed")
	}
	lastSlashIdx := strings.LastIndex(file, "/")
	if lastSlashIdx >= 0 && (len(file)-1) > lastSlashIdx {
		file = file[lastSlashIdx+1:]
	}
	t.Fatalf(
		"\r%s:%d: idx %d\nexpected:\t%v\nactual:\t\t%v\n",
		file,
		line,
		idx,
		expected,
		actual,
	)
}
