package quicnet

import (
	"github.com/DiegoSandival/synap2p-go/protocol"
)

// ProcessRequest es la función principal que el servidor llamará.
func ProcessRequest(msg []byte, parser *protocol.ProtocolParser, handler *CentralHandler) []byte {
	// Validar que el mensaje tenga al menos el tamaño del opcode (4 bytes según diseño: [0x00, 0x00, 0x00, Opcode])
	if len(msg) < 4 {
		return []byte("error: mensaje muy corto")
	}

	// Enrutar según el opcode
	switch parser.Opcode(msg) {
	case 0x01:
		return handler.Relay(parser, msg)
	case 0x02:
		return handler.Dial(parser, msg)
	case 0x03:
		return handler.Sub(parser, msg)
	case 0x04:
		return handler.Use(parser, msg)
	case 0x05:
		return handler.Pub(parser, msg)
	case 0x06:
		return handler.Peers(parser, msg)
	case 0x07:
		return handler.Unsub(parser, msg)
	case 0x08:
		return handler.Announce(parser, msg)
	case 0x09:
		return handler.Unannounce(parser, msg)
	case 0x0A:
		return handler.FindProviders(parser, msg)
	case 0x0B:
		return handler.DirectMsg(parser, msg)
	case 0x0C:
		return handler.Disconnect(parser, msg)
	case 0x0D:
		return handler.GenerateCID(parser, msg)
	default:
		// Opcode no soportado
		return []byte("error: opcode desconocido")
	}
}
