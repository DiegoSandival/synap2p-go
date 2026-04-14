# WebSocket Binary Protocol

Este ejemplo expone un servidor WebSocket local y solo acepta frames binarios.

Endpoint por defecto:

```text
ws://127.0.0.1:8080/ws
```

## Estructura general

Cada request empieza con 1 byte de opcode.

```text
+--------+-------------------+
| opcode | payload especifico|
+--------+-------------------+
```

Los campos variables se codifican como:

- `u16 + bytes` para strings cortos.
- `u32 + bytes` para payloads de mensaje.
- enteros en big-endian.

## Requests

### `0x01` relay

```text
+------+------------------+
| 0x01 | u16 len | addr[] |
+------+------------------+
```

`addr` es la multiaddr completa del relay.

### `0x02` dial

```text
+------+------------------+
| 0x02 | u16 len | addr[] |
+------+------------------+
```

`addr` es la multiaddr completa del peer.

### `0x03` sub

```text
+------+-------------------+
| 0x03 | u16 len | topic[] |
+------+-------------------+
```

### `0x04` use

```text
+------+-------------------+
| 0x04 | u16 len | topic[] |
+------+-------------------+
```

El topic debe estar previamente suscrito.

### `0x05` pub

```text
+------+---------------------+
| 0x05 | u32 len | data[]    |
+------+---------------------+
```

Publica en el topic actual.

### `0x06` peers

```text
+------+
| 0x06 |
+------+
```

Sin payload.

## Responses

### OK generico

```text
+------+----------------+
| 0x00 | request opcode |
+------+----------------+
```

Se usa para `relay`, `dial`, `sub`, `use` y `pub`.

### Error

```text
+------+----------------+---------------------+
| 0x01 | request opcode | u16 len | error[]  |
+------+----------------+---------------------+
```

### Hello inicial de sesion

Se envia automaticamente al abrir la conexion WebSocket.

```text
+------+-------+--------------------+---------------------------+
| 0x00 | 0x7F  | u16 len peerID[]   | u16 count | addr entries[] |
+------+-------+--------------------+---------------------------+
```

Cada `addr entry` es `u16 len | addr[]`.

### Respuesta de peers

```text
+------+-------+------------------------------+
| 0x00 | 0x06  | u16 count | peer entries[]   |
+------+-------+------------------------------+
```

Cada `peer entry` es `u16 len | peerID[]`.

## Eventos asincronos

### Gossip recibido

```text
+------+-------+-------------------+----------------------+
| 0x80 | 0x00  | u16 len topic[]   | u32 len payload[]   |
+------+-------+-------------------+----------------------+
```

Se emite cuando llega un mensaje en un topic suscrito mediante `sub`.