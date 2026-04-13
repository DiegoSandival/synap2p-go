# synap2p-go

Libreria Go para una red P2P basada en libp2p con estas decisiones de arquitectura:

- Transporte exclusivamente QUIC.
- `ServerNode` para backbone WAN con Circuit Relay v2, AutoNAT y DHT en modo servidor.
- `ClientNode` para clientes hibridos LAN + WAN con mDNS, DHT en modo cliente, GossipSub y DCUtR.
- Identidad persistente con clave Ed25519 almacenada en disco.

## Estado actual

Esta primera implementacion ya incluye:

- API publica en la raiz del modulo con opciones funcionales.
- `NewServer(...)` y `NewClient(...)`.
- `ClientNode.Subscribe`, `Unsubscribe`, `Publish`, `AnnounceData`, `UnannounceData`, `FindDataProviders`, `ConnectToRelay`, `ConnectToPeer` y `SendDirectMessage`.
- mDNS para descubrimiento local y conexion automatica a peers descubiertos.
- Integracion de GossipSub sobre GossipSub router.
- Reanuncio periodico de providers en DHT mientras un CID permanezca anunciado localmente.

## Ejemplo rapido

```go
package main

import (
	"context"
	"log"
	"time"

	quicnet "github.com/DiegoSandival/synap2p-go"
)

func main() {
	server, err := quicnet.NewServer(
		quicnet.WithKeyPath("./server.key"),
		quicnet.WithListenAddrs("/ip4/0.0.0.0/udp/4001/quic-v1"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()

	log.Println("server peer id:", server.ID())
	log.Println("server addrs:", server.Addrs())

	select {}
}
```

```go
package main

import (
	"context"
	"log"
	"time"

	quicnet "github.com/DiegoSandival/synap2p-go"
	"github.com/libp2p/go-libp2p/core/peer"
)

func main() {
	client, err := quicnet.NewClient(
		quicnet.WithKeyPath("./client.key"),
		quicnet.WithDirectMessageHandler(func(ctx context.Context, from peer.ID, data []byte) {
			log.Printf("direct message from %s: %s", from, string(data))
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.ConnectToRelay(ctx, "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/12D3KooW..."); err != nil {
		log.Fatal(err)
	}

	if err := client.Subscribe("chat.global", func(msg []byte) {
		log.Printf("topic message: %s", string(msg))
	}); err != nil {
		log.Fatal(err)
	}

	if err := client.Publish(ctx, "chat.global", []byte("hola")); err != nil {
		log.Fatal(err)
	}

	select {}
}
```

## Examples

El repositorio incluye binarios listos para pruebas manuales:

- `go run ./examples/server`
- `go run ./examples/client`
- `go run ./examples/interactive`

La guia completa de uso y el flujo de prueba minimo estan en `examples/README.md`.

### Arranque rapido del servidor

```bash
go run ./examples/server \
	-key ./tmp/server.key
```

El proceso imprime el `peer id` y las direcciones completas que puedes reutilizar como `-bootstrap` y `-relay` desde los clientes.

### Arranque rapido del cliente

```bash
go run ./examples/client \
	-key ./tmp/client-a.key \
	-bootstrap "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/12D3KooW..." \
	-relay "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/12D3KooW..." \
	-topic chat.global
```

Para publicar en GossipSub necesitas al menos otro peer suscrito al mismo topico; por eso el flujo recomendado usa dos clientes.

## Notas de implementacion

- Los metodos de red mas sensibles aceptan `context.Context` para timeout y cancelacion.
- `UnannounceData` es `best effort` local: deja de reanunciar y permite que los registros de provider expiren en la DHT.
- El cliente prioriza direcciones descubiertas por mDNS conectando directamente a peers LAN tan pronto como aparecen.
- La validacion de compilacion y pruebas se ejecuto en una copia temporal local de Windows porque `go` no puede bloquear `go.mod` correctamente sobre la ruta `\\wsl.localhost` del workspace.
