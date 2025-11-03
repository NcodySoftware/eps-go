package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ncodysoftware/eps-go/log"
	"github.com/ncodysoftware/eps-go/stackerr"
	"net"
	"sync"
)

var (
	ErrNotImplemented = errors.New("not implemented")
	ErrBadRequest     = errors.New("bad request")
)

type server struct {
	logger *log.Logger
	bcli   *btcCli
	wman   *walletManager
}

func newServer(logger *log.Logger, bcli *btcCli, wman *walletManager) *server {
	return &server{
		logger: logger,
		bcli:   bcli,
		wman:   wman,
	}
}

type clientCtx struct {
	scriptHashSubs [][32]byte
	notifChan      chan jsonRPCNotification
}

func (s *server) clientCleanup(cctx *clientCtx) {
	s.wman.unsubscribeAll(cctx)
}

func (s *server) handleConn(
	ctx context.Context,
	clientConn net.Conn,
) {
	var (
		wg sync.WaitGroup
	)
	ctx, cancel := context.WithCancel(ctx)
	stopChan := make(chan struct{}, 1)
	reqChanFromClient := make(chan jsonRPCRequest, 1)
	resChanToClient := make(chan jsonRPCResponse, 1)
	notifChanToClient := make(chan jsonRPCNotification, 1)
	clientCtx := clientCtx{nil, notifChanToClient}
	defer s.clientCleanup(&clientCtx)
	wg.Go(func() {
		s.clientRead(ctx, stopChan, clientConn, reqChanFromClient)
	})
	wg.Go(func() {
		s.clientWrite(
			ctx,
			stopChan,
			clientConn,
			resChanToClient,
			notifChanToClient,
		)
	})
	wg.Go(func() {
		s.mux(
			ctx,
			&clientCtx,
			stopChan,
			reqChanFromClient,
			resChanToClient,
		)
	})
	<-stopChan
	cancel()
	clientConn.Close()
	wg.Wait()
}

func (s *server) clientRead(
	ctx context.Context,
	stopChan chan struct{},
	clientConn net.Conn,
	reqChan chan jsonRPCRequest,
) {
	defer func() {
		select {
		case stopChan <- struct{}{}:
		default:
		}
	}()
	defer func() {
		s.logger.Debug("stopping clientRead")
	}()
	errChan := make(chan error, 1)
	go func() {
		dec := json.NewDecoder(clientConn)
		for {
			var req jsonRPCRequest
			err := dec.Decode(&req)
			if err != nil {
				errChan <- stackerr.Wrap(err)
				return
			}
			reqChan <- req
		}
	}()
	s.wait(ctx, errChan, stopChan)
}

func (s *server) clientWrite(
	ctx context.Context,
	stopChan chan struct{},
	clientConn net.Conn,
	resChan chan jsonRPCResponse,
	notifyChan chan jsonRPCNotification,
) {
	defer func() {
		select {
		case stopChan <- struct{}{}:
		default:
		}
	}()
	defer func() {
		s.logger.Debug("stopping clientWrite")
	}()
	errChan := make(chan error, 1)
	go func() {
		enc := json.NewEncoder(clientConn)
		for {
			select {
			case <-ctx.Done():
				return
			case res := <-resChan:
				err := enc.Encode(res)
				if err != nil {
					errChan <- stackerr.Wrap(err)
					return
				}
			case notif := <-notifyChan:
				s.logger.Debugf("sn2c: %s", notif.toString())
				err := enc.Encode(notif)
				if err != nil {
					errChan <- stackerr.Wrap(err)
					return
				}
			}
		}
	}()
	s.wait(ctx, errChan, stopChan)
}

func (s *server) mux(
	ctx context.Context,
	cctx *clientCtx,
	stopChan chan struct{},
	reqChanFromClient chan jsonRPCRequest,
	resChanToClient chan jsonRPCResponse,
) {
	defer func() {
		select {
		case stopChan <- struct{}{}:
		default:
		}
	}()
	defer s.logger.Debug("mux stop")
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
		case req := <-reqChanFromClient:
			s.logger.Debugf("c2s: %s", req.toString())
			res, err := s.handleRequest(cctx, req)
			if err != nil {
				s.logger.Err(stackerr.Wrap(err))
				goto end
			}
			s.logger.Debugf("s2c: %s", res.toString())
			resChanToClient <- res
		}
	}
end:
}

