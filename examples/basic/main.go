package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	quicnet "github.com/DiegoSandival/synap2p-go"
)

func main() {
	configPath := filepath.Join(".", "config.yaml")
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := quicnet.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error cargando config %s: %v", configPath, err)
	}

	node, err := quicnet.NewNode(cfg)
	if err != nil {
		log.Fatalf("Error creando node: %v", err)
	}
	defer node.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := node.Start(ctx); err != nil {
		log.Fatalf("Error arrancando node: %v", err)
	}

	fmt.Printf("Nodo %s levantado en modo %s\n", node.ID(), node.Mode())
	for _, addr := range node.Addrs() {
		fmt.Printf(" - %s/p2p/%s\n", addr, node.ID())
	}

	requestID, err := node.Subscribe("demo.chat")
	if err != nil {
		log.Fatalf("Error suscribiéndose al tópico: %v", err)
	}
	fmt.Printf("Suscripción solicitada. RequestID=%x\n", requestID)
	fmt.Println("Esperando eventos. Publica desde otro nodo compatible para ver mensajes entrantes...")

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Cerrando ejemplo basic...")
			return
		case event, ok := <-node.Events():
			if !ok {
				return
			}
			printEvent(event)
		}
	}
}

func printEvent(event []byte) {
	if len(event) < 20 {
		fmt.Printf("Evento corto: %v\n", event)
		return
	}

	opcode := event[3]
	requestID := event[4:20]
	switch opcode {
	case 0x0E:
		fmt.Printf("[EVENT_PUBSUB_MSG] request=%x payload=%s\n", requestID, string(event[20:]))
	case 0x0F:
		fmt.Printf("[EVENT_DIRECT_MSG] request=%x raw=%x\n", requestID, event[20:])
	case 0x10:
		fmt.Printf("[EVENT_PEER_DISCOVERED] raw=%x\n", event[20:])
	default:
		fmt.Printf("[EVENT opcode=0x%02X] request=%x payload=%x\n", opcode, requestID, event[20:])
	}
}
