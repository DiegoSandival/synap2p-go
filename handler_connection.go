package quicnet

import (
	"github.com/DiegoSandival/synap2p-go/protocol"
)

func (h *CentralHandler) Relay(parser *protocol.ProtocolParser, payload []byte) []byte {
	_, err := parser.ParseU16LenPayload(payload) // Addr
	if err != nil {
		return parser.ErrorResponse(0x01)
	}
	// Lógica: Habilitar o conectar un circuit relay usando h.Host
	return parser.SuccessResponse(0x01)
}

func (h *CentralHandler) Dial(parser *protocol.ProtocolParser, payload []byte) []byte {
	_, err := parser.ParseU16LenPayload(payload) // Addr
	if err != nil {
		return parser.ErrorResponse(0x02)
	}
	// Lógica: h.Host.Connect(...) al peer solicitado
	return parser.SuccessResponse(0x02)
}

func (h *CentralHandler) Peers(parser *protocol.ProtocolParser, payload []byte) []byte {
	// Lógica: Retornar lista de pares conectados h.Host.Network().Peers()
	return parser.SuccessResponse(0x06)
}

func (h *CentralHandler) Disconnect(parser *protocol.ProtocolParser, payload []byte) []byte {
	_, err := parser.ParseU16LenPayload(payload) // PeerID
	if err != nil {
		return parser.ErrorResponse(0x0C)
	}
	// Lógica: h.Host.Network().ClosePeer(...)
	return parser.SuccessResponse(0x0C)
}
