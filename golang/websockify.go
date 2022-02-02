// A Go version WebSocket to TCP socket proxy
// Copyright 2021 Michael.liu
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

var (
	source_addr *string
	target_addr *string
	web         *string
)

func init() {
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
	source_addr = flag.String("l", "127.0.0.1:8080", "http service address")
	target_addr = flag.String("t", "127.0.0.1:5900", "vnc service address")
	web = flag.String("web", path, "web root folder")
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func forwardtcp(wsconn *websocket.Conn, conn net.Conn) {
	var tcpbuffer [1024]byte
	defer wsconn.Close()
	defer conn.Close()
	for {
		n, err := conn.Read(tcpbuffer[0:])
		if err != nil {
			log.Println("TCP Read failed")
			return
		} else {
			if err := wsconn.WriteMessage(websocket.BinaryMessage, tcpbuffer[0:n]); err != nil {
				log.Println(err)
			}
		}
	}
}

func forwardweb(wsconn *websocket.Conn, conn net.Conn) {
	defer wsconn.Close()
	defer conn.Close()
	for {
		defer func() {
			if err := recover(); err != nil {
				log.Println("Reading from websocket failed")
				return
			}
		}()
		_, buffer, err := wsconn.ReadMessage()
		if err == nil {
			if _, err := conn.Write(buffer); err != nil {
				log.Println("tcp write: ", err)
			}
		}
	}
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	vnc, err := net.Dial("tcp", *target_addr)
	go forwardtcp(ws, vnc)
	go forwardweb(ws, vnc)

}

func main() {
	flag.Parse()
	log.SetFlags(0)
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}

	if *web != path {
		http.Handle("/", http.FileServer(http.Dir(*web)))
	}
	http.HandleFunc("/websockify", serveWs)
	log.Fatal(http.ListenAndServe(*source_addr, nil))
}
