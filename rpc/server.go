package rpc

import (
	"bufio"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"
	"sync"
	"unicode"
	"unicode/utf8"
)

var debugLog = false

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

const (
	// Defaults used by HandleHTTP
	DefaultRPCPath   = "/_goRPC_"
	DefaultDebugPath = "/debug/rpc"
)

type JsonRequest struct {
	Cmd    uint32           `json:"cmd"`
	Seq    uint32           `json:"seq"`
	Params *json.RawMessage `json:"params"`
}

func (r *JsonRequest) reset() {
	r.Cmd = 0
	r.Seq = 0
	r.Params = nil
}

type JsonResponse struct {
	Cmd    uint32      `json:"cmd"`
	Seq    uint32      `json:"seq"`
	Error  string      `json:"error"`
	Result interface{} `json:"result"`
}

// Request is a header written before every RPC call.  It is used internally
// but documented here as an aid to debugging, such as when analyzing
// network traffic.
type Request struct {
	Cmd  uint32
	Seq  uint32   // sequence number chosen by client
	next *Request // for free list in Server
}

// Response is a header written before every RPC return.  It is used internally
// but documented here as an aid to debugging, such as when analyzing
// network traffic.
type Response struct {
	Cmd   uint32    // echoes that of the Request
	Seq   uint32    // echoes that of the request
	Error uint32    // error, if any.
	next  *Response // for free list in Server
}

// Server represents an RPC Server.
type Server struct {
	// mu         sync.RWMutex // protects the serviceMap
	method   map[uint32]*methodType
	reqLock  sync.Mutex // protects freeReq
	freeReq  *Request
	respLock sync.Mutex // protects freeResp
	freeResp *Response
}

// NewServer returns a new Server.
func NewServer() *Server {
	return &Server{method: make(map[uint32]*methodType)}
}

var DefaultServer = NewServer()

type Error uint32

func (e Error) Error() string {
	return fmt.Sprintf("%d", uint32(e))
}

type ServerCodec interface {
	ReadRequestHeader(*Request) error
	ReadRequestBody(interface{}) error
	WriteResponse(*Response, interface{}) error
	Close() error
}

type methodType struct {
	// method    reflect.Method
	Func      reflect.Value
	ArgType   reflect.Type
	ReplyType reflect.Type
}

// Is this an exported - upper case - name?
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

func (server *Server) readRequestHeader(codec ServerCodec) (mtype *methodType, req *Request, keepReading bool, err error) {
	// Grab the request header.
	req = server.getRequest()
	err = codec.ReadRequestHeader(req)
	if err != nil {
		req = nil
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		err = errors.New("rpc: server cannot decode request: " + err.Error())
		return
	}

	// We read the header successfully.  If we see an error now,
	// we can still recover and move on to the next request.
	keepReading = true

	mtype = server.method[req.Cmd]
	if mtype == nil {
		err = errors.New("rpc: can't find method")
	}
	return
}

func (server *Server) getRequest() *Request {
	server.reqLock.Lock()
	req := server.freeReq
	if req == nil {
		req = new(Request)
	} else {
		server.freeReq = req.next
		*req = Request{}
	}
	server.reqLock.Unlock()
	return req
}

func (server *Server) freeRequest(req *Request) {
	server.reqLock.Lock()
	req.next = server.freeReq
	server.freeReq = req
	server.reqLock.Unlock()
}

func (server *Server) getResponse() *Response {
	server.respLock.Lock()
	resp := server.freeResp
	if resp == nil {
		resp = new(Response)
	} else {
		server.freeResp = resp.next
		*resp = Response{}
	}
	server.respLock.Unlock()
	return resp
}

func (server *Server) freeResponse(resp *Response) {
	server.respLock.Lock()
	resp.next = server.freeResp
	server.freeResp = resp
	server.respLock.Unlock()
}