func (s *server) handleRequest(
	cctx *clientCtx, req jsonRPCRequest,
) (jsonRPCResponse, error) {
	var (
		method string
		resp   jsonRPCResponse = jsonRPCResponse{
			JsonRPC: req.JsonRPC,
			Id:      req.Id,
		}
		versionResult = fmt.Appendf(
			nil, `["eps-go %s","%s"]`,
			Version,
			protocolVersion,
		)
	)
	err := json.Unmarshal(req.Method, &method)
	if err != nil {
		return resp, stackerr.Wrap(err)
	}
	switch method {
	case "blockchain.block.header":
		var (
			params   []int64
			height   int64
			cpHeight int64
		)
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		if len(params) == 2 {
			height = params[0]
			cpHeight = params[1]
		} else if len(params) == 1 {
			height = params[0]
		} else {
			return resp, ErrBadRequest
		}
		if cpHeight == 0 {
			blockHash, err := s.bcli.getBlockHash(height)
			if err != nil {
				return resp, stackerr.Wrap(err)
			}
			blockHeader, err := s.bcli.getBlockHeader(blockHash)
			if err != nil {
				return resp, stackerr.Wrap(err)
			}
			resp.Result = fmt.Appendf(nil, `"%s"`, blockHeader)
			return resp, nil
		}
		// TODO: handle cpHeight != 0
		return resp, ErrNotImplemented
	case "blockchain.block.headers":
		const maxHeaders = 2016
		var (
			params                       []int64
			startHeight, count, cpHeight int64
			info                         blockChainInfo
			hex                          []byte
		)
		err = json.Unmarshal(req.Params, &params)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		if len(params) == 3 {
			cpHeight = params[2]
		}
		if len(params) < 2 {
			return resp, ErrBadRequest
		}
		startHeight = params[0]
		count = params[1]
		info, err := s.bcli.getBlockChainInfo()
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		if startHeight > info.Blocks {
			return resp, ErrBadRequest
		}
		i := startHeight
		end := min(
			startHeight+count, startHeight+maxHeaders, info.Blocks+1,
		)
		nheaders := end - i
		for ; i < end; i++ {
			blockHash, err := s.bcli.getBlockHash(i)
			if err != nil {
				return resp, stackerr.Wrap(err)
			}
			blockHeader, err := s.bcli.getBlockHeader(blockHash)
			if err != nil {
				return resp, stackerr.Wrap(err)
			}
			hex = fmt.Append(hex, blockHeader)
		}
		resp.Result = fmt.Appendf(
			nil,
			`{"hex":"%s","count":%d,"max":%d}`,
			hex, nheaders, maxHeaders,
		)
		// TODO: handle cpHeight
		_ = cpHeight
		return resp, nil
	case "blockchain.estimatefee":
		var (
			params  []int64
			nblocks int64
		)
		err = json.Unmarshal(req.Params, &params)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		if len(params) != 1 {
			return resp, ErrBadRequest
		}
		nblocks = params[0]
		btcPerKB, err := s.bcli.estimateSmartFee(nblocks)
		if err != nil && errors.Is(err, ErrInsufficientData) {
			var mi mempoolInfo
			mi, err = s.bcli.getMempoolInfo()
			if err != nil {
				return resp, stackerr.Wrap(err)
			}
			btcPerKB = mi.MempoolMinFee
			err = nil
		}
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		resp.Result = fmt.Appendf(nil, `%.8f`, btcPerKB)
		return resp, nil
	case "blockchain.headers.subscribe":
		info, err := s.bcli.getBlockChainInfo()
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		blockHeader, err := s.bcli.getBlockHeader(info.BestBlockHash)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		s.wman.subscribeHeaders(cctx)
		resp.Result = fmt.Appendf(
			nil,
			`{"hex":"%s","height":%d}`,
			blockHeader,
			info.Blocks,
		)
		return resp, nil
	case "blockchain.relayfee":
		info, err := s.bcli.getMempoolInfo()
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		resp.Result = fmt.Appendf(nil, `%.8f`, info.MinRelayTXFee)
		return resp, nil
	case "blockchain.scripthash.get_balance":
		var (
			params     [1]string
			scriptHash string
		)
		err = json.Unmarshal(req.Params, &params)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		scriptHash = params[0]
		if len(scriptHash) != 64 {
			return resp, ErrBadRequest
		}
		sHash := hexDecodeReverse(scriptHash)
		sbalance, err := s.wman.getScriptBalance(sHash)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		resp.Result = fmt.Appendf(
			nil,
			`{"confirmed":%d,"unconfirmed":%d}`,
			sbalance.Confirmed,
			sbalance.Unconfirmed,
		)
		return resp, nil
	case "blockchain.scripthash.get_history":
		var (
			params     [1]string
			scriptHash string
		)
		err = json.Unmarshal(req.Params, &params)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		scriptHash = params[0]
		if len(scriptHash) != 64 {
			return resp, ErrBadRequest
		}
		histJson, err := s.wman.getScriptHistory(
			hexDecodeReverse(scriptHash),
		)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		resp.Result = histJson
		return resp, nil
	case "blockchain.scripthash.subscribe":
		var (
			params     [1]string
			scriptHash string
		)
		err = json.Unmarshal(req.Params, &params)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		scriptHash = params[0]
		if len(scriptHash) != 64 {
			return resp, ErrBadRequest
		}
		dshash := hexDecodeReverse(scriptHash)
		s.wman.subscribeScriptHash(cctx, dshash)
		status := s.wman.scriptHashStatus(dshash)
		if status == "" {
			resp.Result = []byte(`null`)
			return resp, nil
		}
		resp.Result = fmt.Appendf(nil, `"%s"`, status)
		return resp, nil
	case "blockchain.transaction.broadcast":
		var (
			params [1]string
			rawTx  string
		)
		err = json.Unmarshal(req.Params, &params)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		rawTx = params[0]
		txHash, err := s.bcli.sendRawTransaction(rawTx)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		resp.Result = fmt.Appendf(nil, `"%s"`, txHash)
		return resp, nil
	case "blockchain.transaction.get":
		var (
			params [1]string
			txHash string
		)
		err = json.Unmarshal(req.Params, &params)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		txHash = params[0]
		rawTx, err := s.wman.getTransaction(hexDecodeReverse(txHash))
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		resp.Result = fmt.Appendf(nil, `"%s"`, rawTx)
		return resp, nil
	case "blockchain.transaction.get_merkle":
		var (
			params [2]json.RawMessage
			txHash string
			height int64
		)
		err = json.Unmarshal(req.Params, &params)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		err = json.Unmarshal(params[0], &txHash)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		err = json.Unmarshal(params[1], &height)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		mproof, err := s.wman.getTxMerkleProof(
			hexDecodeReverse(txHash), height,
		)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		resp.Result, err = json.Marshal(mproof)
		return resp, nil
	case "blockchain.transaction.id_from_pos":
		var (
			params      []json.RawMessage
			height, pos int64
			merkle      bool
		)
		err = json.Unmarshal(req.Params, &params)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		if len(params) < 2 {
			return resp, ErrBadRequest
		}
		if len(params) == 3 {
			err = json.Unmarshal(params[2], &merkle)
			if err != nil {
				return resp, stackerr.Wrap(err)
			}
		}
		err = json.Unmarshal(params[0], &height)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		err = json.Unmarshal(params[1], &pos)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		txdata, err := s.wman.txidFromPos(height, pos, merkle)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		resp.Result, err = json.Marshal(txdata)
		if err != nil {
			return resp, stackerr.Wrap(err)
		}
		return resp, nil
	case "mempool.get_fee_histogram":
		type histogramItem struct {
			fee   float64 // sat/vbyte
			vsize int64   // distance from tip
		}
		// TODO: real implementation
		resp.Result = []byte(`[]`)
		return resp, nil
	case "server.banner":
		resp.Result = []byte(`"Welcome to eps-go"`)
		return resp, nil
	case "server.donation_address":
		resp.Result = []byte(`"to be added"`)
		return resp, nil
	case "server.peers.subscribe":
		resp.Result = []byte(`[]`)
		return resp, nil
	case "server.ping":
		resp.Result = []byte(`null`)
		return resp, nil
	case "server.version":
		params := make([]string, 0, 2)
		err = json.Unmarshal(req.Params, &params)
		if len(params) != 2 {
			return resp, fmt.Errorf("bad params len")
		}
		if params[1] != protocolVersion {
			return resp, fmt.Errorf("unknown protocol version")
		}
		resp := jsonRPCResponse{
			JsonRPC: []byte(`"2.0"`),
			Id:      req.Id,
			Result:  versionResult,
			Error:   nil,
		}
		return resp, nil
	default:
		return resp, ErrNotImplemented
	}
}

func (s *server) wait(ctx context.Context, errChan chan error, stopChan chan struct{}) {
	select {
	case <-ctx.Done():
	case err := <-errChan:
		s.logger.Err(err)
		select {
		case stopChan <- struct{}{}:
		default:
		}
	}
	<-ctx.Done()
}
