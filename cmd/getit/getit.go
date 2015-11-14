package main

import (
	"net/http"
	"log"
	"io"
	"encoding/base64"
	"os/exec"
	"os"
)

func init() {
}

var errorJson = "{\"result\"=\"Error\"}"
var okJson = "{\"result\"=\"OK\"}"

func handleGet(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	defer r.Body.Close()
	url := r.FormValue("url")
	if len(url) == 0 {
		log.Printf("no url\n")
		io.WriteString(w, errorJson)
		return
	}

	decode_url, err := base64.StdEncoding.DecodeString(url)
	if err != nil {
		log.Printf("decode err %v\n", err)
		io.WriteString(w, errorJson)
		return
	}
	// fmt.Println(string(decode_url))

	// dst := fmt.Sprintf("/tmp/getit/%d", time.Now().UnixNano())
	cmd := exec.Command("wget", string(decode_url), "-P", "/tmp/getit")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("wget err %s, %v\n", out, err)
		io.WriteString(w, errorJson)
		// os.Remove(dst)
		return
	}
	io.WriteString(w, okJson)
}

func main() {
	http.HandleFunc("/get", handleGet)
	os.MkdirAll("/tmp/getit", 0777)
	http.Handle("/download", http.FileServer(http.Dir("/tmp/getit")))
	log.Fatal(http.ListenAndServe(":9002", nil))
}
