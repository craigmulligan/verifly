package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
)

func lookupRecord() {
	txtrecords, _ := net.LookupTXT("basis-test.bike")

	for _, txt := range txtrecords {
		fmt.Println(txt)
	}
}

type payload struct {
	Domain       string
	Callback_url string
}

func route(rw http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var p payload
	err := decoder.Decode(&p)
	if err != nil {
		panic(err)
	}

	log.Println(p.Domain)
}

func main() {
	http.HandleFunc("/", route)

	http.ListenAndServe(":2000", nil)
}
