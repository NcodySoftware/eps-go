package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrJsonRPC = errors.New("jsonrpc error")
)

type jsonRPCRequest struct {
	JsonRPC json.RawMessage `json:"jsonrpc"`
	Id      json.RawMessage `json:"id"`
	Method  json.RawMessage `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func (j *jsonRPCRequest) toString() string {
	return fmt.Sprintf(
		`{"id":%s,"jsonrpc":%s,"method":%s,"params":%s}`,
		j.Id, j.JsonRPC, j.Method, j.Params,
	)
}

type jsonRPCNotification struct {
	JsonRPC json.RawMessage `json:"jsonrpc"`
	Method  json.RawMessage `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func (j *jsonRPCNotification) toString() string {
	return fmt.Sprintf(
		`{"jsonrpc":%s,"method":%s,"params":%s}`,
		j.JsonRPC, j.Method, j.Params,
	)
}

type jsonRPCResponse struct {
	JsonRPC json.RawMessage `json:"jsonrpc"`
	Id      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func (j *jsonRPCResponse) toString() string {
	return fmt.Sprintf(
		`{"id":%s,"jsonrpc":%s,"result":%s,"error":%s}`,
		j.Id, j.JsonRPC, j.Result, j.Error,
	)
}

func checkRPCErr(err error, jr jsonRPCResponse) error {
	if err != nil {
		return err
	}
	if len(jr.Error) == 0 {
		return nil
	}
	if bytes.Equal([]byte(`null`), jr.Error) {
		return nil
	}
	return errors.Join(ErrJsonRPC, fmt.Errorf("%s", jr.Error))
}
