package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"

	libp2p "github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	quicnet "github.com/DiegoSandival/synap2p-go"
	"github.com/DiegoSandival/synap2p-go/protocol"
)

func main() {
	fmt.Println("🚀 Iniciando ejemplo de Router P2P...")
	ctx := context.Background()

	// 1. Inicializar Host de Libp2p
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		log.Fatalf("Error creando host: %v", err)
	}
	defer h.Close()
	fmt.Printf("✅ Host creado con ID: %s\n", h.ID().String())

	// 2. Inicializar DHT
	kdht, err := dht.New(ctx, h)
	if err != nil {
		log.Fatalf("Error creando DHT: %v", err)
	}
	defer kdht.Close()
	fmt.Println("✅ IPFS DHT instanciado")

	// 3. Inicializar PubSub
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		log.Fatalf("Error creando PubSub: %v", err)
	}
	fmt.Println("✅ GossipSub instanciado")

	// 4. Inyectar dependencias al CentralHandler
	handler := quicnet.NewCentralHandler(h, kdht, ps)
	parser := &protocol.ProtocolParser{}

	// 5. Simular un mensaje entrante (Ejemplo: Opcode 0x03 -> Sub)
	// Formato esperado de ParseU16LenPayload: [Opcode: 4] + [Len: 2] + [TopicBytes: Len]
	topic := "my-awesome-topic"
	topicBytes := []byte(topic)
	topicLen := uint16(len(topicBytes))

	// Preparamos el buffer del mensaje
	msg := make([]byte, 4+2+int(topicLen))

	// Byte 3 es el opcode (como espera parser.Opcode())
	msg[3] = 0x03 // Opcode 0x03 = Sub

	// Bytes 4 y 5 son el Len (U16 BigEndian)
	binary.BigEndian.PutUint16(msg[4:6], topicLen)

	// Bytes 6 en adelante es la carga (el string del topic)
	copy(msg[6:], topicBytes)

	fmt.Printf("\n📦 Mensaje crudo construido: %v\n", msg)
	fmt.Println("➡️  Enviando mensaje hacia el ProcessRequest...")

	// 6. Procesar el Request a través de nuestro ruteador
	response := quicnet.ProcessRequest(msg, parser, handler)

	// Validar respuesta
	// El formato de SuccessResponse que usamos es: [0x00, 0x00, 0x00, opcode, 0x01]
	fmt.Printf("⬅️  Respuesta recibida: %v\n", response)
	if len(response) == 5 && response[4] == 0x01 {
		fmt.Println("🎉 ¡Éxito! El Dispatcher reconoció el opcode y llamó a la función Sub del CentralHandler.")
	} else {
		fmt.Println("❌ Mmm, parece que la petición falló o devolvió un formato inesperado.")
	}
}
