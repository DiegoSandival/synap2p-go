# Examples

Los binarios en esta carpeta sirven para pruebas manuales rapidas del backbone WAN y de los clientes hibridos.

Tambien hay un cliente interactivo pensado como equivalente en Go al ejemplo de consola que antes tenias en Rust.

## Requisitos

- Ejecuta los comandos desde la raiz del modulo.
- Usa rutas de clave distintas por proceso para no reutilizar la misma identidad.
- Si trabajas sobre `\\wsl.localhost`, `go build` puede fallar por bloqueo de `go.mod`; en ese caso ejecuta los comandos desde una ruta local o dentro de una distro con Go instalado.

## 1. Levantar el servidor

```bash
go run ./examples/server \
  -key ./tmp/server.key
```

Anota una de las lineas `listening on:`. El valor completo, incluyendo `/p2p/<peer-id>`, sera la direccion del servidor para `-bootstrap` y `-relay`.

Ejemplo:

```text
/ip4/203.0.113.10/udp/4001/quic-v1/p2p/12D3KooWServerExample
```

## 2. Conectar el primer cliente

```bash
go run ./examples/client \
  -key ./tmp/client-a.key \
  -bootstrap "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/12D3KooWServerExample" \
  -relay "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/12D3KooWServerExample" \
  -topic chat.global
```

Este cliente queda suscrito a `chat.global` esperando mensajes.

## 3. Publicar desde un segundo cliente

```bash
go run ./examples/client \
  -key ./tmp/client-b.key \
  -bootstrap "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/12D3KooWServerExample" \
  -relay "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/12D3KooWServerExample" \
  -topic chat.global \
  -message "hola desde client-b"
```

El mensaje debe aparecer en la consola del primer cliente. El ejemplo de cliente publica con una condicion de readiness que espera al menos un peer remoto en el topico, asi que este orden importa: primero suscribe `client-a`, luego publica `client-b`.

## 4. Probar mensaje directo entre clientes

Primero copia una direccion `listening on:` del `client-a` y su `peer id`.

Luego ejecuta otro cliente conectando explicitamente a esa direccion y enviando un payload directo:

```bash
go run ./examples/client \
  -key ./tmp/client-c.key \
  -bootstrap "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/12D3KooWServerExample" \
  -relay "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/12D3KooWServerExample" \
  -peer "/ip4/192.168.1.20/udp/51234/quic-v1/p2p/12D3KooWClientAExample" \
  -direct-peer "12D3KooWClientAExample" \
  -direct-message "ping directo"
```

El `client-a` debe imprimir `direct message from ...: ping directo`.

## Flags utiles

- `examples/server`: `-key`, `-listen`, `-bootstrap`, `-protocol`
- `examples/client`: `-key`, `-bootstrap`, `-relay`, `-peer`, `-topic`, `-message`, `-direct-peer`, `-direct-message`, `-protocol`
- `examples/interactive`: `-key`, `-bootstrap`, `-relay`, `-topic`, `-protocol`
- `examples/websocket`: `-key`, `-listen`, `-path`, `-bootstrap`, `-protocol`

## Cliente interactivo

Arranque basico:

```bash
go run ./examples/interactive \
  -key ./tmp/interactive-a.key \
  -bootstrap "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/12D3KooWServerExample" \
  -relay "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/12D3KooWServerExample" \
  -topic chat.global
```

`-topic` ahora solo define el topic actual sugerido; no suscribe automaticamente. Para publicar, primero usa `/sub <topic>` y luego `/use <topic>` si todavia no hay topic actual activo.

Una vez levantado, acepta estos comandos:

- `/dial <multiaddr>` para conectar manualmente a un peer.
- `/relay <multiaddr>` para reservar un slot en un relay.
- `/pub <mensaje>` o texto libre para publicar en el topic actual.
- `/sub <topic>` para suscribirte a un topic.
- `/unsub <topic>` para desuscribirte de un topic.
- `/topics` para listar todos los topics suscritos.
- `/use <topic>` para elegir el topic actual de publicacion.
- `/provide <cid>` y `/find <cid>` para anunciar y buscar providers en la DHT.
- `/peers`, `/ping <peer-id>` y `/disconnect <peer-id>` para inspeccion y mensajes directos.

Flujo manual recomendado de pruebas: `/relay`, `/dial`, `/sub`, `/use`, `/pub`.

Importante: a diferencia de tu ejemplo de Rust, aqui la capa DHT expone CIDs como identificador de contenido. `provide` y `find` no aceptan texto arbitrario; usa un CID valido.

## Cliente controlado por WebSocket

Arranque basico:

```bash
go run ./examples/websocket \
  -key ./tmp/ws-a.key \
  -listen 127.0.0.1:8080 \
  -path /ws
```

Este ejemplo abre un servidor WebSocket local y crea un `ClientNode` por cada conexion WebSocket entrante.

- Solo acepta frames binarios.
- El protocolo usa `opcode + longitudes + payloads` en big-endian.
- Comandos soportados en esta primera version: `relay`, `dial`, `sub`, `use`, `pub`, `peers`.
- Al conectar, el servidor envia un frame inicial con el `peer id` local y las direcciones de escucha.

La especificacion completa del frame binario esta en `examples/websocket/PROTOCOL.md`.