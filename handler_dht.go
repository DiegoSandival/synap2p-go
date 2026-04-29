package quicnet

import (
	"github.com/DiegoSandival/synap2p-go/protocol"
)

func (h *CentralHandler) Announce(parser *protocol.ProtocolParser, payload []byte) []byte {
	_, err := parser.ParseU16LenPayload(payload) // CID
	if err != nil {
		return parser.ErrorResponse(0x08)
	}
	// Lógica: h.DHT.Provide(...)
	return parser.SuccessResponse(0x08)
}

func (h *CentralHandler) Unannounce(parser *protocol.ProtocolParser, payload []byte) []byte {
	_, err := parser.ParseU16LenPayload(payload) // CID
	if err != nil {
		return parser.ErrorResponse(0x09)
	}
	// Lógica: remover provider o esperar que caduque en el DHT
	return parser.SuccessResponse(0x09)
}

func (h *CentralHandler) FindProviders(parser *protocol.ProtocolParser, payload []byte) []byte {
	_, err := parser.ParseU16LenPayload(payload) // CID
	if err != nil {
		return parser.ErrorResponse(0x0A)
	}
	// Lógica: h.DHT.FindProvidersAsync(...)
	return parser.SuccessResponse(0x0A)
}
