package quicnet

import (
	"context"

	"github.com/DiegoSandival/synap2p-go/protocol"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	mc "github.com/multiformats/go-multicodec"
	mh "github.com/multiformats/go-multihash"
)

func (h *CentralHandler) DirectMsg(parser *protocol.ProtocolParser, payload []byte) []byte {
	id, peerBytes, dataBytes, err := parser.ParseDirectMsg(payload) // Extrae PeerID + Data internamente
	if err != nil {
		if id == nil {
			id = make([]byte, 16)
		}
		return parser.ErrorResponse(0x0B, id)
	}

	peerID, err := peer.Decode(string(peerBytes))
	if err != nil {
		return parser.ErrorResponse(0x0B, id)
	}

	err = h.Client.SendDirectMessage(context.Background(), peerID, dataBytes)
	if err != nil {
		return parser.ErrorResponse(0x0B, id)
	}

	return parser.SuccessResponse(0x0B, id)
}

func (h *CentralHandler) GenerateCID(parser *protocol.ProtocolParser, payload []byte) []byte {
	id, dataBytes, err := parser.ParseSinglePayload(payload) // Data
	if err != nil {
		return parser.ErrorResponse(0x0D, id)
	}

	// Generar CID v1 simple (Sha2-256)
	pref := cid.Prefix{
		Version:  1,
		Codec:    uint64(mc.Raw),
		MhType:   mh.SHA2_256,
		MhLength: -1, // default
	}

	c, err := pref.Sum(dataBytes)
	if err != nil {
		return parser.ErrorResponse(0x0D, id)
	}

	return parser.SuccessDataResponse(0x0D, id, []byte(c.String()))
}
