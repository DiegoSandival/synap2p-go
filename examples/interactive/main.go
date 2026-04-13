package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	quicnet "github.com/DiegoSandival/synap2p-go"
	"github.com/libp2p/go-libp2p/core/peer"
)

const commandTimeout = 15 * time.Second

func main() {
	keyPath := flag.String("key", "./interactive_identity.key", "path to the client identity key")
	bootstrap := flag.String("bootstrap", "", "comma-separated bootstrap peer multiaddrs")
	relayAddr := flag.String("relay", "", "relay multiaddr to reserve a slot on after startup")
	defaultTopic := flag.String("topic", "chat.global", "default pubsub topic")
	protocolPrefix := flag.String("protocol", "/synap2p", "protocol prefix for the DHT")
	flag.Parse()

	opts := []quicnet.Option{
		quicnet.WithKeyPath(*keyPath),
		quicnet.WithProtocolPrefix(*protocolPrefix),
		quicnet.WithDirectMessageHandler(func(ctx context.Context, from peer.ID, data []byte) {
			fmt.Printf("\n[DM] %s: %s\n> ", from, string(data))
		}),
	}

	if bootstrapPeers := splitCSV(*bootstrap); len(bootstrapPeers) > 0 {
		opts = append(opts, quicnet.WithBootstrapPeers(bootstrapPeers...))
	}

	client, err := quicnet.NewClient(opts...)
	if err != nil {
		log.Fatalf("create interactive client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("close client: %v", err)
		}
	}()

	currentTopic := strings.TrimSpace(*defaultTopic)
	if currentTopic == "" {
		currentTopic = "chat.global"
	}

	if err := subscribeTopic(client, currentTopic); err != nil {
		log.Fatalf("subscribe default topic %q: %v", currentTopic, err)
	}

	printBanner(client, currentTopic)

	if strings.TrimSpace(*relayAddr) != "" {
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		if err := client.ConnectToRelay(ctx, strings.TrimSpace(*relayAddr)); err != nil {
			cancel()
			log.Fatalf("connect to relay: %v", err)
		}
		cancel()
		fmt.Printf("[OK] Relay conectado: %s\n", strings.TrimSpace(*relayAddr))
	}

	lineCh := make(chan string)
	go readLines(lineCh)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	fmt.Print("> ")
	for {
		select {
		case <-signals:
			fmt.Println("\n[INFO] Cerrando nodo interactivo...")
			return
		case line, ok := <-lineCh:
			if !ok {
				fmt.Println("\n[INFO] Entrada cerrada.")
				return
			}

			nextTopic, shouldExit := handleCommand(client, currentTopic, line)
			if shouldExit {
				return
			}
			currentTopic = nextTopic
			fmt.Print("> ")
		}
	}
}

