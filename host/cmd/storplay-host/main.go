package main

import (
	"flag"
	"log"

	"github.com/danilostorm/storplay/host/internal/signaling"
	"github.com/danilostorm/storplay/host/internal/sunshine"
)

func main() {
	addr := flag.String("listen", "0.0.0.0:48120", "HTTP/WebSocket listen address")
	stun := flag.String("stun", "", "optional STUN URL, for example stun:stun.example.com:3478")
	webDir := flag.String("web-dir", "", "optional directory containing the built StorPlay web client")
	sunshineURL := flag.String("sunshine", "http://127.0.0.1:47989", "Sunshine/GameStream HTTP base URL; empty disables probing")
	testPattern := flag.Bool("test-pattern", false, "stream an embedded H264 diagnostic frame instead of a real media source")
	flag.Parse()

	var sunshineClient *sunshine.Client
	if *sunshineURL != "" {
		sunshineClient = sunshine.New(*sunshineURL, sunshine.NewUniqueID())
		log.Printf("Sunshine adapter: probing %s on demand", *sunshineURL)
	}

	server := signaling.Server{
		Addr:        *addr,
		StunURL:     *stun,
		WebDir:      *webDir,
		Sunshine:    sunshineClient,
		TestPattern: *testPattern,
	}

	log.Printf("open %s/healthz to verify the host", signaling.DescribeURL(*addr))
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
