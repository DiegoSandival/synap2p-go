package main

import (
	"fmt"
	"log"
	"time"

	libp2p "github.com/libp2p/go-libp2p"

	quicnet "github.com/DiegoSandival/synap2p-go"
)

func main() {
	fmt.Println("🚀 Iniciando ejemplo de Router P2P simplificado con ID de 16 bytes...")

	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		log.Fatalf("Error creando host: %v", err)
	}
	defer h.Close()

	engine, err := quicnet.NewEngine(
		quicnet.WithEventHandler(func(event []byte) {
			if len(event) >= 4 {
				opcode := event[3]
				id := event[4:20]
				fmt.Printf("📬 [EVENTO ASÍNCRONO] Recibido Opcode: 0x%02X, Origin ID: %v\n", opcode, id)
			}
		}),
	)
	if err != nil {
		log.Fatalf("Error creando engine: %v", err)
	}
	defer engine.Close()

	// 5. Construimos el mensaje de Sub (0x03)
	// Formato ideal = [Opcode: 4] + [ID: 16] + [Topic rest of bytes]
	topic := "my-awesome-topic"
	topicBytes := []byte(topic)

	msg := make([]byte, 20+len(topicBytes))
	msg[3] = 0x03 // Opcode 0x03 = Sub

	// Simulando un ID de 16 bytes: 010203...
	id := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	copy(msg[4:20], id)

	// El resto es la carga
	copy(msg[20:], topicBytes)

	fmt.Printf("\n📦 Mensaje (Opcode: %d, ID: %v, Tema: %s)\n", msg[3], id, topic)

	engine.Process(msg)

	// Esperamos un segundo para darle tiempo a la rutina asíncrona de regresar el evento a la consola.
	// En Unity/Android, el EventHandler estará siempre activo recibiendo el callback.
	fmt.Println("⏳ Solicitud lanzada, esperando respuesta asíncrona en hilo principal...")
	time.Sleep(1 * time.Second)
	fmt.Println("🚀 Fin de la simulación")
}