func (server *Server) readRequest(codec ServerCodec) (mtype *methodType, req *Request, argv, replyv reflect.Value, keepReading bool, err error) {
	mtype, req, keepReading, err = server.readRequestHeader(codec)
	if err != nil {
		if !keepReading {
			return
		}
		// discard body
		codec.ReadRequestBody(nil)
		return
	}

	// Decode the argument value.
	argIsValue := false // if true, need to indirect before calling.
	if mtype.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(mtype.ArgType.Elem())
	} else {
		argv = reflect.New(mtype.ArgType)
		argIsValue = true
	}
	// argv guaranteed to be a pointer now.
	if err = codec.ReadRequestBody(argv.Interface()); err != nil {
		return
	}
	if argIsValue {
		argv = argv.Elem()
	}

	replyv = reflect.New(mtype.ReplyType.Elem())
	return
}

type gobServerCodec struct {
	rwc    io.ReadWriteCloser
	dec    *gob.Decoder
	enc    *gob.Encoder
	encBuf *bufio.Writer
	closed bool
}

func (c *gobServerCodec) ReadRequestHeader(r *Request) error {
	return c.dec.Decode(r)
}

func (c *gobServerCodec) ReadRequestBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *gobServerCodec) WriteResponse(r *Response, body interface{}) (err error) {
	if err = c.enc.Encode(r); err != nil {
		if c.encBuf.Flush() == nil {
			// Gob couldn't encode the header. Should not happen, so if it does,
			// shut down the connection to signal that the connection is broken.
			log.Println("rpc: gob error encoding response:", err)
			c.Close()
		}
		return
	}
	if err = c.enc.Encode(body); err != nil {
		if c.encBuf.Flush() == nil {
			// Was a gob problem encoding the body but the header has been written.
			// Shut down the connection to signal that the connection is broken.
			log.Println("rpc: gob error encoding body:", err)
			c.Close()
		}
		return
	}
	return c.encBuf.Flush()
}

func (c *gobServerCodec) Close() error {
	if c.closed {
		// Only call c.rwc.Close once; otherwise the semantics are undefined.
		return nil
	}
	c.closed = true
	return c.rwc.Close()
}

// ServeConn runs the server on a single connection.
// ServeConn blocks, serving the connection until the client hangs up.
// The caller typically invokes ServeConn in a go statement.
// ServeConn uses the gob wire format (see package gob) on the
// connection.  To use an alternate codec, use ServeCodec.
func (server *Server) ServeConn(ctx context.Context, conn io.ReadWriteCloser) {
	buf := bufio.NewWriter(conn)
	srv := &gobServerCodec{
		rwc:    conn,
		dec:    gob.NewDecoder(conn),
		enc:    gob.NewEncoder(buf),
		encBuf: buf,
	}
	server.ServeCodec(ctx, srv)
}

// ServeCodec is like ServeConn but uses the specified codec to
// decode requests and encode responses.
func (server *Server) ServeCodec(ctx context.Context, codec ServerCodec) {
	sending := new(sync.Mutex)
	arg1 := reflect.ValueOf(ctx)
	for {
		mtype, req, argv, replyv, keepReading, err := server.readRequest(codec)
		if err != nil {
			if debugLog && err != io.EOF {
				log.Println("rpc:", err)
			}
			if !keepReading {
				break
			}
			// send a response if we actually managed to read a header.
			if req != nil {
				server.sendResponse(sending, req, invalidRequest, codec, err)
				server.freeRequest(req)
			}
			continue
		}
		go server.call(sending, mtype, req, arg1, argv, replyv, codec)
	}
	codec.Close()
}

// test
type PendingCall struct {
	sending *sync.Mutex
	mtype   *methodType
	req     *Request
	arg1    reflect.Value
	argv    reflect.Value
	replyv  reflect.Value
	codec   ServerCodec
}

func (pc *PendingCall) Context() context.Context {
	return pc.arg1.Interface().(context.Context)
}

// test
func (server *Server) ServeCodec2(ctx context.Context, codec ServerCodec, ch chan interface{}) {
	sending := new(sync.Mutex)
	arg1 := reflect.ValueOf(ctx)
	for {
		mtype, req, argv, replyv, keepReading, err := server.readRequest(codec)
		if err != nil {
			if debugLog && err != io.EOF {
				log.Println("rpc:", err)
			}
			if !keepReading {
				break
			}
			// send a response if we actually managed to read a header.
			if req != nil {
				server.sendResponse(sending, req, invalidRequest, codec, err)
				server.freeRequest(req)
			}
			continue
		}
		ch <- &PendingCall{
			sending: sending,
			mtype:   mtype,
			req:     req,
			arg1:    arg1,
			argv:    argv,
			replyv:  replyv,
			codec:   codec,
		}
		// go server.call(sending, mtype, req, arg1, argv, replyv, codec)
	}
	codec.Close()
}

