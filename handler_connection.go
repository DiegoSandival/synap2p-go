package quicnet

import (
	"context"
	"strings"

	"github.com/DiegoSandival/synap2p-go/protocol"
	"github.com/libp2p/go-libp2p/core/peer"
)

func (h *CentralHandler) Relay(parser *protocol.ProtocolParser, payload []byte) []byte {
	id, addrBytes, err := parser.ParseSinglePayload(payload)
	if err != nil {
		return parser.ErrorResponse(0x01, id)
	}

	addr := string(addrBytes)
	err = h.Client.ConnectToRelay(context.Background(), addr)
	if err != nil {
		return parser.ErrorResponse(0x01, id)
	}

	return parser.SuccessResponse(0x01, id)
}

func (h *CentralHandler) Dial(parser *protocol.ProtocolParser, payload []byte) []byte {
	id, addrBytes, err := parser.ParseSinglePayload(payload)
	if err != nil {
		return parser.ErrorResponse(0x02, id)
	}

	addr := string(addrBytes)
	err = h.Client.ConnectToPeer(context.Background(), addr)
	if err != nil {
		return parser.ErrorResponse(0x02, id)
	}

	return parser.SuccessResponse(0x02, id)
}

func (h *CentralHandler) Peers(parser *protocol.ProtocolParser, payload []byte) []byte {
	id, _, err := parser.ParseSinglePayload(payload)
	if err != nil {
		return parser.ErrorResponse(0x06, id)
	}

	peers := h.Client.ConnectedPeers()
	peerStrs := make([]string, len(peers))
	for i, p := range peers {
		peerStrs[i] = p.String()
	}

	// Joining peer IDs with a comma (or anything similar)
	dataStr := strings.Join(peerStrs, ",")
	return parser.SuccessDataResponse(0x06, id, []byte(dataStr))
}

func (h *CentralHandler) Disconnect(parser *protocol.ProtocolParser, payload []byte) []byte {
	id, peerBytes, err := parser.ParseSinglePayload(payload) // PeerID
	if err != nil {
		return parser.ErrorResponse(0x0C, id)
	}

	peerID, err := peer.Decode(string(peerBytes))
	if err != nil {
		return parser.ErrorResponse(0x0C, id)
	}

	err = h.Client.DisconnectPeer(peerID)
	if err != nil {
		return parser.ErrorResponse(0x0C, id)
	}

	return parser.SuccessResponse(0x0C, id)
}
