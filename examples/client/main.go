package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	quicnet "github.com/DiegoSandival/synap2p-go"
	"github.com/libp2p/go-libp2p/core/peer"
)

func main() {
	keyPath := flag.String("key", "./client_identity.key", "path to the client identity key")
	relayAddr := flag.String("relay", "", "relay multiaddr to reserve a slot on")
	peerAddr := flag.String("peer", "", "peer multiaddr to connect to after startup")
	bootstrap := flag.String("bootstrap", "", "comma-separated bootstrap peer multiaddrs")
	topic := flag.String("topic", "chat.global", "pubsub topic to subscribe to")
	message := flag.String("message", "", "message to publish after joining the topic")
	directPeerID := flag.String("direct-peer", "", "peer ID to send a direct message to")
	directMessage := flag.String("direct-message", "", "direct message payload to send")
	protocolPrefix := flag.String("protocol", "/synap2p", "protocol prefix for the DHT")
	flag.Parse()

	opts := []quicnet.Option{
		quicnet.WithKeyPath(*keyPath),
		quicnet.WithProtocolPrefix(*protocolPrefix),
		quicnet.WithDirectMessageHandler(func(ctx context.Context, from peer.ID, data []byte) {
			log.Printf("direct message from %s: %s", from, string(data))
		}),
	}

	if bootstrapPeers := splitCSV(*bootstrap); len(bootstrapPeers) > 0 {
		opts = append(opts, quicnet.WithBootstrapPeers(bootstrapPeers...))
	}

	client, err := quicnet.NewClient(opts...)
	if err != nil {
		log.Fatalf("create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("close client: %v", err)
		}
	}()

	log.Printf("client peer id: %s", client.ID())
	for _, addr := range client.Addrs() {
		log.Printf("listening on: %s/p2p/%s", addr, client.ID())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if *relayAddr != "" {
		if err := client.ConnectToRelay(ctx, *relayAddr); err != nil {
			log.Fatalf("connect to relay: %v", err)
		}
		log.Printf("connected to relay: %s", *relayAddr)
		for _, addr := range client.Addrs() {
			if strings.Contains(addr, "/p2p-circuit") {
				log.Printf("relay address: %s/p2p/%s", addr, client.ID())
			}
		}
	}

	if *peerAddr != "" {
		if err := client.ConnectToPeer(ctx, *peerAddr); err != nil {
			log.Fatalf("connect to peer: %v", err)
		}
		log.Printf("connected to peer: %s", *peerAddr)
	}

	if *topic != "" {
		if err := client.Subscribe(*topic, func(msg []byte) {
			log.Printf("topic %s: %s", *topic, string(msg))
		}); err != nil {
			log.Fatalf("subscribe to topic: %v", err)
		}
		log.Printf("subscribed to topic: %s", *topic)
	}

	if *message != "" {
		publishCtx, publishCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer publishCancel()
		if err := client.Publish(publishCtx, *topic, []byte(*message)); err != nil {
			log.Fatalf("publish message: %v", err)
		}
		log.Printf("published message to %s", *topic)
	}

	if *directPeerID != "" && *directMessage != "" {
		peerID, err := peer.Decode(*directPeerID)
		if err != nil {
			log.Fatalf("parse direct peer ID: %v", err)
		}

		directCtx, directCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer directCancel()
		if err := client.SendDirectMessage(directCtx, peerID, []byte(*directMessage)); err != nil {
			log.Fatalf("send direct message: %v", err)
		}
		log.Printf("direct message sent to %s", peerID)
	}

	log.Print("client running; press Ctrl+C to stop")
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