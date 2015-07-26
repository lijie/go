package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/lijie/go/sendpaste"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"path/filepath"
)

func savePaste(c *rpc.Client) {
	var arg sendpaste.SendData
	var reply int64
	arg.Data = []byte(os.Args[1])
	err := c.Call("Paste.Add", &arg, &reply)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Paste Success:%d\n", reply)
}

func saveFile(c *rpc.Client) {
	var arg sendpaste.SendData
	var reply int64
	var err error
	arg.Data, err = ioutil.ReadFile(sendFile)
	if err != nil {
		log.Fatal(err)
	}
	arg.FileName = filepath.Base(sendFile)
	err = c.Call("Paste.Add", &arg, &reply)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Paste Success:%d\n", reply)
}

func getPaste(c *rpc.Client, count int) {
	var reply sendpaste.PasteData
	err := c.Call("Paste.Get", count, &reply)
	if err != nil {
		log.Fatal(err)
	}
	if len(reply.FileName) > 0 {
		f, err := os.Create(reply.FileName)
		if err != nil {
			log.Fatal(err)
		}
		f.Write(reply.Data)
		f.Close()
		return
	}
	fmt.Println(string(reply.Data))
	return
}

func getList(c *rpc.Client, count int) {
	var reply []*sendpaste.PasteData
	err := c.Call("Paste.List", count, &reply)
	if err != nil {
		log.Fatal(err)
	}
	for i := range reply {
		pd := reply[i]
		if len(pd.FileName) > 0 {
			fmt.Printf("file: %s, id: %d\n", pd.FileName, pd.ID)
		} else {
			fmt.Printf("paste: %s\n", string(pd.Data))
		}
	}
	return
}

var enableGet bool
var enableList bool
var sendFile string

type pasteConfig struct {
	ServerAddr string
	Auth       string
}

func readPasteConfig() *pasteConfig {
	pc := &pasteConfig{
		ServerAddr: "127.0.0.1:20003",
		Auth:       "",
	}
	path := os.Getenv("HOME") + "/.sendpaste.json"
	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("cannot found %s, user default config\n", path)
		return pc
	}
	err = json.Unmarshal(b, pc)
	if err != nil {
		log.Fatal(err)
	}
	return pc
}

func main() {
	flag.BoolVar(&enableGet, "g", false, "get recent paste data")
	flag.BoolVar(&enableList, "l", false, "get recent paste list")
	flag.StringVar(&sendFile, "f", "", "paste file")
	flag.Parse()

	pc := readPasteConfig()

	c, err := rpc.Dial("tcp", pc.ServerAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	if len(os.Args) < 2 {
		getPaste(c, 1)
		return
	}
	if enableList {
		getList(c, 10)
		return
	}
	if len(sendFile) > 0 {
		saveFile(c)
		return
	}

	savePaste(c)
}