func handleCommand(client *quicnet.ClientNode, currentTopic, line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return currentTopic, false
	}

	parts := strings.Fields(trimmed)
	command := parts[0]

	switch command {
	case "/help":
		printHelp(currentTopic)
	case "/exit", "/quit":
		fmt.Println("[INFO] Saliendo...")
		return currentTopic, true
	case "/dial":
		if len(parts) < 2 {
			fmt.Println("Uso: /dial <multiaddr>")
			return currentTopic, false
		}
		addr := strings.TrimSpace(strings.TrimPrefix(trimmed, "/dial"))
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		err := client.ConnectToPeer(ctx, strings.TrimSpace(addr))
		cancel()
		if err != nil {
			fmt.Printf("[ERR] No se pudo conectar: %v\n", err)
			return currentTopic, false
		}
		fmt.Printf("[OK] Conectado a %s\n", strings.TrimSpace(addr))
	case "/relay":
		if len(parts) < 2 {
			fmt.Println("Uso: /relay <multiaddr-relay>")
			return currentTopic, false
		}
		addr := strings.TrimSpace(strings.TrimPrefix(trimmed, "/relay"))
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		err := client.ConnectToRelay(ctx, strings.TrimSpace(addr))
		cancel()
		if err != nil {
			fmt.Printf("[ERR] No se pudo reservar relay: %v\n", err)
			return currentTopic, false
		}
		fmt.Printf("[OK] Relay conectado: %s\n", strings.TrimSpace(addr))
	case "/pub":
		if len(parts) < 2 {
			fmt.Println("Uso: /pub <mensaje>")
			return currentTopic, false
		}
		message := strings.TrimSpace(strings.TrimPrefix(trimmed, "/pub"))
		if err := publish(client, currentTopic, message); err != nil {
			fmt.Printf("[ERR] No se pudo publicar: %v\n", err)
			return currentTopic, false
		}
		fmt.Printf("[OK] Publicado en %s\n", currentTopic)
	case "/sub":
		if len(parts) != 2 {
			fmt.Println("Uso: /sub <topic>")
			return currentTopic, false
		}
		topic := parts[1]
		if err := subscribeTopic(client, topic); err != nil {
			fmt.Printf("[ERR] No se pudo suscribir: %v\n", err)
			return currentTopic, false
		}
		fmt.Printf("[OK] Suscrito a %s\n", topic)
		return topic, false
	case "/provide":
		if len(parts) != 2 {
			fmt.Println("Uso: /provide <cid>")
			return currentTopic, false
		}
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		err := client.AnnounceData(ctx, parts[1])
		cancel()
		if err != nil {
			fmt.Printf("[ERR] No se pudo anunciar el CID: %v\n", err)
			return currentTopic, false
		}
		fmt.Printf("[OK] CID anunciado en la DHT: %s\n", parts[1])
	case "/unprovide":
		if len(parts) != 2 {
			fmt.Println("Uso: /unprovide <cid>")
			return currentTopic, false
		}
		if err := client.UnannounceData(parts[1]); err != nil {
			fmt.Printf("[ERR] No se pudo retirar el anuncio: %v\n", err)
			return currentTopic, false
		}
		fmt.Printf("[OK] Reanuncio detenido para %s\n", parts[1])
	case "/find":
		if len(parts) != 2 {
			fmt.Println("Uso: /find <cid>")
			return currentTopic, false
		}
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		providers, err := client.FindDataProviders(ctx, parts[1])
		cancel()
		if err != nil {
			fmt.Printf("[ERR] Error buscando providers: %v\n", err)
			return currentTopic, false
		}
		if len(providers) == 0 {
			fmt.Printf("[INFO] No se encontraron providers para %s\n", parts[1])
			return currentTopic, false
		}
		fmt.Printf("[OK] Providers para %s:\n", parts[1])
		for _, provider := range providers {
			fmt.Printf("  - %s %v\n", provider.ID, provider.Addrs)
		}
	case "/peers":
		peers := client.ConnectedPeers()
		if len(peers) == 0 {
			fmt.Println("[INFO] No hay peers conectados.")
			return currentTopic, false
		}
		fmt.Printf("[OK] Peers conectados (%d):\n", len(peers))
		for _, peerID := range peers {
			fmt.Printf("  - %s\n", peerID)
		}
	case "/ping":
		if len(parts) != 2 {
			fmt.Println("Uso: /ping <peer-id>")
			return currentTopic, false
		}
		peerID, err := peer.Decode(parts[1])
		if err != nil {
			fmt.Printf("[ERR] Peer ID invalido: %v\n", err)
			return currentTopic, false
		}
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		err = client.SendDirectMessage(ctx, peerID, []byte("ping"))
		cancel()
		if err != nil {
			fmt.Printf("[ERR] No se pudo enviar ping: %v\n", err)
			return currentTopic, false
		}
		fmt.Printf("[OK] Ping enviado a %s\n", peerID)
	case "/disconnect":
		if len(parts) != 2 {
			fmt.Println("Uso: /disconnect <peer-id>")
			return currentTopic, false
		}
		peerID, err := peer.Decode(parts[1])
		if err != nil {
			fmt.Printf("[ERR] Peer ID invalido: %v\n", err)
			return currentTopic, false
		}
		if err := client.DisconnectPeer(peerID); err != nil {
			fmt.Printf("[ERR] No se pudo desconectar: %v\n", err)
			return currentTopic, false
		}
		fmt.Printf("[OK] Peer desconectado: %s\n", peerID)
	default:
		if err := publish(client, currentTopic, trimmed); err != nil {
			fmt.Printf("[ERR] No se pudo publicar: %v\n", err)
			return currentTopic, false
		}
		fmt.Printf("[OK] Publicado en %s\n", currentTopic)
	}

	return currentTopic, false
}

func subscribeTopic(client *quicnet.ClientNode, topic string) error {
	return client.Subscribe(topic, func(msg []byte) {
		fmt.Printf("\n[GOSSIP %s] %s\n> ", topic, string(msg))
	})
}

func publish(client *quicnet.ClientNode, topic, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	return client.Publish(ctx, topic, []byte(message))
}

func readLines(lineCh chan<- string) {
	defer close(lineCh)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		lineCh <- scanner.Text()
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("\n[ERR] Error leyendo stdin: %v\n", err)
	}
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

func printBanner(client *quicnet.ClientNode, currentTopic string) {
	fmt.Println("Iniciando nodo P2P interactivo...")
	fmt.Printf("Peer ID: %s\n", client.ID())
	for _, addr := range client.Addrs() {
		fmt.Printf("Escuchando en: %s/p2p/%s\n", addr, client.ID())
	}
	fmt.Println(strings.Repeat("=", 62))
	printHelp(currentTopic)
	fmt.Println(strings.Repeat("=", 62))
}

func printHelp(currentTopic string) {
	fmt.Println("Comandos disponibles:")
	fmt.Println("  /dial <multiaddr>      -> Conectarse a otro peer")
	fmt.Println("  /relay <multiaddr>     -> Reservar slot en relay")
	fmt.Println("  /pub <mensaje>         -> Publicar en el topic actual")
	fmt.Println("  /sub <topic>           -> Suscribirse y cambiar topic actual")
	fmt.Println("  /provide <cid>         -> Anunciar un CID en Kademlia")
	fmt.Println("  /unprovide <cid>       -> Dejar de reanunciar un CID")
	fmt.Println("  /find <cid>            -> Buscar providers de un CID")
	fmt.Println("  /peers                 -> Ver peers conectados")
	fmt.Println("  /ping <peer-id>        -> Enviar mensaje directo 'ping'")
	fmt.Println("  /disconnect <peer-id>  -> Cerrar conexion con un peer")
	fmt.Println("  /help                  -> Mostrar esta ayuda")
	fmt.Println("  /exit                  -> Salir")
	fmt.Printf("  <texto libre>          -> Publicar en %s\n", currentTopic)
	fmt.Println("Nota: /provide y /find usan CIDs validos, no texto arbitrario.")
}
