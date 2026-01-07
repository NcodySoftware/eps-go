package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"ncody.com/ncgo.git/log"
	"ncody.com/ncgo.git/stackerr"
)

type ServerHandler interface {
	OnConnect(connId uint32)
	OnRequest(ctx *Ctx) error
	OnDisconnect(connId uint32)
}

type Notifier interface {
	Notify(connId uint32, n *Notification) error
}

type Ctx struct {
	ConnId   uint32
	Request  Request
	Response Response
	Notifier Notifier
}

type connState struct {
	notChan chan<- Notification
}

type Server struct {
	cancel func()
	done   chan struct{}
	log    *log.Logger

	mu          sync.Mutex
	connections map[uint32]connState
}

func NewServer(
	ctx context.Context,
	log *log.Logger,
	addr string,
	h ServerHandler,
) (*Server, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	var once sync.Once
	cancel2 := func() {
		once.Do(func() {
			cancel()
			listener.Close()
		})
	}
	s := &Server{
		cancel:      cancel2,
		done:        make(chan struct{}),
		log:         log,
		connections: make(map[uint32]connState),
	}
	go func() {
		s.run(ctx, listener, h)
		close(s.done)
	}()
	return s, nil
}

func (s *Server) Close(ctx context.Context) error {
	s.cancel()
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) Wait(ctx context.Context) {
	select {
	case <-s.done:
	case <-ctx.Done():
	}
}

func (s *Server) Notify(connId uint32, n *Notification) error {
	s.mu.Lock()
	st, ok := s.connections[connId]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("no resource associated with connId")
	}
	if n.JsonRPC == nil {
		n.JsonRPC = []byte(`"2.0"`)
	}
	s.log.Debugf("%d <== %s", connId, n)
	ok = trySend(st.notChan, *n)
	if ok {
		return nil
	}
	return fmt.Errorf("sending notification would block")
}

type connAccepter interface {
	Accept() (net.Conn, error)
}

func (s *Server) run(
	ctx context.Context,
	listener connAccepter,
	h ServerHandler,
) {
	var wg sync.WaitGroup
	var connId uint32
	for ctx.Err() == nil {
		conn, err := listener.Accept()
		if err != nil && strings.Contains(
			err.Error(),
			"use of closed network connection",
		) {
			continue
		} else if err != nil {
			s.log.Err(stackerr.Wrap(err))
			continue
		}
		wg.Go(func() {
			err := s.connHandler(ctx, conn, connId, h)
			if err != nil {
				s.log.Err(stackerr.Wrap(err))
			}
		})
		connId++
	}
}

func (s *Server) connHandler(
	ctx context.Context, conn net.Conn, connId uint32, h ServerHandler,
) error {
	cOnce := sync.Once{}
	connClose := func() {
		cOnce.Do(func() {
			conn.Close()
		})
	}
	defer connClose()
	wg := sync.WaitGroup{}
	defer wg.Wait()
	errC := make(chan error, 1)
	reqC := make(chan Request, 1)
	resC := make(chan Response, 1)
	notC := make(chan Notification, 1)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.mu.Lock()
	s.connections[connId] = connState{notC}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.connections, connId)
		s.mu.Unlock()
	}()
	wg.Go(func() {
		s.read(ctx, errC, reqC, conn)
	})
	wg.Go(func() {
		s.write(ctx, errC, resC, notC, conn)
	})
	wg.Go(func() {
		s.rpcLoop(ctx, errC, reqC, resC, connId, h)
	})
	select {
	case err := <-errC:
		cancel()
		connClose()
		if err != nil && errors.Is(err, io.EOF) {
			return nil
		}
		return stackerr.Wrap(err)
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(
			context.Background(), time.Second*5,
		)
		defer cancel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			wg.Wait()
		}()
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			connClose()
		}
		<-done
		return nil
	}
}

func (s *Server) read(
	ctx context.Context,
	errC chan<- error,
	reqC chan<- Request,
	conn net.Conn,
) {
	dec := json.NewDecoder(conn)
	for ctx.Err() == nil {
		var r Request
		err := dec.Decode(&r)
		if err != nil {
			trySend(errC, stackerr.Wrap(err))
			return
		}
		reqC <- r
	}
}

func (s *Server) write(
	ctx context.Context,
	errC chan<- error,
	resC <-chan Response,
	notC <-chan Notification,
	conn net.Conn,
) {
	enc := json.NewEncoder(conn)
	for ctx.Err() == nil {
		select {
		case r := <-resC:
			err := enc.Encode(r)
			if err != nil {
				trySend(errC, stackerr.Wrap(err))
				return
			}
		case n := <-notC:
			err := enc.Encode(n)
			if err != nil {
				trySend(errC, stackerr.Wrap(err))
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Server) rpcLoop(
	ctx context.Context,
	errC chan<- error,
	reqC <-chan Request,
	resC chan<- Response,
	connId uint32,
	h ServerHandler,
) {
	var err error
	h.OnConnect(connId)
	s.log.Info("new connection ", connId)
loop:
	for ctx.Err() == nil {
		var c Ctx
		c.ConnId = connId
		select {
		case <-ctx.Done():
			break loop
		case c.Request = <-reqC:
			s.log.Debugf("%d ==> %s", c.ConnId, &c.Request)
			c.Notifier = s
			err = h.OnRequest(&c)
			if err != nil {
				break loop
			}
			c.Response.Id = c.Request.Id
			c.Response.JsonRPC = []byte(`"2.0"`)
			s.log.Debugf("%d <== %s", c.ConnId, &c.Response)
			resC <- c.Response
		}
	}
	h.OnDisconnect(connId)
	s.log.Info("disconnected ", connId)
	if err != nil {
		trySend(errC, stackerr.Wrap(err))
	}
}
