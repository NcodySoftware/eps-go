package jsonrpc_test

import (
	"testing"

	"github.com/ncodysoftware/eps-go/jsonrpc"
	"github.com/ncodysoftware/eps-go/testutil"
	"ncody.com/ncgo.git/assert"
)

type testHandler struct {
	events   []int
	connId   chan uint32
	connDone chan struct{}
}

func (t *testHandler) OnConnect(connId uint32) {
	t.events = append(t.events, 0)
	t.connId <- connId
}

func (t *testHandler) OnRequest(ctx *jsonrpc.Ctx) error {
	ctx.Response.Result = ctx.Request.Params
	t.events = append(t.events, 1)
	return nil
}

func (t *testHandler) OnDisconnect(connId uint32) {
	t.events = append(t.events, 2)
	close(t.connDone)
}

func nHandler(
	notC chan<- jsonrpc.Notification,
) jsonrpc.NotificationHandlerFunc {
	return func(n *jsonrpc.Notification) {
		notC <- *n
	}
}

func TestIntegrationClientServer(t *testing.T) {
	tc, cls := testutil.GetTCtx(t)
	defer cls()
	hdl := testHandler{
		connId:   make(chan uint32, 1),
		connDone: make(chan struct{}),
	}
	srv, err := jsonrpc.NewServer(
		tc.C, tc.L, "127.0.0.1:8080", &hdl,
	)
	assert.Must(t, err)
	defer func() {
		err := srv.Close(tc.C)
		assert.Must(t, err)
	}()
	notC := make(chan jsonrpc.Notification, 1)
	cli, err := jsonrpc.NewClient(
		tc.C, tc.L, "127.0.0.1:8080", jsonrpc.ClientOpts{
			NHandler: nHandler(notC),
		},
	)
	assert.Must(t, err)
	defer func() {
		err := cli.Close(tc.C)
		assert.Must(t, err)
	}()
	connId := <-hdl.connId
	res, err := cli.Send(jsonrpc.Request{
		Id:     []byte(`0`),
		Method: []byte(`"echo"`),
		Params: []byte(`["hello"]`),
	})
	assert.Must(t, err)
	assert.MustEqual(t, res.Result, []byte(`["hello"]`))
	notS := jsonrpc.Notification{
		Params: []byte(`["notification sent"]`),
	}
	err = srv.Notify(connId, &notS)
	assert.Must(t, err)
	not := <-notC
	assert.MustEqual(t, not.Params, []byte(`["notification sent"]`))
	err = cli.Close(tc.C)
	assert.Must(t, err)
	<-hdl.connDone
	assert.MustEqual(t, hdl.events, []int{0, 1, 2})
}
