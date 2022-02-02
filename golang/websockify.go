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
	"time"

	"github.com/gorilla/websocket"
)

var (
	sourceAddr *string
	targetAddr *string
	web        *string
)

func init() {
	path, err := os.Getwd()
	if err != nil {
		log.Printf("Could net get current working directory: %s", err)
	}
	sourceAddr = flag.String("l", "127.0.0.1:8080", "http service address")
	targetAddr = flag.String("t", "127.0.0.1:5900", "vnc service address")
	web = flag.String("web", path, "web root folder")
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func forwardTcp(wsConn *websocket.Conn, conn net.Conn) {
	var tcpBuffer [1024]byte
	defer wsConn.Close()
	defer conn.Close()
	for {
		n, err := conn.Read(tcpBuffer[0:])
		if err != nil {
			log.Printf("%s: reading from TCP failed: %s", time.Now().Format(time.Stamp), err)
			return
		} else {
			if err := wsConn.WriteMessage(websocket.BinaryMessage, tcpBuffer[0:n]); err != nil {
				log.Printf("%s: writing to WS failed: %s", time.Now().Format(time.Stamp), err)
			}
		}
	}
}

func forwardWeb(wsConn *websocket.Conn, conn net.Conn) {
	defer wsConn.Close()
	defer conn.Close()
	for {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("%s: reading from WS failed: %s", time.Now().Format(time.Stamp), err)
				return
			}
		}()
		_, buffer, err := wsConn.ReadMessage()
		if err == nil {
			if _, err := conn.Write(buffer); err != nil {
				log.Printf("%s: writing to TCP failed: %s", time.Now().Format(time.Stamp), err)
			}
		}
	}
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("%s: failed to upgrade to WS: %s", time.Now().Format(time.Stamp), err)
		return
	}
	vnc, err := net.Dial("tcp", *targetAddr)
	go forwardTcp(ws, vnc)
	go forwardWeb(ws, vnc)

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
	log.Fatal(http.ListenAndServe(*sourceAddr, nil))
}
