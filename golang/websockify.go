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

func forwardTcp(wsConn *websocket.Conn, conn net.Conn) {
	var tcpBuffer [1024]byte
	defer func() {
		if conn != nil {
			conn.Close()
		}
		if wsConn != nil {
			wsConn.Close()
		}
	}()
	for {
		if (conn == nil) || (wsConn == nil) {
			return
		}
		n, err := conn.Read(tcpBuffer[0:])
		if err != nil {
			log.Printf("%s: TCP.Read() failed: %s", time.Now().Format(time.Stamp), err)
			return
		}
		if err := wsConn.WriteMessage(websocket.BinaryMessage, tcpBuffer[0:n]); err != nil {
			log.Printf("%s: websocket.WriteMessage() failed: %s", time.Now().Format(time.Stamp), err)
			return
		}
	}
}

func forwardWeb(wsConn *websocket.Conn, conn net.Conn) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("%s: websocket forwarding side panic: %s", time.Now().Format(time.Stamp), err)
		}
		if conn != nil {
			conn.Close()
		}
		if wsConn != nil {
			wsConn.Close()
		}
	}()
	for {
		if (conn == nil) || (wsConn == nil) {
			return
		}

		_, buffer, err := wsConn.ReadMessage()
		if err != nil {
			log.Printf("%s: websocket.ReadMessage() failed: %s", time.Now().Format(time.Stamp), err)
			return
		}
		if _, err := conn.Write(buffer); err != nil {
			log.Printf("%s: tcp.Write() failed: %s", time.Now().Format(time.Stamp), err)
			return
		}
	}
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		Subprotocols: websocket.Subprotocols(r),
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("%s: failed to upgrade to WS: %s", time.Now().Format(time.Stamp), err)
		return
	}

	vnc, err := net.Dial("tcp", *targetAddr)
	if err != nil {
		log.Printf("%s: failed to bind to the VNC Server: %s", time.Now().Format(time.Stamp), err)
	}

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
		log.Printf("Serving %s at %s", *web, *sourceAddr)
		http.Handle("/", http.FileServer(http.Dir(*web)))
	}
	log.Printf("Serving WS of %s at %s", *targetAddr, *sourceAddr)
	http.HandleFunc("/websockify", serveWs)
	log.Fatal(http.ListenAndServe(*sourceAddr, nil))
}
