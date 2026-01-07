package jsonrpc

import (
	"encoding/json"
	"fmt"
)

type Request struct {
	JsonRPC json.RawMessage `json:"jsonrpc"`
	Id      json.RawMessage `json:"id"`
	Method  json.RawMessage `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func (j *Request) String() string {
	return fmt.Sprintf(
		`{"id":%s,"jsonrpc":%s,"method":%s,"params":%s}`,
		j.Id, j.JsonRPC, j.Method, j.Params,
	)
}

type Notification struct {
	JsonRPC json.RawMessage `json:"jsonrpc"`
	Method  json.RawMessage `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func (j *Notification) String() string {
	return fmt.Sprintf(
		`{"jsonrpc":%s,"method":%s,"params":%s}`,
		j.JsonRPC, j.Method, j.Params,
	)
}

type Response struct {
	JsonRPC json.RawMessage `json:"jsonrpc"`
	Id      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func (j *Response) String() string {
	return fmt.Sprintf(
		`{"id":%s,"jsonrpc":%s,"result":%s,"error":%s}`,
		j.Id, j.JsonRPC, j.Result, j.Error,
	)
}

func trySend[T any](c chan<- T, v T) bool {
	select {
	case c <- v:
		return true
	default:
		return false
	}
}
