// Copyright 2010 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonrpc

import (
	"encoding/json"
	"errors"
	rpc "github.com/lijie/go/rpc"
	"io"
)

var errMissingParams = errors.New("jsonrpc: request body missing params")

type ServerCodec struct {
	dec *json.Decoder // for reading JSON values
	enc *json.Encoder // for writing JSON values
	c   io.Closer

	// temporary work space
	req serverRequest
}

// NewServerCodec returns a new rpc.ServerCodec using JSON-RPC on conn.
func NewServerCodec(conn io.ReadWriteCloser) rpc.ServerCodec {
	return &ServerCodec{
		dec: json.NewDecoder(conn),
		enc: json.NewEncoder(conn),
		c:   conn,
	}
}

func InitServerCodec(codec *ServerCodec, conn io.ReadWriteCloser) rpc.ServerCodec {
	codec.dec = json.NewDecoder(conn)
	codec.enc = json.NewEncoder(conn)
	codec.c = conn
	return codec
}

type serverRequest struct {
	Cmd    uint32           `json:"cmd"`
	Params *json.RawMessage `json:"params"`
	Id     uint32           `json:"id"`
}

func (r *serverRequest) reset() {
	r.Cmd = 0
	r.Params = nil
	r.Id = 0
}

type serverResponse struct {
	Id     uint32      `json:"id"`
	Result interface{} `json:"result"`
	Error  interface{} `json:"error"`
}

func (c *ServerCodec) ReadRequestHeader(r *rpc.Request) error {
	c.req.reset()
	if err := c.dec.Decode(&c.req); err != nil {
		return err
	}
	r.Cmd = c.req.Cmd
	return nil
}

func (c *ServerCodec) ReadRequestBody(x interface{}) error {
	if x == nil {
		return nil
	}
	if c.req.Params == nil {
		return errMissingParams
	}
	// JSON params is array value.
	// RPC params is struct.
	// Unmarshal into array containing struct for now.
	// Should think about making RPC more general.
	var params [1]interface{}
	params[0] = x
	return json.Unmarshal(*c.req.Params, &params)
}

var null = json.RawMessage([]byte("null"))

func (c *ServerCodec) WriteResponse(r *rpc.Response, x interface{}) error {
	resp := serverResponse{Id: r.Seq}
	if r.Error == 0 {
		resp.Result = x
	} else {
		resp.Error = r.Error
	}
	return c.enc.Encode(resp)
}

func (c *ServerCodec) Close() error {
	return c.c.Close()
}

// ServeConn runs the JSON-RPC server on a single connection.
// ServeConn blocks, serving the connection until the client hangs up.
// The caller typically invokes ServeConn in a go statement.
func ServeConn(conn io.ReadWriteCloser) {
	rpc.ServeCodec(NewServerCodec(conn))
}
