package quicnet

import (
	"github.com/DiegoSandival/synap2p-go/protocol"
)

// ProcessRequest es la función principal que el servidor llamará.
func ProcessRequest(msg []byte, parser *protocol.ProtocolParser, handler *CentralHandler) {
	// Copiar el buffer. Si Unity/C# libera el puntero al retornar de esta función síncrona,
	// el slice "msg" apuntará a memoria muerta. La copia nos asegura supervivencia en Go.
	payload := make([]byte, len(msg))
	copy(payload, msg)

	// Ahora el mensaje mínimo son 20 bytes: Opcode (4) + ID (16)
	if len(payload) < 20 {
		// Retornar error con ID todo ceros ya que no sabemos el ID
		handler.Client.EmitEvent(parser.ErrorResponse(0xFF, make([]byte, 16)))
		return
	}

	// Ejecutar la operación de forma asíncrona para liberar inmediatamente el hilo base (FFI Thread)
	go func(req []byte) {
		var response []byte

		// Enrutar según el opcode
		switch parser.Opcode(req) {
		case 0x01:
			response = handler.Relay(parser, req)
		case 0x02:
			response = handler.Dial(parser, req)
		case 0x03:
			response = handler.Sub(parser, req)
		case 0x04:
			response = handler.Use(parser, req)
		case 0x05:
			response = handler.Pub(parser, req)
		case 0x06:
			response = handler.Peers(parser, req)
		case 0x07:
			response = handler.Unsub(parser, req)
		case 0x08:
			response = handler.Announce(parser, req)
		case 0x09:
			response = handler.Unannounce(parser, req)
		case 0x0A:
			response = handler.FindProviders(parser, req)
		case 0x0B:
			response = handler.DirectMsg(parser, req)
		case 0x0C:
			response = handler.Disconnect(parser, req)
		case 0x0D:
			response = handler.GenerateCID(parser, req)
		default:
			// Opcode no soportado
			response = parser.ErrorResponse(0xFF, req[4:20])
		}

		if response != nil {
			handler.Client.EmitEvent(response)
		}
	}(payload)
}
