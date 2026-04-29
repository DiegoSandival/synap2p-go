package main

import (
	"fmt"
	"log"
	"time"

	quicnet "github.com/DiegoSandival/synap2p-go"
)

func main() {
	fmt.Println("🔵 Iniciando Daemon P2P (Simulación de puente WebSocket)")

	// 1. Inicializamos el motor Synap2P inyectando los manejadores de eventos y logs.
	engine, err := quicnet.NewEngine(
		// WithEventHandler simula el envío del frame binario hacia el cliente conectado por WebSocket.
		quicnet.WithEventHandler(func(event []byte) {
			if len(event) >= 20 {
				opcode := event[3]
				id := event[4:20]
				statusOrData := event[20:]

				fmt.Printf("   [WebSocket <- Motor P2P] Emitiendo al cliente gráfico | Opcode: 0x%02X, ID: %v, PayloadLen: %d bytes\n", opcode, id, len(statusOrData))

				// Opcionalmente, imprimir el payload si es texto legible (e.g. CID, Peers)
				if opcode == 0x0D || opcode == 0x06 {
					if len(statusOrData) > 1 {
						// Ignoramos el byte de status (0x01) que está en index 20, e imprimimos desde el 21 usando la lógica de SuccessDataResponse
						// Corrección: SuccessDataResponse pone success en 20 y la data en 21 en adelante
						fmt.Printf("      -> Payload de Datos: %s\n", string(event[21:]))
					}
				}
			}
		}),
		// WithLogger canaliza la telemetría interna hacia el sistema de logs que desee el Daemon.
		quicnet.WithLogger(func(level, msg string) {
			fmt.Printf("   [P2P Log] %s: %s\n", level, msg)
		}),
	)

	if err != nil {
		log.Fatalf("Error creando el motor P2P: %v", err)
	}
	defer engine.Close()

	fmt.Println("🟢 Motor P2P listo y a la espera de mensajes binarios...")

	// 2. Simulamos la recepción de un mensaje binario asíncrono desde la Interfaz Gráfica de Usuario
	//    a través de la conexión de un WebSocket.
	fmt.Println("\n🟡 [GUI -> WebSocket] Enviando solicitud GenerateCID (Opcode: 0x0D)")

	// Vamos a crear la solicitud de Generate CID
	dataStr := "Mi string para hashear en IPFS"
	dataBytes := []byte(dataStr)

	msg := make([]byte, 20+len(dataBytes))
	msg[3] = 0x0D // Opcode de GENERATE_CID

	// Generamos nuestro UUID / ID del Request origen desde el "GUI" (16 Bytes)
	requestID := []byte{0, 1, 2, 3, 4, 5, 255, 255, 0, 0, 0, 0, 0, 0, 0, 99}
	copy(msg[4:20], requestID)
	// Pasamos el contenido
	copy(msg[20:], dataBytes)

	// 3. Ocurre la inyección al Motor P2P
	//    Esto es asíncrono y no bloqueante. Regresa inmediatamente liberando al WebSocket listener!
	engine.Process(msg)

	// 4. Simulamos funcionamiento de la aplicación en el mundo real...
	//    Dormimos el hilo esperando que el sistema se comunique en backend y responda por los eventos.
	time.Sleep(2 * time.Second)

	fmt.Println("\n🔴 Apagando nodo Daemon P2P...")
}
