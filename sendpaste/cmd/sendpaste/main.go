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
)

func savePaste(c *rpc.Client) {
	var arg sendpaste.PasteData
	var reply sendpaste.PasteID
	arg.Data = []byte(os.Args[1])
	err := c.Call("Paste.Add", &arg, &reply)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Paste Success:%d\n", reply.ID)
}

func saveFile(c *rpc.Client) {
	var arg sendpaste.PasteData
	var reply sendpaste.PasteID
	var err error
	arg.Data, err = ioutil.ReadFile(sendFile)
	if err != nil {
		log.Fatal(err)
	}
	arg.FileName = sendFile
	err = c.Call("Paste.Add", &arg, &reply)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Paste Success:%d\n", reply.ID)

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

var enableGet bool
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
	if len(sendFile) > 0 {
		saveFile(c)
		return
	}

	savePaste(c)
}