// test
func (server *Server) Call(pc interface{}) {
	call := pc.(*PendingCall)
	server.call(call.sending, call.mtype, call.req, call.arg1, call.argv, call.replyv, call.codec)
}

// test
// func (server *Server) callWithChan(ch chan *PendingCall) {
// 	for {
// 		call := <-ch
// 		server.call(call.sending, call.mtype, call.req, call.arg1, call.argv, call.replyv, call.codec)
// 	}
// }

// A value sent as a placeholder for the server's response value when the server
// receives an invalid request. It is never decoded by the client since the Response
// contains an error when it is used.
var invalidRequest = struct{}{}

func (server *Server) sendResponse(sending *sync.Mutex, req *Request, reply interface{}, codec ServerCodec, errmsg error) {
	resp := server.getResponse()
	// Encode the response header
	resp.Cmd = req.Cmd
	if errmsg != nil {
		errcode, ok := errmsg.(Error)
		if ok {
			resp.Error = uint32(errcode)
		} else {
			resp.Error = uint32(0xFFFFFFFF)
		}
	}
	resp.Seq = req.Seq
	sending.Lock()
	err := codec.WriteResponse(resp, reply)
	if debugLog && err != nil {
		log.Println("rpc: writing response:", err)
	}
	sending.Unlock()
	server.freeResponse(resp)
}

func (server *Server) call(sending *sync.Mutex, mtype *methodType, req *Request, arg1, argv, replyv reflect.Value, codec ServerCodec) {
	function := mtype.Func
	// Invoke the method, providing a new value for the reply.
	returnValues := function.Call([]reflect.Value{arg1, argv, replyv})
	// The return value for the method is an error.
	errInter := returnValues[0].Interface()
	var err error
	if errInter != nil {
		err = errInter.(error)
	}
	server.sendResponse(sending, req, replyv.Interface(), codec, err)
	server.freeRequest(req)
}

func (server *Server) Register(cmd uint32, function interface{}) error {
	mtype := reflect.TypeOf(function)
	// Method needs three ins: receiver, *args, *reply.
	if mtype.NumIn() != 3 {
		return errors.New("method has wrong number of ins")
	}
	// First arg need not be a pointer.
	argType := mtype.In(1)
	if !isExportedOrBuiltinType(argType) {
		return errors.New("argument type not exported")
	}
	// Second arg must be a pointer.
	replyType := mtype.In(2)
	if replyType.Kind() != reflect.Ptr {
		return errors.New("reply type not a pointer")
	}
	// Reply type must be exported.
	if !isExportedOrBuiltinType(replyType) {
		return errors.New("reply type not exported")
	}
	// Method needs one out.
	if mtype.NumOut() != 1 {
		return errors.New("has wrong number of outs:")
	}
	// The return type of the method must be error.
	if returnType := mtype.Out(0); returnType != typeOfError {
		return errors.New("not error")
	}
	server.method[cmd] = &methodType{Func: reflect.ValueOf(function), ArgType: argType, ReplyType: replyType}
	return nil
}

// ServeConn runs the DefaultServer on a single connection.
// ServeConn blocks, serving the connection until the client hangs up.
// The caller typically invokes ServeConn in a go statement.
// ServeConn uses the gob wire format (see package gob) on the
// connection.  To use an alternate codec, use ServeCodec.
func ServeConn(conn io.ReadWriteCloser) {
	DefaultServer.ServeConn(context.Background(), conn)
}

// ServeCodec is like ServeConn but uses the specified codec to
// decode requests and encode responses.
func ServeCodec(codec ServerCodec) {
	DefaultServer.ServeCodec(context.Background(), codec)
}

// Register publishes the receiver's methods in the DefaultServer.
func Register(cmd uint32, function interface{}) error { return DefaultServer.Register(cmd, function) }

// Can connect to RPC service using HTTP CONNECT to rpcPath.
var connected = "200 Connected to Go RPC"
