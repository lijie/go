package main

import (
	"flag"
	sendpaste "github.com/lijie/go/sendpaste"
	"net/http"
)

func main() {
	flag.Parse()
	ps := sendpaste.NewPasteServer()
	go ps.RunWithHttp(":20003", "/paste", http.DefaultServeMux)
	http.ListenAndServe(":8089", nil)
}
