package main

import (
	"flag"
	"log"

	"github.com/danilostorm/storplay/host/internal/signaling"
)

func main() {
	addr := flag.String("listen", "0.0.0.0:47990", "HTTP/WebSocket listen address")
	stun := flag.String("stun", "", "optional STUN URL, for example stun:stun.example.com:3478")
	webDir := flag.String("web-dir", "", "optional directory containing the built StorPlay web client")
	flag.Parse()

	server := signaling.Server{
		Addr:    *addr,
		StunURL: *stun,
		WebDir:  *webDir,
	}

	log.Printf("open %s/healthz to verify the host", signaling.DescribeURL(*addr))
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
