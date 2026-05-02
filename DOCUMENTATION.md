# Documentación Rápida: synap2p-go

`synap2p-go` es un motor de red P2P incrustable, 100% asíncrono y diseñado para ser manejado puramente mediante arreglos de bytes (`[]byte`).

## Uso Básico (El Ciclo de Vida)

El motor funciona bajo un esquema muy simple: **Instanciar -> Inyectar Bytes -> Escuchar Eventos**.

```go
import quicnet "github.com/DiegoSandival/synap2p-go"

// 1. Inicializar el Engine y su puente asíncrono (Event Handler)
engine, err := quicnet.NewEngine(
    quicnet.WithEventHandler(func(eventBytes []byte) {
        // Aquí recibes todas las respuestas, errores y eventos asíncronos de la red libremente.
        // Envía este buffer hacia tu GUI (ej. a través de un WebSocket).
    }),
)

// 2. Procesar buffers binarios de entrada (No bloqueante)
// Reemplaza "msg" con los bytes que llegan desde tu interfaz.
engine.Process(msg)

// 3. Limpieza al salir
engine.Close()
```

### Constructor Unificado con Config

También puedes crear un nodo completo desde una estructura serializable a JSON o YAML.

```go
import (
    "context"

    quicnet "github.com/DiegoSandival/synap2p-go"
)

cfg := quicnet.Config{
    Mode:          quicnet.ModeClient,
    ListenPort:    0,
    BootstrapList: []string{"/ip4/127.0.0.1/udp/4001/quic-v1/p2p/12D3KooW..."},
    Namespace:     "_mi_red_relays",
    DataDir:       "./data",
}

node, err := quicnet.NewNode(cfg)
if err != nil {
    panic(err)
}
defer node.Close()

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

if err := node.Start(ctx); err != nil {
    panic(err)
}

go func() {
    for event := range node.Events() {
        _ = event
    }
}()

requestID := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
if err := node.SendOpcode(0x0D, requestID, []byte("payload")); err != nil {
    panic(err)
}
```

Si prefieres archivo, puedes cargarlo con `LoadConfig("config.yaml")` o `LoadConfig("config.json")` y luego pasarlo a `NewNode`.

## Topología Recomendada

La red queda separada en dos perfiles:

* **Servidores Relay**: Son nodos públicos que levantan `Relay v2`, participan en la DHT como backbone y se anuncian periódicamente en el namespace `_mi_red_relays`.
* **Clientes Locales**: Son nodos ligeros que arrancan con `BootstrapPeers`, usan la DHT en modo cliente y delegan la selección y reconexión de relays al `AutoRelay` nativo de libp2p.

En esta arquitectura, la biblioteca no programa un worker manual para failover de relays en el cliente. La recuperación de relays queda en manos de libp2p a través de `AutoRelay`.

### Bootstrap Peers

Para integrarse a la red, tanto el cliente como los relays pueden recibir peers semilla con `WithBootstrapPeers(...)`. En la práctica, el cliente necesita conocer al menos un relay o nodo backbone para que la DHT y el descubrimiento de relays funcionen correctamente.

### Namespace de Relays

El namespace de descubrimiento de relays se configura con `WithRelayDiscoveryNamespace(...)` y por defecto usa `_mi_red_relays`. Los relays dedicados también pueden ajustar la frecuencia de anuncio con `WithRelayAdvertiseInterval(...)`.

## Formato del Paquete (Buffer Binario)

Todos los mensajes (Tanto los que envías a `Process()` como los que recibes en el `EventHandler`) comparten una estructura idéntica:

`[Opcode: 4 bytes] + [ID de Origen: 16 bytes] + [Carga Útil / Payload: N bytes]`

* **Opcode**: Indica la acción a ejecutar o el tipo de evento recibido.
* **ID (16 bytes)**: Un identificador que tú asignas en la interfaz gráfica. El motor **siempre te devolverá este mismo ID exacto** en la respuesta, para que sepas a qué solicitud original le corresponde.

---

## Reglas Especiales de Payload

### ⚠️ Publicar en un Tópico (Opcode 0x05 - PUB)
Dado que el protocolo binario empaqueta el contenido dinámicamente, al enviar un comando de publicación en PubSub, **debes concatenar el nombre del Tópico y el Mensaje usando el carácter Pipe (`|`)** en la carga útil.

**Ejemplo de Payload para PUB:**
`nombre-del-topico|Hola, esta es mi data binaria o texto`

El motor de Go cortará el mensaje exactamente en el primer `|` que encuentre; usará la primera parte para ruteo de red, y enviará la segunda parte como el mensaje real.

### Comandos que devuelven Datos (SuccessDataResponse)
La interfaz responderá con `Status: 1` pero la Carga Útil (a partir del byte 21) contendrá información en los siguientes Opcodes:
* `GENERATE_CID (0x0D)`: Retorna un string representando el Hash (CID v1).
* `PEERS (0x06)` y `FIND_PROVIDERS (0x0A)`: Retornan una lista de `PeerIDs` recuperados, separados por comas.

## Opcodes Soportados
*Consulte `PROTOCOL_FORMAT.txt` para las descripciones binarias completas.*

**Operaciones (Entrada hacia Go):**
* `0x01` RELAY | `0x02` DIAL | `0x03` SUB | `0x04` USE
* `0x05` PUB *(Recuerda la regla del `|`)*
* `0x06` PEERS | `0x07` UNSUB | `0x08` ANNOUNCE
* `0x09` UNANNOUNCE | `0x0A` FIND_PROVIDERS
* `0x0B` DIRECT_MSG | `0x0C` DISCONNECT
* `0x0D` GENERATE_CID

**Eventos Asíncronos de Red (Salida desde Go):**
* `0x0E` EVENT_PUBSUB_MSG (Mensajes que llegan de canales suscritos)
* `0x0F` EVENT_DIRECT_MSG (Mensajes directos desde otros Peers)
* `0x10` EVENT_PEER_DISCOVERED (Notificación cuando libp2p detecta a alguien localmente)