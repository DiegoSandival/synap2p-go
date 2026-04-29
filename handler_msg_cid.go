package quicnet

import (
	"github.com/DiegoSandival/synap2p-go/protocol"
)

func (h *CentralHandler) DirectMsg(parser *protocol.ProtocolParser, payload []byte) []byte {
	_, err := parser.ParseDirectMsg(payload) // PeerID + Data
	if err != nil {
		return parser.ErrorResponse(0x0B)
	}
	// Lógica: Abrir stream directo h.Host.NewStream(...)
	return parser.SuccessResponse(0x0B)
}

func (h *CentralHandler) GenerateCID(parser *protocol.ProtocolParser, payload []byte) []byte {
	_, err := parser.ParseU32LenPayload(payload) // Data
	if err != nil {
		return parser.ErrorResponse(0x0D)
	}
	// Lógica: Construir tu cid en tiempo de ejecución (cid.NewPrefixV1(...) etc.)
	return parser.SuccessResponse(0x0D)
}
