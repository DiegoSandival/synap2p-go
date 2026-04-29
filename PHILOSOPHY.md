# Filosofía y Arquitectura de synap2p-go

`synap2p-go` está diseñado bajo un principio estricto de **Responsabilidad Única** (Single Responsibility Principle). No es una aplicación terminada, ni un daemon, ni un servidor web; es un **motor de red P2P puro e incrustable**.

La librería está construida para actuar como un "cerebro" de comunicaciones descentralizadas que funciona detrás de escena, pensado para ser consumido por interfaces gráficas (ej. Unity, React, Flutter) a través de intermediarios como WebSockets, C-FFI o puentes JNI.

## Principios Fundamentales

### 1. "Bytes in, Bytes out" (Agnóstico al Medio)
El motor no sabe, ni le interesa, cómo te comunicas con él. No incluye servidores HTTP, WebSockets ni bindings para C. 
Toda la interacción con el motor se realiza mediante **arreglos de bytes (`[]byte`)** estructurados según un protocolo binario minimalista y predefinido. 
- Tú le entregas un bloque de memoria (Request).
- Él te devuelve un bloque de memoria (Response o Eventos).

### 2. Operaciones 100% Asíncronas y No Bloqueantes
El enrutamiento de la librería está diseñado para jamás congelar el hilo que lo invoca.
Cuando se llama a `engine.Process(msg)`, el buffer binario es copiado y delegado a hilos secundarios (Goroutines) en nanosegundos. Esto asegura que si estás construyendo un wrapper de WebSocket o una librería nativa (`.dll` / `.so`), el canal de lectura principal siempre estará libre de atascos; incluso si una operación de red como descubrir peers (`FindProviders`) toma 15 segundos en completarse.

### 3. Dirigido por Eventos (Event-Driven)
Ante la naturaleza no bloqueante del entorno, las respuestas de toda operación (ya sean datos, mensajes directos de usuarios, acuses de recibo o notificaciones de la red) se emiten a un **Emisor Universal (`EventHandler`)**.
El consumidor externo (tu daemon o aplicación principal) inyecta este handler y centraliza todos los eventos asíncronos en una sola función (un *"tubo de bajada"*).

### 4. Trazabilidad Estricta (Preservación del Origin ID)
Al ser asíncrono, si envías tres solicitudes diferentes, las respuestas podrían llegar en desorden. Para solucionar esto y mantener sincronía con la interfaz gráfica, el protocolo usa una arquitectura de **IDs de 16 bytes**.
El motor promete **preservar rigurosamente el ID original** suministrado por la capa superior y devolverlo dentro de la respuesta correspondiente, permitiendo al cliente correlacionar peticiones a la perfección.

### 5. Silencio por Defecto (Inversión de Control de Logs)
Una librería backend no debe imprimir basura en la consola (stdout/stderr). `synap2p-go` es completamente silencioso. Si necesitas depuración profunda, se expone la opción `WithLogger(func(level, msg))`. Esto delega el control de la telemetría al desarrollador, permitiéndote registrarlo en disco, imprimirlo en la terminal, o reenviarlo como logs remotos hacia tu interfaz de usuario.

---

## Ciclo de Uso Ideal (Ejemplo con WebSocket)

La anatomía de consumo recomendada consta de 3 simples pasos:

1. **Inicialización**: Crear el `Engine` configurando los *callbacks* que hablarán con tu socket de bajada.
2. **Puente (Bridge)**: Escuchar un socket, tomar sus frames binarios crudos, e inyectarlos con `engine.Process(bytes)`.
3. **Manejo de Respuestas**: Cuando la red responde o recibe mensajes P2P, el motor ejecuta el `EventHandler` y tú utilizas la conexión websocket (`socket.WriteMessage(bytes)`) para pasar esos eventos directo al frontend de manera asíncrona.

Esta separación absoluta te permite reutilizar `synap2p-go` como binario precompilado en videojuegos, empacarlo como Daemon para escritorios, o correrlo en servidores cloud, asegurándote que la base del P2P permanezca inmutable y aislada.