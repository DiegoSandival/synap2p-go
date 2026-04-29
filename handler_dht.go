package quicnet

import (
	"context"
	"strings"

	"github.com/DiegoSandival/synap2p-go/protocol"
)

func (h *CentralHandler) Announce(parser *protocol.ProtocolParser, payload []byte) []byte {
	id, cidBytes, err := parser.ParseSinglePayload(payload) // CID
	if err != nil {
		return parser.ErrorResponse(0x08, id)
	}

	cidStr := string(cidBytes)
	err = h.Client.AnnounceData(context.Background(), cidStr)
	if err != nil {
		return parser.ErrorResponse(0x08, id)
	}

	return parser.SuccessResponse(0x08, id)
}

func (h *CentralHandler) Unannounce(parser *protocol.ProtocolParser, payload []byte) []byte {
	id, cidBytes, err := parser.ParseSinglePayload(payload) // CID
	if err != nil {
		return parser.ErrorResponse(0x09, id)
	}

	cidStr := string(cidBytes)
	err = h.Client.UnannounceData(cidStr)
	if err != nil {
		return parser.ErrorResponse(0x09, id)
	}

	return parser.SuccessResponse(0x09, id)
}

func (h *CentralHandler) FindProviders(parser *protocol.ProtocolParser, payload []byte) []byte {
	id, cidBytes, err := parser.ParseSinglePayload(payload) // CID
	if err != nil {
		return parser.ErrorResponse(0x0A, id)
	}

	cidStr := string(cidBytes)
	infos, err := h.Client.FindDataProviders(context.Background(), cidStr)
	if err != nil || len(infos) == 0 {
		return parser.ErrorResponse(0x0A, id)
	}

	peersStrs := make([]string, len(infos))
	for i, info := range infos {
		peersStrs[i] = info.ID.String() // Solo mandamos PeerID por simplicidad o JSON
	}

	dataStr := strings.Join(peersStrs, ",")
	return parser.SuccessDataResponse(0x0A, id, []byte(dataStr))
}
