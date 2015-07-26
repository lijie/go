package sendpaste

import (
	"container/list"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"sync"
	"time"
)

type pasteData struct {
	PasteData
	time time.Time
}

type Paste struct {
	s *PasteServer
}

type PasteServer struct {
	dataLock sync.Mutex
	dataList *list.List
	rpc      *rpc.Server
}

func (p *Paste) Get(arg int, reply *PasteData) error {
	server := p.s
	server.dataLock.Lock()
	if server.dataList.Len() == 0 {
		server.dataLock.Unlock()
		return nil
	}
	pd := server.dataList.Front().Value.(*pasteData)
	server.dataLock.Unlock()
	reply.Data = pd.Data
	reply.FileName = pd.FileName
	return nil
}

func (p *Paste) Add(arg *PasteData, reply *PasteID) error {
	server := p.s
	now := time.Now()
	pd := &pasteData{
		// Data: arg.Data,
		// FileName:       arg.FileName,
		time: now,
	}
	pd.Data = arg.Data
	pd.FileName = arg.FileName
	server.dataLock.Lock()
	server.dataList.PushFront(pd)
	if server.dataList.Len() > 100 {
		e := server.dataList.Back()
		server.dataList.Remove(e)
	}
	server.dataLock.Unlock()
	reply.ID = now.UnixNano()
	return nil
}

func (s *PasteServer) RunWithHttp(addr string, patten string, mux *http.ServeMux) {
	rpcServer := rpc.NewServer()
	p := &Paste{
		s: s,
	}
	rpcServer.Register(p)

	mux.HandleFunc(patten, func(w http.ResponseWriter, r *http.Request) {
		s.dataLock.Lock()
		defer s.dataLock.Unlock()
		for e := s.dataList.Front(); e != nil; e = e.Next() {
			// do something with e.Value
			d := e.Value.(*pasteData)
			io.WriteString(w, string(d.Data))
		}
	})

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	for {
		c, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go rpcServer.ServeConn(c)
	}

}

func NewPasteServer() *PasteServer {
	paste := &PasteServer{
		dataList: list.New(),
		rpc:      rpc.NewServer(),
	}

	return paste
}
