package bitcoin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"ncody.com/ncgo.git/log"
	"ncody.com/ncgo.git/stackerr"
)

var ErrBitcoinClientNotConnected = errors.New("bitcoin client: not connected")

type MessageHandlerFunc = func(b *Client, m message) error

type MessageHandler struct {
	f       MessageHandlerFunc
	command [12]byte
}

type Client struct {
	ctx         context.Context
	ctxCancel   context.CancelFunc
	nodeAddr    string
	log         *log.Logger
	magicBytes  [4]byte
	readChan    chan message
	writeChan   chan message
	stopChan    chan struct{}
	wg          sync.WaitGroup
	timeout     time.Duration
	mu          sync.Mutex
	initialized atomic.Bool
	handlers    map[[12]byte]MessageHandlerFunc
}

func NewClient(
	ctx context.Context,
	nodeAddress string,
	logger *log.Logger,
	net Network,
) (*Client, error) {
	handlers := make(map[[12]byte]MessageHandlerFunc)
	handlers[message_ping] = pingHandler
	ctx, cancel := context.WithCancel(ctx)
	c := &Client{
		ctx:        ctx,
		ctxCancel:  cancel,
		nodeAddr:   nodeAddress,
		log:        logger,
		magicBytes: magicBytes[net],
		timeout:    time.Second * 10,
		handlers:   handlers,
		readChan:   make(chan message, 1),
		writeChan:  make(chan message, 1),
		stopChan:   make(chan struct{}, 1),
	}
	err := c.start()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (b *Client) start() error {
	errChan := make(chan error, 1)
	connectTimeout := time.Second * 10
	reconnectWait := time.Second * 10
	b.wg.Go(func() {
		var firstHandshakeOk bool
		for b.ctx.Err() == nil {
			func() {
				b.initialized.Store(false)
			drainChannelsLoop:
				for {
				drainChannelsSelect:
					select {
					case <-b.readChan:
						break drainChannelsSelect
					case <-b.writeChan:
						break drainChannelsSelect
					case <-b.stopChan:
						break drainChannelsSelect
					default:
						break drainChannelsLoop
					}
				}
				conn, err := net.DialTimeout(
					"tcp", b.nodeAddr, connectTimeout,
				)
				if err != nil && !firstHandshakeOk {
					// At this point caller is waiting initialization
					trySend(errChan, stackerr.Wrap(err))
					return
				} else if err != nil {
					b.log.Warnf("bitcoin client: connect: %s", err)
					// Now, we keep retrying forever
					time.Sleep(reconnectWait)
					return
				}
				var wg sync.WaitGroup
				muxWait := make(chan struct{})
				wg.Go(func() { b.connHandler(conn, muxWait) })
				defer wg.Wait()
				// perform handshake
				ctx, cancel := context.WithTimeout(
					b.ctx, connectTimeout,
				)
				defer cancel()
				err = b.performHandshake(ctx)
				if err != nil && !firstHandshakeOk {
					trySend(b.stopChan, struct{}{})
					trySend(errChan, stackerr.Wrap(err))
					return
				} else if err != nil {
					trySend(b.stopChan, struct{}{})
					b.log.Info("bitcoin client: handshake failed")
					return
				}
				if firstHandshakeOk {
					b.log.Info("bitcoin client: reconnected")
				}
				// safe to client send requests
				b.initialized.Store(true)
				// Allow mux to start
				close(muxWait)
				// Unlock caller that is waiting
				trySend(errChan, nil)
				firstHandshakeOk = true
			}()
		}
	})
	err := <-errChan
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (b *Client) Close() {
	b.ctxCancel()
	b.wg.Wait()
}

func (b *Client) GetHeaders(
	ctx context.Context, headerHashes [][32]byte, headerEnd [32]byte,
) ([]Header, error) {
	if !b.initialized.Load() {
		return nil, ErrBitcoinClientNotConnected
	}
	headersChan := make(chan []Header)
	errChan := make(chan error)
	handleHeader := func(c *Client, m message) error {
		var p headers
		err := p.Deserialize(bytes.NewReader(m.Payload))
		if err != nil {
			errChan <- err
		}
		headersChan <- p.Headers
		return nil
	}
	b.setHandler(message_headers, handleHeader)
	defer b.unsetHandler(message_headers)
	err := b.send(
		ctx,
		makeGetHeadersMessage(
			b.magicBytes, headerHashes, headerEnd,
		),
	)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	select {
	case err := <-errChan:
		return nil, stackerr.Wrap(err)
	case headers := <-headersChan:
		return headers, nil
	case <-ctx.Done():
		return nil, stackerr.Wrap(ctx.Err())
	}
}

func (b *Client) GetBlock(ctx context.Context, bhash [32]byte) (Block, error) {
	if !b.initialized.Load() {
		return Block{}, ErrBitcoinClientNotConnected
	}
	blockChan := make(chan Block)
	errChan := make(chan error)
	blockHandler := func(c *Client, m message) error {
		var b Block
		err := b.Deserialize(bytes.NewReader(m.Payload))
		if err != nil {
			errChan <- err
		}
		blockChan <- b
		return nil
	}
	b.setHandler(message_block, blockHandler)
	defer b.unsetHandler(message_block)
	msg := makeInvMessage(
		b.magicBytes,
		message_getdata,
		inv{
			Count: 1,
			Inventory: []inventory{
				{
					Type: datatype_witness_block,
					Hash: bhash,
				},
			},
		},
	)
	err := b.send(ctx, msg)
	if err != nil {
		return Block{}, stackerr.Wrap(err)
	}
	select {
	case <-ctx.Done():
		return Block{}, stackerr.Wrap(ctx.Err())
	case err = <-errChan:
		return Block{}, stackerr.Wrap(err)
	case block := <-blockChan:
		return block, nil
	}
}

func (b *Client) BroadcastWTX(ctx context.Context, rawTx []byte) error {
	if !b.initialized.Load() {
		return ErrBitcoinClientNotConnected
	}
	wtxid := sha256.Sum256(rawTx)
	wtxid = sha256.Sum256(wtxid[:])
	txInvMsg := makeInvMessage(b.magicBytes, message_inv, inv{
		Count: 1,
		Inventory: []inventory{
			{
				Type: datatype_witness_tx,
				Hash: wtxid,
			},
		},
	})
	proceedC := make(chan struct{}, 1)
	errC := make(chan error, 1)
	hdl := func(b *Client, m message) error {
		var invM inv
		err := invM.Deserialize(bytes.NewReader(m.Payload))
		if err != nil {
			errC <- stackerr.Wrap(err)
			return nil
		}
		if len(invM.Inventory) != 1 {
			errC <- stackerr.Wrap(fmt.Errorf("unexpected inv count"))
			return nil
		}
		if invM.Inventory[0].Type != datatype_tx {
			errC <- stackerr.Wrap(fmt.Errorf("unexpected inv type"))
			return nil
		}
		if invM.Inventory[0].Hash != wtxid {
			errC <- stackerr.Wrap(fmt.Errorf("unexpected wtxid"))
			return nil
		}
		txM := message{
			Command:    message_tx,
			Payload:    rawTx,
			MagicBytes: b.magicBytes,
		}
		err = b.send(ctx, txM)
		if err != nil {
			errC <- stackerr.Wrap(err)
			return nil
		}
		proceedC <- struct{}{}
		return nil
	}
	b.setHandler(message_getdata, hdl)
	defer b.unsetHandler(message_getdata)
	err := b.send(ctx, txInvMsg)
	if err != nil {
		return stackerr.Wrap(err)
	}
	ctx1, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	select {
	case err := <-errC:
		return stackerr.Wrap(err)
	case <-proceedC:
	case <-ctx1.Done():
		return fmt.Errorf("broadcast timeout")
	}
	return nil
}

func (b *Client) send(ctx context.Context, m message) error {
	select {
	case <-b.ctx.Done():
		return stackerr.Wrap(b.ctx.Err())
	case <-ctx.Done():
		return stackerr.Wrap(ctx.Err())
	default:
	}
	select {
	case <-b.ctx.Done():
		return stackerr.Wrap(b.ctx.Err())
	case <-ctx.Done():
		return stackerr.Wrap(ctx.Err())
	case b.writeChan <- m:
		return nil
	}
}

func (b *Client) receive(ctx context.Context) (message, error) {
	var m message
	select {
	case <-b.ctx.Done():
		return m, stackerr.Wrap(b.ctx.Err())
	case <-ctx.Done():
		return m, stackerr.Wrap(ctx.Err())
	default:
	}
	select {
	case <-b.ctx.Done():
		return m, stackerr.Wrap(b.ctx.Err())
	case <-ctx.Done():
		return m, stackerr.Wrap(ctx.Err())
	case m = <-b.readChan:
		return m, nil
	}
}

func (b *Client) performHandshake(ctx context.Context) error {
	err := b.send(ctx, makeVersionMessage(b.magicBytes))
	if err != nil {
		return stackerr.Wrap(err)
	}
	msg, err := b.receive(ctx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if !bytes.HasPrefix(msg.Command[:], message_version[:]) {
		return stackerr.Wrap(fmt.Errorf("bad message"))
	}
	err = b.send(ctx, makeVerAckMessage(b.magicBytes))
	if err != nil {
		return stackerr.Wrap(err)
	}
	msg, err = b.receive(ctx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if !bytes.HasPrefix(msg.Command[:], message_verack[:]) {
		return stackerr.Wrap(fmt.Errorf("bad message"))
	}
	return nil
}

func (b *Client) setHandler(message [12]byte, hf MessageHandlerFunc) {
	b.mu.Lock()
	b.handlers[message] = hf
	b.mu.Unlock()
}

func (b *Client) unsetHandler(message [12]byte) {
	b.mu.Lock()
	delete(b.handlers, message)
	b.mu.Unlock()
}

func (b *Client) connHandler(conn readWriteCloser, muxUnlock chan struct{}) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(b.ctx)
	defer cancel()
	//
	wg.Go(func() {
		b.connRead(ctx, b.stopChan, b.readChan, conn)
	})
	wg.Go(func() {
		b.connWrite(ctx, b.stopChan, b.writeChan, conn)
	})
	select {
	case <-ctx.Done():
	case <-b.stopChan:
		cancel()
	case <-muxUnlock:
		wg.Go(func() {
			b.mux(ctx)
		})
	}
	//
	select {
	case <-ctx.Done():
	case <-b.stopChan:
		cancel()
	}
	conn.Close()
	wg.Wait()
}

func (b *Client) connRead(
	ctx context.Context,
	stopChan chan struct{},
	readChan chan message,
	conn readWriteCloser,
) {
	var wg sync.WaitGroup
	errChan := make(chan error, 1)
	wg.Go(func() {
		var (
			msg message
			err error
		)
		for {
			err = msg.Deserialize(conn)
			if err != nil {
				errChan <- err
				break
			}
			readChan <- msg
		}
	})
	select {
	case <-ctx.Done():
	case err := <-errChan:
		b.log.Warnf("%s", stackerr.Wrap(err))
	}
	trySend(stopChan, struct{}{})
	wg.Wait()
}

func (b *Client) connWrite(
	ctx context.Context,
	stopChan chan struct{},
	writeChan chan message,
	conn readWriteCloser,
) {
	var wg sync.WaitGroup
	errChan := make(chan error, 1)
	wg.Go(func() {
		var (
			buf []byte
			err error
			msg message
		)
		for {
			buf = buf[:0]
			select {
			case msg = <-writeChan:
			case <-ctx.Done():
				goto end
			}
			buf = msg.Serialize(buf)
			err = writeAll(conn, buf)
			if err != nil {
				errChan <- err
				break
			}
		}
	end:
	})
	select {
	case <-ctx.Done():
	case err := <-errChan:
		b.log.Errf("%s", stackerr.Wrap(err))
	}
	trySend(stopChan, struct{}{})
	wg.Wait()
}

func (b *Client) mux(ctx context.Context) {
	for {
	start:
		select {
		case <-ctx.Done():
			goto end
		case msg := <-b.readChan:
			b.mu.Lock()
			hf, ok := b.handlers[msg.Command]
			b.mu.Unlock()
			if !ok {
				if msg.Command != message_inv {
					b.log.Debugf(
						"bitcoin client: unhandled message: %s",
						msg.ToString(),
					)
				}
				goto start
			}
			err := hf(b, msg)
			if err != nil {
				b.log.Errf("err: %s", stackerr.Wrap(err))
			}
		}
	}
end:
}

func pingHandler(b *Client, m message) error {
	var p ping
	err := p.Deserialize(bytes.NewReader(m.Payload))
	if err != nil {
		return stackerr.Wrap(err)
	}
	m = makePongMessage(b.magicBytes, p.Nonce)
	err = sendWithCtxAndTimeout(b.ctx, b.timeout, b.writeChan, m)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func sendWithCtxAndTimeout[T any](
	ctx context.Context, t time.Duration, c chan T, m T,
) error {
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c <- m:
		return nil
	}
}

func receiveWithCtxAndTimeout[T any](
	ctx context.Context, t time.Duration, c chan T,
) (T, bool) {
	var m T
	timer := time.NewTimer(t)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return m, false
	case <-timer.C:
		return m, false
	case m = <-c:
		return m, true
	}
}

func trySend[T any](c chan T, m T) bool {
	select {
	case c <- m:
		return true
	default:
	}
	return false
}
