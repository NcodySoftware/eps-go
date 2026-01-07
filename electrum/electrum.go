package electrum

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/ncodysoftware/eps-go/jsonrpc"
	"github.com/ncodysoftware/eps-go/walletmanager"
	"ncody.com/ncgo.git/log"
	"ncody.com/ncgo.git/stackerr"
)

var nullResult = []byte(`null`)
var (
	errBadParams = errors.New("bad params")
)

func ListenAndServe(
	ctx context.Context,
	addr string,
	log *log.Logger,
	wm *walletmanager.W,
	onStart func(),
) error {
	log.Infof("to listen on %s", addr)
	mux := newMux(ctx, log, wm)
	srv, err := jsonrpc.NewServer(ctx, log, addr, mux)
	if err != nil {
		return stackerr.Wrap(err)
	}
	onStart()
	srv.Wait(ctx)
	return srv.Close(ctx)
}

type mux struct {
	ctx      context.Context
	log      *log.Logger
	handlers map[string]func(ctx *jsonrpc.Ctx) error
	w        *walletmanager.W
	bufPool  sync.Pool
}

func newMux(
	ctx context.Context,
	log *log.Logger,
	w *walletmanager.W,
) *mux {
	m := &mux{
		ctx: ctx,
		log: log,
		w:   w,
	}
	m.bufPool.New = func() any {
		return []byte{}
	}
	m.handlers = m.defaultHandlers()
	return m
}

func (m *mux) OnConnect(connId uint32) {
	_ = connId
}

func (h *mux) OnRequest(ctx *jsonrpc.Ctx) error {
	var m string
	err := json.Unmarshal(ctx.Request.Method, &m)
	if err != nil {
		return stackerr.Wrap(err)
	}
	handler, ok := h.handlers[m]
	if !ok {
		return fmt.Errorf("unknown method: %s", m)
	}
	err = handler(ctx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *mux) OnDisconnect(connId uint32) {
	m.w.UnsubscribeAll(connId)
}

func (m *mux) bGet() []byte {
	b := m.bufPool.Get().([]byte)
	bClear(&b)
	return b
}

func (m *mux) bPut(b []byte) {
	m.bufPool.Put(b)
}

func bClear(b *[]byte) {
	*b = (*b)[:0]
}
