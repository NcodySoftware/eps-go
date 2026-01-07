package jsonrpc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"

	"ncody.com/ncgo.git/log"
	"ncody.com/ncgo.git/stackerr"
)

type NotificationHandlerFunc = func(n *Notification)

type clientFlags byte

const (
	TLS clientFlags = 1 << iota
	TLSNoVerify
)

type ClientOpts struct {
	NHandler NotificationHandlerFunc
	Flags    clientFlags
}

type Client struct {
	log    *log.Logger
	cancel func()
	done   chan struct{}
	reqC   chan Request
	resC   chan Response
}

func NewClient(
	ctx context.Context,
	log *log.Logger,
	addr string,
	opts ClientOpts,
) (*Client, error) {
	if opts.NHandler == nil {
		opts.NHandler = func(*Notification) {}
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	if opts.Flags&(TLS|TLSNoVerify) == (TLS | TLSNoVerify) {
		conn = tls.Client(conn, &tls.Config{
			InsecureSkipVerify: true,
		})
	} else if opts.Flags&TLS != 0 {
		conn = tls.Client(conn, &tls.Config{})
	}
	ctx, cancel := context.WithCancel(ctx)
	c := &Client{
		log:    log,
		cancel: cancel,
		done:   make(chan struct{}),
		reqC:   make(chan Request, 1),
		resC:   make(chan Response, 1),
	}
	go func() {
		c.run(ctx, conn, opts.NHandler)
		close(c.done)
	}()
	return c, nil
}

func (c *Client) Close(ctx context.Context) error {
	c.cancel()
	select {
	case <-ctx.Done():
		return fmt.Errorf("jsonrpc client stop timeout reached")
	case <-c.done:
		return nil
	}
}

func (c *Client) Send(req Request) (Response, error) {
	select {
	case c.reqC <- req:
	case <-c.done:
		return Response{},
			fmt.Errorf("jsonrpc client: stopped when sending")
	}
	select {
	case res := <-c.resC:
		return res, nil
	case <-c.done:
		return Response{},
			fmt.Errorf("jsonrpc client: stopped when receiving")
	}
}

func (c *Client) run(
	ctx context.Context, conn net.Conn, nh NotificationHandlerFunc,
) {
	var wg sync.WaitGroup
	defer wg.Wait()
	var once sync.Once
	onceClose := func() {
		once.Do(func() {
			err := conn.Close()
			if err != nil {
				c.log.Err(stackerr.Wrap(err))
			}
		})
	}
	defer onceClose()
	errC := make(chan error, 1)
	notC := make(chan Notification, 1)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	wg.Go(func() {
		c.write(ctx, errC, c.reqC, conn)
	})
	wg.Go(func() {
		c.read(ctx, errC, c.resC, notC, conn)
	})
	wg.Go(func() {
		c.notify(ctx, notC, nh)
	})
	select {
	case err := <-errC:
		c.log.Err(stackerr.Wrap(err))
	case <-ctx.Done():
	}
	cancel()
	onceClose()
}

func (c *Client) write(
	ctx context.Context,
	errC chan<- error,
	reqC <-chan Request,
	conn io.Writer,
) {
	enc := json.NewEncoder(conn)
	for ctx.Err() == nil {
		select {
		case req := <-reqC:
			err := enc.Encode(req)
			if err != nil {
				trySend(errC, stackerr.Wrap(err))
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) read(
	ctx context.Context,
	errC chan<- error,
	resC chan<- Response,
	notC chan<- Notification,
	conn io.Reader,
) {
	dec := json.NewDecoder(conn)
	var raw json.RawMessage
	for ctx.Err() == nil {
		raw = raw[:0]
		err := dec.Decode(&raw)
		if err != nil {
			trySend(errC, stackerr.Wrap(err))
			return
		}
		var res Response
		var not Notification
		err = json.Unmarshal(raw, &res)
		if err != nil {
			trySend(errC, stackerr.Wrap(err))
			return
		}
		if res.Id != nil {
			resC <- res
			continue
		}
		err = json.Unmarshal(raw, &not)
		if err != nil {
			trySend(errC, stackerr.Wrap(err))
			return
		}
		notC <- not
	}
}

func (c *Client) notify(
	ctx context.Context,
	notC <-chan Notification,
	nh NotificationHandlerFunc,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case not := <-notC:
			nh(&not)
		}
	}
}
