package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	quicnet "github.com/DiegoSandival/synap2p-go"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
)

const (
	OpRelay byte = 0x01
	OpDial  byte = 0x02
	OpPeers byte = 0x03

	OpSub   byte = 0x04
	OpUse   byte = 0x05
	OpPub   byte = 0x06
	OpUnsub byte = 0x07

	OpAnnounce      byte = 0x08
	OpUnannounce    byte = 0x09
	OpFindProviders byte = 0x0A

	OpDirectMsg   byte = 0x0B
	OpDisconnect  byte = 0x0C
	OpGenerateCID byte = 0x0D

	OpResponseOK    byte = 0x00
	OpResponseError byte = 0x01
)

type RequestID [16]byte

type P2PMessage struct {
	Opcode  byte
	ID      RequestID
	Payload []byte
}

func Unmarshal(data []byte) (P2PMessage, error) {
	if len(data) < 17 {
		return P2PMessage{}, errors.New("mensaje demasiado corto")
	}

	var msg P2PMessage
	msg.Opcode = data[0]
	copy(msg.ID[:], data[1:17])
	msg.Payload = data[17:]
	return msg, nil
}

func (m P2PMessage) Marshal() []byte {
	res := make([]byte, 1+16+len(m.Payload))
	res[0] = m.Opcode
	copy(res[1:17], m.ID[:])
	copy(res[17:], m.Payload)
	return res
}

type P2PHandler struct {
	client *quicnet.ClientNode
	state  handlerState
}

type handlerState struct {
	currentTopic     string
	subscribedTopics map[string]struct{}
}

func NewP2PHandler(client *quicnet.ClientNode) *P2PHandler {
	return &P2PHandler{
		client: client,
		state: handlerState{
			subscribedTopics: make(map[string]struct{}),
		},
	}
}

func (h *P2PHandler) ProcessMessage(msg P2PMessage) (P2PMessage, error) {
	switch msg.Opcode {
	case OpRelay:
		addr, rest, err := readU16Bytes(msg.Payload)
		if err != nil || len(rest) != 0 {
			return h.errorResponse(msg.ID, msg.Opcode, "payload inválido para relay")
		}
		ctx, cancel := contextWithTimeout()
		defer cancel()
		if err := h.client.ConnectToRelay(ctx, string(addr)); err != nil {
			return h.errorResponse(msg.ID, msg.Opcode, err.Error())
		}
		return h.okResponse(msg.ID, msg.Opcode), nil

	case OpDial:
		addr, rest, err := readU16Bytes(msg.Payload)
		if err != nil || len(rest) != 0 {
			return h.errorResponse(msg.ID, msg.Opcode, "payload inválido para dial")
		}
		ctx, cancel := contextWithTimeout()
		defer cancel()
		if err := h.client.ConnectToPeer(ctx, string(addr)); err != nil {
			return h.errorResponse(msg.ID, msg.Opcode, err.Error())
		}
		return h.okResponse(msg.ID, msg.Opcode), nil

	case OpSub:
		topic, rest, err := readU16Bytes(msg.Payload)
		if err != nil || len(rest) != 0 {
			return h.errorResponse(msg.ID, msg.Opcode, "payload inválido para sub")
		}
		topicName := string(topic)
		if err := h.client.Subscribe(topicName, func(data []byte) {
			fmt.Printf("gossip event topic=%s payload=%d bytes\n", topicName, len(data))
		}); err != nil {
			return h.errorResponse(msg.ID, msg.Opcode, err.Error())
		}
		h.state.subscribedTopics[topicName] = struct{}{}
		return h.okResponse(msg.ID, msg.Opcode), nil

	case OpUse:
		topic, rest, err := readU16Bytes(msg.Payload)
		if err != nil || len(rest) != 0 {
			return h.errorResponse(msg.ID, msg.Opcode, "payload inválido para use")
		}
		topicName := string(topic)
		if _, exists := h.state.subscribedTopics[topicName]; !exists {
			return h.errorResponse(msg.ID, msg.Opcode, "topic no suscrito")
		}
		h.state.currentTopic = topicName
		return h.okResponse(msg.ID, msg.Opcode), nil

	case OpPub:
		message, rest, err := readU32Bytes(msg.Payload)
		if err != nil || len(rest) != 0 {
			return h.errorResponse(msg.ID, msg.Opcode, "payload inválido para pub")
		}
		if h.state.currentTopic == "" {
			return h.errorResponse(msg.ID, msg.Opcode, "no hay topic seleccionado")
		}
		ctx, cancel := contextWithTimeout()
		defer cancel()
		if err := h.client.Publish(ctx, h.state.currentTopic, message); err != nil {
			return h.errorResponse(msg.ID, msg.Opcode, err.Error())
		}
		return h.okResponse(msg.ID, msg.Opcode), nil

	case OpPeers:
		if len(msg.Payload) != 0 {
			return h.errorResponse(msg.ID, msg.Opcode, "peers no acepta payload")
		}
		return h.peersResponse(msg.ID), nil

	case OpGenerateCID:
		data, rest, err := readU32Bytes(msg.Payload)
		if err != nil || len(rest) != 0 {
			return h.errorResponse(msg.ID, msg.Opcode, "payload inválido para generateCID")
		}
		cidBytes, err := generateCID(data)
		if err != nil {
			return h.errorResponse(msg.ID, msg.Opcode, err.Error())
		}
		return h.cidResponse(msg.ID, cidBytes), nil

	default:
		return h.errorResponse(msg.ID, msg.Opcode, fmt.Sprintf("opcode no soportado 0x%02x", msg.Opcode))
	}
}

