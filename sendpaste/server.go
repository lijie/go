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

type Paste struct {
	s *PasteServer
}

type PasteServer struct {
	dataLock sync.Mutex // protect dataList and dataMap
	dataList *list.List
	dataMap  map[int64]*PasteData
	rpc      *rpc.Server
}

func (p *Paste) Get(arg int, reply *PasteData) error {
	server := p.s
	server.dataLock.Lock()
	if server.dataList.Len() == 0 {
		server.dataLock.Unlock()
		return nil
	}
	pd := server.dataList.Front().Value.(*PasteData)
	server.dataLock.Unlock()
	*reply = *pd
	return nil
}

func isFile(pd *PasteData) bool {
	if len(pd.FileName) > 0 {
		return true
	}
	return false
}

func (p *Paste) List(arg int, reply *[]*PasteData) error {
	server := p.s
	server.dataLock.Lock()
	defer server.dataLock.Unlock()

	if server.dataList.Len() == 0 {
		return nil
	}

	count := 0
	for e := server.dataList.Front(); e != nil && count < arg; e = e.Next() {
		pd := e.Value.(*PasteData)
		if isFile(pd) {
			npd := new(PasteData)
			*npd = *pd
			npd.Data = []byte("")
			*reply = append(*reply, npd)
		} else {
			*reply = append(*reply, pd)
		}
	}
	return nil
}

func (p *Paste) GetByID(arg int64, reply *PasteData) error {
	server := p.s
	server.dataLock.Lock()
	if server.dataList.Len() == 0 {
		server.dataLock.Unlock()
		return nil
	}
	pd, ok := server.dataMap[arg]
	server.dataLock.Unlock()

	if !ok {
		return nil
	}
	*reply = *pd
	return nil
}

func (p *Paste) Add(arg *SendData, reply *int64) error {
	server := p.s
	now := time.Now()
	pd := &PasteData{
		Data:       arg.Data,
		FileName:   arg.FileName,
		CreateTime: now,
		ID:         now.UnixNano(),
	}

	server.dataLock.Lock()
	defer server.dataLock.Unlock()

	// add to list
	server.dataList.PushFront(pd)

	// discard old data
	if server.dataList.Len() > 100 {
		e := server.dataList.Back()
		delete(server.dataMap, e.Value.(*PasteData).ID)
		server.dataList.Remove(e)
	}

	*reply = pd.ID

	// add to map
	server.dataMap[pd.ID] = pd
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
			d := e.Value.(*PasteData)
			if isFile(d) {
				io.WriteString(w, d.FileName)
			} else {
				io.WriteString(w, string(d.Data))
			}
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
		dataMap:  make(map[int64]*PasteData),
		rpc:      rpc.NewServer(),
	}

	return paste
}
