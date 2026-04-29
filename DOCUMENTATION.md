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