func (h *P2PHandler) okResponse(id RequestID, reqOpcode byte) P2PMessage {
	return P2PMessage{
		Opcode:  OpResponseOK,
		ID:      id,
		Payload: []byte{reqOpcode},
	}
}

func (h *P2PHandler) errorResponse(id RequestID, reqOpcode byte, message string) (P2PMessage, error) {
	payload := []byte{reqOpcode}
	payload = appendU16Bytes(payload, []byte(message))
	return P2PMessage{
		Opcode:  OpResponseError,
		ID:      id,
		Payload: payload,
	}, nil
}

func (h *P2PHandler) peersResponse(id RequestID) P2PMessage {
	peers := h.client.ConnectedPeers()
	peerIDs := make([]string, 0, len(peers))
	for _, peerID := range peers {
		peerIDs = append(peerIDs, peerID.String())
	}

	payload := []byte{OpPeers}
	payload = appendU16(payload, uint16(len(peerIDs)))
	for _, peerID := range peerIDs {
		payload = appendU16Bytes(payload, []byte(peerID))
	}

	return P2PMessage{
		Opcode:  OpResponseOK,
		ID:      id,
		Payload: payload,
	}
}

func (h *P2PHandler) cidResponse(id RequestID, cidBytes []byte) P2PMessage {
	payload := []byte{OpGenerateCID}
	payload = appendU16Bytes(payload, cidBytes)
	return P2PMessage{
		Opcode:  OpResponseOK,
		ID:      id,
		Payload: payload,
	}
}

func readU16Bytes(data []byte) ([]byte, []byte, error) {
	if len(data) < 2 {
		return nil, nil, errors.New("missing u16 length")
	}
	length := int(binary.BigEndian.Uint16(data[:2]))
	if len(data)-2 < length {
		return nil, nil, errors.New("payload shorter than u16 length")
	}
	return data[2 : 2+length], data[2+length:], nil
}

func readU32Bytes(data []byte) ([]byte, []byte, error) {
	if len(data) < 4 {
		return nil, nil, errors.New("missing u32 length")
	}
	length := int(binary.BigEndian.Uint32(data[:4]))
	if len(data)-4 < length {
		return nil, nil, errors.New("payload shorter than u32 length")
	}
	return data[4 : 4+length], data[4+length:], nil
}

func appendU16(dst []byte, value uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, value)
	return append(dst, buf...)
}

func appendU16Bytes(dst []byte, payload []byte) []byte {
	if len(payload) > 0xFFFF {
		payload = payload[:0xFFFF]
	}
	dst = appendU16(dst, uint16(len(payload)))
	return append(dst, payload...)
}

func appendU32Bytes(dst []byte, payload []byte) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(len(payload)))
	return append(append(dst, buf...), payload...)
}

func generateCID(data []byte) ([]byte, error) {
	prefix := cid.Prefix{
		Version:  1,
		Codec:    uint64(multicodec.Raw),
		MhType:   multihash.SHA2_256,
		MhLength: -1,
	}
	contentID, err := prefix.Sum(data)
	if err != nil {
		return nil, err
	}
	return []byte(contentID.String()), nil
}

func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Second)
}

func main() {
	fmt.Println("P2P handler example: transport-agnostic message processing")
}
