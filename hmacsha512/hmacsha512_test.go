package hmacsha512

import (
	"encoding/hex"
	"github.com/ncodysoftware/eps-go/assert"
	"testing"
)

func Test_HMACSHA512(t *testing.T) {
	key := "key"
	msg := "msg"
	exp := "1e4b55b925ccc28ed90d9d18fc2393fc" +
		"be164c0d84e67e173cc5aa486b7afc10" +
		"6633c66bdc309076f5f8d9fdbbb62456" +
		"f894f2c23377fbcc12f4ab2940eb6d70"
	hash := Sum512([]byte(key), []byte(msg), nil)
	assert.MustEqual(t, exp, hex.EncodeToString(hash[:]))
}
