# Tabla de Opcodes del Protocolo WebSocket

Esta tabla resume todos los opcodes definidos en el protocolo WebSocket binario, incluyendo su tipo, función, comando de ejemplo y estructura de parámetros.

## Requests

| Opcode | Función | Descripción | Parámetros | Estructura Binaria |
|--------|---------|-------------|------------|-------------------|
| `0x01` | Relay | Configura un peer relay | multiaddr | `u16 len` + `addr[]` |
| `0x02` | Dial | Conecta con otro peer | multiaddr del peer | `u16 len` + `addr[]` |
| `0x03` | Sub | Se suscribe a un topic | nombre del topic | `u16 len` + `topic[]` |
| `0x04` | Use | Selecciona topic activo | nombre del topic | `u16 len` + `topic[]` |
| `0x05` | Pub | Publica mensaje | datos | `u32 len` + `data[]` |
| `0x06` | Peers | Solicita lista de peers | (ninguno) | solo opcode |
| `0x07` | Unsub | Desuscribe de un topic | nombre del topic | `u16 len` + `topic[]` |
| `0x08` | Announce | Anuncia datos en DHT | CID | `u16 len` + `cid[]` |
| `0x09` | Unannounce | Desanuncia datos de DHT | CID | `u16 len` + `cid[]` |
| `0x0A` | FindProviders | Busca proveedores de datos | CID | `u16 len` + `cid[]` |
| `0x0B` | DirectMsg | Envía mensaje directo | peerID + datos | `u16 len` + `peerID[]` + `u32 len` + `data[]` |
| `0x0C` | Disconnect | Desconecta de un peer | peerID | `u16 len` + `peerID[]` |
| `0x0D` | GenerateCID | Genera un CID a partir de los datos proporcionados | datos binarios | `u32 len` + `data[]` |

## Responses

| Opcode | Función | Descripción | Parámetros | Estructura Binaria |
|--------|---------|-------------|------------|-------------------|
| `0x00` | OK | Confirmación exitosa | request opcode | `1 byte` (opcode del request) |
| `0x01` | Error | Indica un error | request opcode + error | `request opcode` + `u16 len` + `error[]` |
| `0x00 0x7F` | Hello | Saludos iniciales | peerID + addrs | `u16 len` + `peerID[]` + `u16 count` + múltiples `[u16 len + addr[]]` |
| `0x00 0x06` | Peers List | Lista de peers | conexiones activas | `u16 count` + múltiples `[u16 len + peerID[]]` |
| `0x00 0x0A` | Providers List | Lista de proveedores | proveedores encontrados | `u16 count` + múltiples `[u16 len + peerID[] + u16 len + addr[]]` |
| `0x00 0x0D` | CID Generated | Respuesta con el CID generado | CID | `u16 len` + `cid[]` |

## Eventos Asincronos

| Opcode | Función | Descripción | Parámetros | Estructura Binaria |
|--------|---------|-------------|------------|-------------------|
| `0x80 0x00` | Gossip | Mensaje recibido en topic suscrito | topic + payload | `u16 len` + `topic[]` + `u32 len` + `payload[]` |
| `0x81 0x00` | Direct | Mensaje directo recibido | peerID + payload | `u16 len` + `peerID[]` + `u32 len` + `payload[]` |

## Notas de Codificación

- **u16 len**: Longitud en bytes como entero sin signo de 16 bits (big-endian)
- **u32 len**: Longitud en bytes como entero sin signo de 32 bits (big-endian)
- **addr[]**: Multiaddr binaria del peer
- **topic[]**: Nombre del topic en UTF-8
- **peerID[]**: ID del peer en bytes
- **data[]**: Payload del mensaje (datos binarios arbitrarios)
- **cid[]**: CID en formato binario (por ejemplo, Base58 o bytes crudos)
- **error[]**: Descripción del error en UTF-8