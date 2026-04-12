package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	quicnet "github.com/DiegoSandival/synap2p-go"
)

func main() {
	keyPath := flag.String("key", "./server_identity.key", "path to the server identity key")
	listen := flag.String("listen", "/ip4/0.0.0.0/udp/4001/quic-v1,/ip6/::/udp/4001/quic-v1", "comma-separated QUIC listen multiaddrs")
	bootstrap := flag.String("bootstrap", "", "comma-separated bootstrap peer multiaddrs")
	protocolPrefix := flag.String("protocol", "/synap2p", "protocol prefix for the DHT")
	flag.Parse()

	listenAddrs := splitCSV(*listen)
	if len(listenAddrs) == 0 {
		log.Fatal("at least one listen address is required")
	}

	opts := []quicnet.Option{
		quicnet.WithKeyPath(*keyPath),
		quicnet.WithListenAddrs(listenAddrs...),
		quicnet.WithProtocolPrefix(*protocolPrefix),
	}

	if bootstrapPeers := splitCSV(*bootstrap); len(bootstrapPeers) > 0 {
		opts = append(opts, quicnet.WithBootstrapPeers(bootstrapPeers...))
	}

	server, err := quicnet.NewServer(opts...)
	if err != nil {
		log.Fatalf("create server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			log.Printf("close server: %v", err)
		}
	}()

	log.Printf("server peer id: %s", server.ID())
	for _, addr := range server.Addrs() {
		log.Printf("listening on: %s/p2p/%s", addr, server.ID())
	}

	log.Print("server running; press Ctrl+C to stop")
	waitForSignal()
	log.Print("shutdown signal received")
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	return items
}

func waitForSignal() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	<-signals
}