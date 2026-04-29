package quicnet

import (
	"github.com/DiegoSandival/synap2p-go/protocol"
)

func (h *CentralHandler) Sub(parser *protocol.ProtocolParser, payload []byte) []byte {
	_, err := parser.ParseU16LenPayload(payload) // Topic
	if err != nil {
		return parser.ErrorResponse(0x03)
	}
	// Lógica: h.PubSub.Join(topic) y suscribir
	return parser.SuccessResponse(0x03)
}

func (h *CentralHandler) Use(parser *protocol.ProtocolParser, payload []byte) []byte {
	_, err := parser.ParseU16LenPayload(payload) // Topic
	if err != nil {
		return parser.ErrorResponse(0x04)
	}
	// Lógica: Setear topic activo en tu cliente
	return parser.SuccessResponse(0x04)
}

func (h *CentralHandler) Pub(parser *protocol.ProtocolParser, payload []byte) []byte {
	_, err := parser.ParseU32LenPayload(payload) // Data
	if err != nil {
		return parser.ErrorResponse(0x05)
	}
	// Lógica: h.PubSub... topic.Publish()
	return parser.SuccessResponse(0x05)
}

func (h *CentralHandler) Unsub(parser *protocol.ProtocolParser, payload []byte) []byte {
	_, err := parser.ParseU16LenPayload(payload) // Topic
	if err != nil {
		return parser.ErrorResponse(0x07)
	}
	// Lógica: Cancelar suscripción local del topic
	return parser.SuccessResponse(0x07)
}
