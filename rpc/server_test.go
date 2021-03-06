package rpc

import (
	"net"
	"testing"
	"time"
)

var runserver = false

type AddParams struct {
	A, B int
}

func Add(codec ServerCodec, arg *AddParams, reply *int) error {
	*reply = arg.A + arg.B
	return nil
}

func Fail(codec ServerCodec, arg *AddParams, reply *int) error {
	*reply = arg.A + arg.B
	return Error(777)
}

func Timeout(codec ServerCodec, arg *AddParams, reply *int) error {
	*reply = arg.A + arg.B
	time.Sleep(1 * time.Second)
	return nil
}

func runServer(t *testing.T) {
	if runserver {
		return
	}
	var err error
	err = Register(100, Add)
	err = Register(101, Fail)
	err = Register(102, Timeout)
	l, err := net.Listen("tcp", ":20003")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				t.Fatal(err)
			}
			ServeConn(conn)
		}
	}()
	runserver = true
}

func runClient(t *testing.T) *Client {
	c, err := Dial("tcp", ":20003")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAddCall(t *testing.T) {
	runServer(t)
	c := runClient(t)
	defer c.Close()

	param := &AddParams{100, 200}
	reply := 0
	err := c.Call(100, &param, &reply)
	if err != nil {
		t.Fatal(err)
	}
	if reply != 300 {
		t.Fatal("call failed")
	}
}

func TestFailCall(t *testing.T) {
	runServer(t)
	c := runClient(t)
	defer c.Close()

	param := &AddParams{100, 200}
	reply := 0
	err := c.Call(101, &param, &reply)
	if err == nil || err.Error() != "777" {
		t.Fatal("should return err 777")
	}
}

func TestTimeoutCall(t *testing.T) {
	runServer(t)
	c := runClient(t)
	defer c.Close()

	param := &AddParams{100, 200}
	reply := 0
	err := c.CallWithTimeout(102, &param, &reply, 500*time.Millisecond)
	if err != ErrTimeout {
		t.Fatal("should return err:", ErrTimeout, "but:", err)
	}
}

func TestTimeoutCall2(t *testing.T) {
	runServer(t)
	c := runClient(t)
	defer c.Close()

	param := &AddParams{100, 200}
	reply := 0
	err := c.CallWithTimeout(102, &param, &reply, 2*time.Second)
	if err != nil {
		t.Fatal("should return nil, but:", err)
	}
}

func TestTimeoutCall3(t *testing.T) {
	runServer(t)
	c := runClient(t)
	defer c.Close()

	param := &AddParams{100, 200}
	reply := 0
	call := c.GoWithTimeout(102, &param, &reply, make(chan *Call, 1), 500*time.Second)
	_ = <-call.Done
	if call.Error != nil {
		t.Fatal("should return err:", ErrTimeout, "but:", call.Error)
	}
}

func TestTimeoutCall4(t *testing.T) {
	runServer(t)
	c := runClient(t)
	defer c.Close()

	param := &AddParams{100, 200}
	reply := 0
	call := c.GoWithTimeout(102, &param, &reply, make(chan *Call, 1), 2*time.Second)
	_ = <-call.Done
	if call.Error != nil {
		t.Fatal("should return nil, but:", call.Error)
	}
}
